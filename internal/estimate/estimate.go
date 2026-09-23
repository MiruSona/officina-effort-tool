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
	// 표본이 무엇이었나. 두 중앙값은 배율을 안 탄 표본 값이고, 표본 0건이거나 --metric pure 면 0 이다.
	// 채택한 쪽(total 이면 WithAgent, wall 이면 Mainline)이 P50Ms 의 밑값이다.
	Unit           string // group | task
	SampleN        int
	MainlineP50Ms  int64 // 메인 본줄만의 중앙값
	WithAgentP50Ms int64 // 본줄 ∪ 서브 구간(하나당 sub max 로 자름)의 중앙값
	Capped         int   // 그 분류 표본 가운데 sub max 에 걸린 수
}

// 표본 단위 이름.
const (
	UnitGroup = "group"
	UnitTask  = "task"
)

// 표본 시간 이름.
const (
	MetricTotal = "total" // 본줄 ∪ 서브 구간 (기본). 서브 몫은 하나당 rules 의 sub max 로 자른다
	MetricWall  = "wall"  // 본줄만 (09-23 전의 기본)
	MetricPure  = "pure"  // 턴 합
)

// Options 는 예상 계산에 붙는 손잡이다.
type Options struct {
	Metric   string // total | wall | pure
	Unit     string // group | task
	SinceDay int    // 표본으로 볼 지난 날 수
	NoX2     bool
	Freeze   bool
	Human    bool
	Now      time.Time
}

// DefaultOptions 는 기본 손잡이다. 표본은 묶음(소단계) 단위, 시간은 본줄 ∪ 서브가 기본이다.
// 이 저장소는 메인이 맡기고 검토만 해서 일이 서브 구간에서 돈다 — 본줄만 세면 스무 배 작다 (2026-09-23).
func DefaultOptions() Options {
	return Options{Metric: MetricTotal, Unit: UnitGroup, SinceDay: 120, Now: time.Now()}
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
	samples map[model.Class][]float64 // 채택한 metric 의 값. 밑값은 여기서 낸다
	// mainline · withAgent 는 같은 표본의 본줄 · 본줄 ∪ 서브 값이다. 근거 칸에 둘 다 찍는다.
	mainline  map[model.Class][]float64
	withAgent map[model.Class][]float64
	capped    map[model.Class]int
}

// New 는 표본에서 분류별 값을 뽑는다.
func New(rules *classify.Rules, samples []Sample, opt Options) *Estimator {
	e := &Estimator{rules: rules, samples: map[model.Class][]float64{},
		mainline: map[model.Class][]float64{}, withAgent: map[model.Class][]float64{}, capped: map[model.Class]int{}}
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
		total, hit := TotalMs(s.WallMs, s.WithAgentMs, rules.SubMax)
		v := s.WallMs
		switch opt.Metric {
		case MetricPure:
			v = s.PureMs
		case MetricTotal:
			v = total
		}
		if v <= 0 {
			continue
		}
		e.samples[s.Class] = append(e.samples[s.Class], float64(v))
		// 서브 구간은 벽시계 이야기다. 순수시간(턴 합)과 섞으면 뜻이 안 맞아 뺀다.
		if opt.Metric == MetricPure {
			continue
		}
		// 서브만 돈 표본(본줄 0)은 본줄 중앙을 0 쪽으로 끌지 않게 뺀다.
		if s.WallMs > 0 {
			e.mainline[s.Class] = append(e.mainline[s.Class], float64(s.WallMs))
		}
		e.withAgent[s.Class] = append(e.withAgent[s.Class], float64(total))
		if hit {
			e.capped[s.Class]++
		}
	}
	return e
}

// TotalMs 는 본줄 ∪ 서브 시간에서 서브 몫(합집합 − 본줄)을 subMaxMin 분으로 자른 값이다.
// 승인 대기로 몇 시간 산 갈래가 p80 을 끌어올리는 것을 막는다. 서브 값을 모르면(0) 본줄 그대로다.
// estimate 표본과 actual 실제 칸이 같은 자리에서 잰다. 두 번째 값은 상한에 걸렸는가다.
func TotalMs(wallMs, withAgentMs int64, subMaxMin int) (int64, bool) {
	capMs := int64(subMaxMin) * 60000
	extra := withAgentMs - wallMs
	if extra <= 0 {
		return wallMs, false
	}
	if capMs > 0 && extra > capMs {
		return wallMs + capMs, true
	}
	return wallMs + extra, false
}

// Capped 는 그 분류 표본 가운데 sub max 에 걸린 수다.
func (e *Estimator) Capped(c model.Class) int { return e.capped[c] }

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
	r.Capped = e.capped[it.Class]
	r.Source = src + e.medianNote(&r, it.Class, opt)
	if it.Human > 0 {
		hm := e.rules.Human[it.Class]
		if hm == 0 {
			hm = 1
		}
		r.HumanMs = int64(it.Human * hm * 60000)
	}
	return r
}

// medianNote 는 근거 칸 뒤에 붙일 표본 중앙값 글이다. 채택한 값이 앞, 다른 쪽이 괄호다.
// 두 중앙이 같으면(서브가 없던 표본) 괄호는 알려 줄 것이 없어 뺀다.
func (e *Estimator) medianNote(r *Row, c model.Class, opt Options) string {
	if r.SampleN == 0 {
		return ""
	}
	unit := unitLabel(opt.Unit)
	if opt.Metric == MetricPure {
		return fmt.Sprintf(" · 순수 %s 중앙 %s", unit, decMinutes(int64(Quantile(e.samples[c], 0.5))))
	}
	r.MainlineP50Ms = int64(Quantile(e.mainline[c], 0.5))
	r.WithAgentP50Ms = int64(Quantile(e.withAgent[c], 0.5))
	head, headMs, other, otherMs := "서브 포함", r.WithAgentP50Ms, "본줄", r.MainlineP50Ms
	if opt.Metric == MetricWall {
		head, headMs, other, otherMs = "본줄", r.MainlineP50Ms, "서브 포함", r.WithAgentP50Ms
	}
	out := fmt.Sprintf(" · %s %s 중앙 %s", head, unit, decMinutes(headMs))
	if otherMs != headMs {
		out += fmt.Sprintf(" (%s %s)", other, decMinutes(otherMs))
	}
	return out
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
