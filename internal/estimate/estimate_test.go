package estimate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

func rules(t *testing.T) *classify.Rules {
	t.Helper()
	r, err := classify.ParseRules(strings.NewReader(classify.DefaultRulesText))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// samplesOf 는 같은 분류의 표본 n건을 만든다 (한 건이 wallMin 분).
func samplesOf(c model.Class, n int, wallMin int64) []Sample {
	out := make([]Sample, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Sample{
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
	e := New(rules(t), samplesOf(model.ClassBuild, 3, 100), opt)
	r := e.Estimate(Item{Name: "a", Class: model.ClassBuild, Size: "M"}, opt)
	if r.P50Ms != 2*60000 {
		t.Fatalf("예상 = %d ms, 바란 값 시드 2분", r.P50Ms)
	}
	if !strings.HasPrefix(r.Source, "시드") {
		t.Fatalf("근거 = %q", r.Source)
	}
}

func TestEstimateBlend(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), samplesOf(model.ClassBuild, 8, 100), opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	// w = (8-5)/7 → 3/7×100분 + 4/7×2분(시드)
	mixed := 3.0/7.0*100 + 4.0/7.0*2
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
	e := New(rules(t), samplesOf(model.ClassBuild, 20, 100), opt)
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
	if l.P50Ms != int64(float64(m.P50Ms)*2.35) {
		t.Fatalf("L = %d, M = %d (2.35배가 아니다)", l.P50Ms, m.P50Ms)
	}
}

func TestEstimateMeasureX2(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Class: model.ClassMeasure, Size: "M"}, opt)
	if r.P50Ms != 4*60000 {
		t.Fatalf("실측 예상 = %d ms, 바란 값 2분×2", r.P50Ms)
	}
	opt.NoX2 = true
	r2 := e.Estimate(Item{Class: model.ClassMeasure, Size: "M"}, opt)
	if r2.P50Ms != 2*60000 {
		t.Fatalf("--no-x2 예상 = %d ms", r2.P50Ms)
	}
}

func TestEstimateFreezeMultiplier(t *testing.T) {
	opt := DefaultOptions()
	opt.Freeze = true
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt)
	if r.P50Ms != int64(2*60000*1.5) {
		t.Fatalf("동결 예상 = %d ms", r.P50Ms)
	}
}

func TestEstimateSkipsInProgressTasks(t *testing.T) {
	opt := DefaultOptions()
	samples := samplesOf(model.ClassBuild, 20, 100)
	for i := range samples {
		samples[i].Warn = []string{collect.WarnInProgress}
	}
	e := New(rules(t), samples, opt)
	if e.SampleCount(model.ClassBuild) != 0 {
		t.Fatalf("진행중 작업이 표본에 들어갔다 : %d", e.SampleCount(model.ClassBuild))
	}
}

// 벽시계로 잘린 turn_duration 은 진짜 턴 길이가 아니다.
func TestEstimateSkipsPureClamped(t *testing.T) {
	opt := DefaultOptions()
	opt.Metric = "pure"
	samples := samplesOf(model.ClassBuild, 20, 100)
	for i := range samples {
		samples[i].Warn = []string{collect.WarnPureClamped}
	}
	e := New(rules(t), samples, opt)
	if e.SampleCount(model.ClassBuild) != 0 {
		t.Fatalf("이상한 순수시간이 표본에 들어갔다 : %d", e.SampleCount(model.ClassBuild))
	}
	opt.Metric = "wall"
	e2 := New(rules(t), samples, opt)
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

// withAgent 는 서브 포함 시간을 넣은 표본 n건이다.
func withAgent(c model.Class, n int, wallMin, withMin int64) []Sample {
	out := samplesOf(c, n, wallMin)
	for i := range out {
		out[i].WithAgentMs = withMin * 60000
	}
	return out
}

// 근거 칸은 표본 단위·본줄 중앙·서브 포함 중앙을 같이 찍는다.
func TestEstimateSourceShowsSampleMedians(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), withAgent(model.ClassBuild, 20, 2, 5), opt)
	r := e.Estimate(Item{Name: "a", Class: model.ClassBuild, Size: "M"}, opt)
	want := "실측 n=20 · 본줄 묶음 중앙 2.0분 (서브 포함 5.0분)"
	if r.Source != want {
		t.Fatalf("근거 = %q, 바란 값 %q", r.Source, want)
	}
	if r.Unit != UnitGroup || r.SampleN != 20 || r.SampleP50Ms != 2*60000 || r.WithAgentP50Ms != 5*60000 {
		t.Fatalf("JSON 칸 = %+v", r)
	}
}

