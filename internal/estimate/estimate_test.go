package estimate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/efforttool/internal/classify"
	"github.com/mirusona/efforttool/internal/collect"
	"github.com/mirusona/efforttool/internal/model"
)

func rules(t *testing.T) *classify.Rules {
	t.Helper()
	r, err := classify.ParseRules(strings.NewReader(classify.DefaultRulesText))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// tasksOf 는 같은 분류의 표본 n건을 만든다 (한 건이 wallMin 분).
func tasksOf(c model.Class, n int, wallMin int64) []model.Task {
	out := make([]model.Task, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.Task{
			Class:  c,
			Start:  time.Now().Add(-time.Hour),
			WallMs: wallMin * 60000,
			PureMs: wallMin * 60000,
		})
	}
	return out
}

func TestQuantileInterpolation(t *testing.T) {
	v := []float64{10, 20, 30, 40, 50}
	if got := Quantile(v, 0.5); got != 30 {
		t.Fatalf("p50 = %v", got)
	}
	if got := Quantile(v, 0.2); math.Abs(got-18) > 0.001 {
		t.Fatalf("p20 = %v, 바란 값 18", got)
	}
	if got := Quantile(v, 0.8); math.Abs(got-42) > 0.001 {
		t.Fatalf("p80 = %v, 바란 값 42", got)
	}
}

func TestEstimateSeedOnlyWhenFewSamples(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), tasksOf(model.ClassBuild, 3, 100), opt)
	r := e.Estimate(Item{Name: "a", Class: model.ClassBuild, Size: "M"}, opt)
	if r.P50Ms != 20*60000 {
		t.Fatalf("예상 = %d ms, 바란 값 시드 20분", r.P50Ms)
	}
	if !strings.HasPrefix(r.Source, "시드") {
		t.Fatalf("근거 = %q", r.Source)
	}
}

func TestEstimateBlend(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), tasksOf(model.ClassBuild, 8, 100), opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	// w = (8-5)/7 → 3/7×100분 + 4/7×20분
	mixed := 3.0/7.0*100 + 4.0/7.0*20
	want := int64(mixed * 60000)
	if diff := r.P50Ms - want; diff > 1000 || diff < -1000 {
		t.Fatalf("섞은 값 = %d, 바란 값 %d", r.P50Ms, want)
	}
	if !strings.HasPrefix(r.Source, "섞음 n=8") {
		t.Fatalf("근거 = %q", r.Source)
	}
}

func TestEstimateMeasuredOnly(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), tasksOf(model.ClassBuild, 20, 100), opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	if r.P50Ms != 100*60000 {
		t.Fatalf("예상 = %d ms, 바란 값 100분", r.P50Ms)
	}
	if !strings.HasPrefix(r.Source, "실측 n=20") {
		t.Fatalf("근거 = %q", r.Source)
	}
}

func TestEstimateSizeMultiplier(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), nil, opt)
	m := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	l := e.Estimate(Item{Class: model.ClassBuild, Size: "L"}, opt)
	if l.P50Ms != int64(float64(m.P50Ms)*2.4) {
		t.Fatalf("L = %d, M = %d (2.4배가 아니다)", l.P50Ms, m.P50Ms)
	}
}

func TestEstimateMeasureX2(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Class: model.ClassMeasure, Size: "M"}, opt)
	if r.P50Ms != 90*60000 {
		t.Fatalf("실측 예상 = %d ms, 바란 값 45분×2", r.P50Ms)
	}
	opt.NoX2 = true
	r2 := e.Estimate(Item{Class: model.ClassMeasure, Size: "M"}, opt)
	if r2.P50Ms != 45*60000 {
		t.Fatalf("--no-x2 예상 = %d ms", r2.P50Ms)
	}
}

func TestEstimateFreezeMultiplier(t *testing.T) {
	opt := DefaultOptions()
	opt.Freeze = true
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	if r.P50Ms != int64(20*60000*1.5) {
		t.Fatalf("동결 예상 = %d ms", r.P50Ms)
	}
}

func TestEstimateSkipsInProgressTasks(t *testing.T) {
	opt := DefaultOptions()
	tasks := tasksOf(model.ClassBuild, 20, 100)
	for i := range tasks {
		tasks[i].Warn = []string{collect.WarnInProgress}
	}
	e := New(rules(t), tasks, opt)
	if e.SampleCount(model.ClassBuild) != 0 {
		t.Fatalf("진행중 작업이 표본에 들어갔다 : %d", e.SampleCount(model.ClassBuild))
	}
}

// turn_duration 합이 벽시계보다 훨씬 크면 사람을 기다린 시간이 섞인 것이다.
func TestEstimateSkipsPureOverWall(t *testing.T) {
	opt := DefaultOptions()
	opt.Metric = "pure"
	tasks := tasksOf(model.ClassBuild, 20, 100)
	for i := range tasks {
		tasks[i].Warn = []string{collect.WarnPureOverWall}
	}
	e := New(rules(t), tasks, opt)
	if e.SampleCount(model.ClassBuild) != 0 {
		t.Fatalf("이상한 순수시간이 표본에 들어갔다 : %d", e.SampleCount(model.ClassBuild))
	}
	opt.Metric = "wall"
	e2 := New(rules(t), tasks, opt)
	if e2.SampleCount(model.ClassBuild) != 20 {
		t.Fatalf("벽시계 표본까지 뺐다 : %d", e2.SampleCount(model.ClassBuild))
	}
}

func TestEstimateHumanScale(t *testing.T) {
	opt := DefaultOptions()
	opt.Human = true
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M", Human: 60}, opt)
	if r.HumanMs != int64(60*0.25*60000) {
		t.Fatalf("사람 눈금 = %d ms, 바란 값 15분", r.HumanMs)
	}
}
