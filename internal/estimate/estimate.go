package estimate

import (
	"fmt"
	"time"

	"github.com/mirusona/efforttool/internal/classify"
	"github.com/mirusona/efforttool/internal/collect"
	"github.com/mirusona/efforttool/internal/model"
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
}

// Options 는 예상 계산에 붙는 손잡이다.
type Options struct {
	Metric   string // wall | pure
	SinceDay int    // 표본으로 볼 지난 날 수
	NoX2     bool
	Freeze   bool
	Human    bool
	Now      time.Time
}

// DefaultOptions 는 기본 손잡이다.
func DefaultOptions() Options {
	return Options{Metric: "wall", SinceDay: 120, Now: time.Now()}
}

// Estimator 는 캐시 표본과 규칙을 들고 예상을 낸다.
type Estimator struct {
	rules   *classify.Rules
	samples map[model.Class][]float64
}

// New 는 작업 목록에서 분류별 표본을 뽑는다.
func New(rules *classify.Rules, tasks []model.Task, opt Options) *Estimator {
	e := &Estimator{rules: rules, samples: map[model.Class][]float64{}}
	cut := opt.Now.AddDate(0, 0, -opt.SinceDay)
	for i := range tasks {
		t := &tasks[i]
		if t.HasWarn(collect.WarnInProgress) {
			continue
		}
		// 일 아닌 칸과 미분류는 표본에서 뺀다. 거르는 자리는 여기 하나뿐이다.
		if !t.Class.IsWork() {
			continue
		}
		if !t.Start.IsZero() && t.Start.Before(cut) {
			continue
		}
		// 사람을 기다린 시간이 섞인 turn_duration 은 순수시간 표본에서 뺀다.
		if opt.Metric == "pure" && t.HasWarn(collect.WarnPureOverWall) {
			continue
		}
		v := metricOf(t, opt.Metric)
		if v <= 0 {
			continue
		}
		e.samples[t.Class] = append(e.samples[t.Class], float64(v))
	}
	return e
}

func metricOf(t *model.Task, metric string) int64 {
	if metric == "pure" {
		return t.PureMs
	}
	return t.WallMs
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