func TestEstimateSourceSeedAndTaskUnit(t *testing.T) {
	opt := DefaultOptions()
	opt.Unit = UnitTask
	e := New(rules(t), withAgent(model.ClassBuild, 3, 1, 4), opt)
	r := e.Estimate(Item{Name: "a", Class: model.ClassBuild, Size: "M"}, opt)
	if r.Source != "시드 (n=3) · 본줄 작업 중앙 1.0분 (서브 포함 4.0분)" {
		t.Fatalf("근거 = %q", r.Source)
	}
	// 실측 ×2 꼬리는 중앙값 앞에 붙는다 — 중앙값은 배율을 안 탄 표본 값이다.
	e2 := New(rules(t), withAgent(model.ClassMeasure, 3, 1, 4), opt)
	r2 := e2.Estimate(Item{Name: "b", Class: model.ClassMeasure, Size: "M"}, opt)
	if r2.Source != "시드 (n=3) ×2(실측) · 본줄 작업 중앙 1.0분 (서브 포함 4.0분)" {
		t.Fatalf("근거 = %q", r2.Source)
	}
}

// 표본이 없으면 중앙값을 안 찍는다. 서브 포함 중앙이 없거나 본줄 중앙과 같으면 괄호를 뺀다.
// 순수시간에는 서브 포함이 없다.
func TestEstimateSourceOmitsMissingOrSame(t *testing.T) {
	opt := DefaultOptions()
	e := New(rules(t), nil, opt)
	r := e.Estimate(Item{Name: "a", Class: model.ClassBuild, Size: "M"}, opt)
	if r.Source != "시드 (n=0)" || r.SampleP50Ms != 0 {
		t.Fatalf("근거 = %q · %+v", r.Source, r)
	}
	e2 := New(rules(t), samplesOf(model.ClassBuild, 3, 2), opt)
	if got := e2.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt).Source; got != "시드 (n=3) · 본줄 묶음 중앙 2.0분" {
		t.Fatalf("서브 없음 근거 = %q", got)
	}
	opt.Metric = "pure"
	e3 := New(rules(t), withAgent(model.ClassBuild, 3, 2, 5), opt)
	if got := e3.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt).Source; got != "시드 (n=3) · 순수 묶음 중앙 2.0분" {
		t.Fatalf("순수 근거 = %q", got)
	}
	opt.Metric = "wall"
	e4 := New(rules(t), withAgent(model.ClassBuild, 3, 2, 2), opt)
	if got := e4.Estimate(Item{Class: model.ClassBuild, Size: "M"}, opt).Source; got != "시드 (n=3) · 본줄 묶음 중앙 2.0분" {
		t.Fatalf("서브 포함이 본줄과 같은데 괄호가 붙었다 = %q", got)
	}
}

func TestSamplesFromTasksWithAgent(t *testing.T) {
	now := time.Now()
	tk := model.Task{Class: model.ClassBuild, Start: now, End: now.Add(2 * time.Minute), WallMs: 2 * 60000,
		Agents: []model.Agent{{Start: now.Add(time.Minute), End: now.Add(6 * time.Minute)}}}
	s := SamplesFromTasks([]model.Task{tk})
	if s[0].WithAgentMs != 6*60000 {
		t.Fatalf("서브 포함 = %d", s[0].WithAgentMs)
	}
}
