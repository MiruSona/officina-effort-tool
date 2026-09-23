package estimate

import (
	"fmt"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/group"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

// Item 은 예상할 소단계 하나다.
type Item struct {
	Name  string
	Class model.Class
	Size  string
	Human float64 // 사람 눈금(분). 0 이면 없음
}

// Row 는 나온 예상 한 줄이다.
type Row struct {
	Item
	P50Ms   int64
	P20Ms   int64
	P80Ms   int64
	Source  string
	HumanMs int64
	// 표본이 무엇이었나. 예상값이 작게 나오는 까닭(메인 본줄만 셈)을 표 안에서 보이게 한다.
	Unit           string // group | task
	SampleN        int
	SampleP50Ms    int64 // 배율을 안 탄 표본 중앙값. 표본 0건이면 0
	WithAgentP50Ms int64 // 서브에이전트 구간까지 합친 중앙값. 없으면 0
}

// 표본 단위 이름.
const (
	UnitGroup = "group"
	UnitTask  = "task"
)

// Options 는 예상 계산에 붙는 손잡이다.
type Options struct {
	Metric   string // wall | pure
	Unit     string // group | task
	SinceDay int    // 표본으로 볼 지난 날 수
	NoX2     bool
	Freeze   bool
	Human    bool
	Now      time.Time
}

// DefaultOptions 는 기본 손잡이다. 표본은 묶음(소단계) 단위가 기본이다 — 사람 소단계와 자릿수가 맞는다.
func DefaultOptions() Options {
	return Options{Metric: "wall", Unit: UnitGroup, SinceDay: 120, Now: time.Now()}
}

// Sample 은 예상 표본 한 건이다. 작업 하나일 수도, 묶음 하나일 수도 있다.
type Sample struct {
	Class  model.Class
	WallMs int64
	PureMs int64
	// WithAgentMs 는 본줄과 서브에이전트 구간의 합집합이다. 참고값으로 근거 칸에만 쓴다. 0 이면 모름
	WithAgentMs int64
	Start       time.Time
	Warn        []string
}

func (s *Sample) hasWarn(w string) bool {
	for _, v := range s.Warn {
		if v == w {
			return true
		}
	}
	return false
}

// SamplesFromTasks 는 작업 하나를 표본 하나로 삼는다 (--unit task).
func SamplesFromTasks(tasks []model.Task) []Sample {
	out := make([]Sample, 0, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		spans := []collect.Span{{Start: t.Start, End: t.End}}
		for _, a := range t.Agents {
			spans = append(spans, collect.Span{Start: a.Start, End: a.End})
		}
		out = append(out, Sample{
			Class: t.Class, WallMs: t.WallMs, PureMs: t.PureMs, Start: t.Start, Warn: t.Warn,
			WithAgentMs: collect.UnionMs(spans),
		})
	}
	return out
}

// SamplesFromGroups 는 묶음 하나를 표본 하나로 삼는다 (--unit group).
func SamplesFromGroups(gs []group.Group) []Sample {
	out := make([]Sample, 0, len(gs))
	for i := range gs {
		g := &gs[i]
		out = append(out, Sample{
			Class: g.Class, WallMs: g.WallMs, PureMs: g.PureMs, Start: g.Start, Warn: g.Warn,
			WithAgentMs: g.WithAgentMs,
		})
	}
	return out
}

// KeepSample 은 예상 표본으로 쓸 것인지다 — 대표 분류가 일 칸이고 아직 도는 중이 아니어야 한다.
// 「진행 중」 작업은 끝 시각이 없어 시간이 짧게 잡히므로 배율을 낮춰 버린다.
// list --group 도 이 잣대를 그대로 쓴다. 잰 표본과 보는 표본이 갈리면 안 된다.
func KeepSample(c model.Class, warn []string) bool {
	for _, w := range warn {
		if w == collect.WarnInProgress {
			return false
		}
	}
	return c.IsWork()
}

// Estimator 는 캐시 표본과 규칙을 들고 예상을 낸다.
type Estimator struct {
	rules   *classify.Rules
	samples map[model.Class][]float64
	// withAgent 는 같은 표본의 서브 포함 시간이다. 근거 칸 참고값으로만 쓰고 예상에는 안 든다.
	withAgent map[model.Class][]float64
}

// New 는 표본에서 분류별 값을 뽑는다.
func New(rules *classify.Rules, samples []Sample, opt Options) *Estimator {
	e := &Estimator{rules: rules, samples: map[model.Class][]float64{}, withAgent: map[model.Class][]float64{}}
	cut := opt.Now.AddDate(0, 0, -opt.SinceDay)
	for i := range samples {
		s := &samples[i]
		// 일 아닌 칸·미분류·「진행 중」은 표본에서 뺀다. 잣대는 KeepSample 한 자리뿐이다.
		if !KeepSample(s.Class, s.Warn) {
			continue
		}
		if !s.Start.IsZero() && s.Start.Before(cut) {
			continue
		}
		// 벽시계로 잘린 turn_duration 은 진짜 턴 길이가 아니므로 순수시간 표본에서 뺀다.
		if opt.Metric == "pure" && s.hasWarn(collect.WarnPureClamped) {
			continue
		}
		v := metricOf(s, opt.Metric)
		if v <= 0 {
			continue
		}
		e.samples[s.Class] = append(e.samples[s.Class], float64(v))
		// 서브 구간은 벽시계 이야기다. 순수시간(턴 합)과 섞으면 뜻이 안 맞아 뺀다.
		if opt.Metric != "pure" && s.WithAgentMs > 0 {
			e.withAgent[s.Class] = append(e.withAgent[s.Class], float64(s.WithAgentMs))
		}
	}
	return e
}

func metricOf(s *Sample, metric string) int64 {
	if metric == "pure" {
		return s.PureMs
	}
	return s.WallMs
}

// SampleCount 는 그 분류의 표본 수다.
func (e *Estimator) SampleCount(c model.Class) int { return len(e.samples[c]) }

// Estimate 는 소단계 하나의 예상과 범위를 낸다.
func (e *Estimator) Estimate(it Item, opt Options) Row {
	r := Row{Item: it}
	mul := e.rules.Size(it.Size)
	p50, p20, p80, src := e.base(it.Class)
	extra := 1.0
	if it.Class == model.ClassMeasure && !opt.NoX2 {
		extra *= 2
		src += " ×2(실측)"
	}
	if opt.Freeze {
		extra *= 1.5
		src += " ×1.5(동결)"
	}
	r.P50Ms = int64(p50 * mul * extra)
	r.P20Ms = int64(p20 * mul * extra)
	r.P80Ms = int64(p80 * mul * extra)
	r.Unit = opt.Unit
	r.SampleN = len(e.samples[it.Class])
	if r.SampleN > 0 {
		r.SampleP50Ms = int64(Quantile(e.samples[it.Class], 0.5))
		src += fmt.Sprintf(" · %s %s 중앙 %s", metricLabel(opt.Metric), unitLabel(opt.Unit), decMinutes(r.SampleP50Ms))
	}
	if w := e.withAgent[it.Class]; r.SampleN > 0 && len(w) > 0 {
		r.WithAgentP50Ms = int64(Quantile(w, 0.5))
		// 합집합에는 본줄이 늘 들어 있어 서브가 없으면 본줄 중앙과 같다. 같으면 알려 줄 것이 없다.
		if r.WithAgentP50Ms != r.SampleP50Ms {
			src += fmt.Sprintf(" (서브 포함 %s)", decMinutes(r.WithAgentP50Ms))
		}
	}
	r.Source = src
	if it.Human > 0 {
		hm := e.rules.Human[it.Class]
		if hm == 0 {
			hm = 1
		}
		r.HumanMs = int64(it.Human * hm * 60000)
	}
	return r
}

func metricLabel(metric string) string {
	if metric == "pure" {
		return "순수"
	}
	return "본줄"
}

// UnitLabel 은 표본 단위의 사람 이름이다 (묶음 · 작업).
func UnitLabel(unit string) string { return unitLabel(unit) }

func unitLabel(unit string) string {
	if unit == UnitTask {
		return "작업"
	}
	return "묶음"
}

// decMinutes 는 근거 칸용 소수 한 자리 분이다. 예상 칸(정수 분)과 달리 1분 아래 차이도 보여야 한다.
func decMinutes(ms int64) string {
	return fmt.Sprintf("%.1f분", float64(ms)/60000)
}

// base 는 섞기 규칙에 따라 밑값(ms)을 고른다.
func (e *Estimator) base(c model.Class) (p50, p20, p80 float64, src string) {
	seed := e.rules.Seeds[c]
	sp50, sp20, sp80 := seed.P50*60000, seed.P20*60000, seed.P80*60000
	s := e.samples[c]
	n := len(s)
	if n < e.rules.MinSample {
		return sp50, sp20, sp80, fmt.Sprintf("시드 (n=%d)", n)
	}
	mp50 := Quantile(s, 0.5)
	mp20 := Quantile(s, 0.2)
	mp80 := Quantile(s, 0.8)
	if n >= e.rules.BlendMax {
		return mp50, mp20, mp80, fmt.Sprintf("실측 n=%d", n)
	}
	span := float64(e.rules.BlendMax - e.rules.MinSample)
	w := float64(n-e.rules.MinSample) / span
	mix := func(m, sd float64) float64 { return w*m + (1-w)*sd }
	return mix(mp50, sp50), mix(mp20, sp20), mix(mp80, sp80), fmt.Sprintf("섞음 n=%d", n)
}
