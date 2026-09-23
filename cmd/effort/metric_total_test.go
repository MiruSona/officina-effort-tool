package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/estimate"
	"github.com/mirusona/officina-effort-tool/internal/group"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

func defaultRulesT(t *testing.T) *classify.Rules {
	t.Helper()
	r, err := classify.ParseRules(strings.NewReader(classify.DefaultRulesText))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// actual 의 실제 칸은 예상과 같은 잣대다 — total 이면 본줄 ∪ 서브(하나당 sub max 까지), wall 이면 본줄만.
func TestActualRealUsesSameMetricAsPlan(t *testing.T) {
	rules := defaultRulesT(t) // sub max 120
	g := group.Group{ID: "g1", Name: "고치기", Class: model.ClassBuild,
		Start: time.Now().Add(-time.Hour), WallMs: 2 * 60000, WithAgentMs: 300 * 60000, PureMs: 60000}
	items := []estimate.Item{{Name: "고치기", Class: model.ClassBuild, Size: "M"}}
	cases := map[string]int64{
		estimate.MetricTotal: 122 * 60000, // 서브 몫 298분 → 120분으로 자름
		estimate.MetricWall:  2 * 60000,
		estimate.MetricPure:  60000,
	}
	for metric, want := range cases {
		opt := estimate.DefaultOptions()
		opt.Metric = metric
		e := estimate.New(rules, estimate.SamplesFromGroups([]group.Group{g}), opt)
		rows := matchActual(items, []group.Group{g}, e, opt, "name", metric, rules.SubMax)
		if !rows[0].Matched || rows[0].RealMs != want {
			t.Errorf("%s : 실제 = %d ms, 바란 값 %d", metric, rows[0].RealMs, want)
		}
	}
}

func TestActualDefaultMetricIsTotal(t *testing.T) {
	home := scanForGroups(t)
	plan := filepath.Join(t.TempDir(), "소단계.md")
	if err := os.WriteFile(plan, []byte("| 소단계 | 분류 |\n| --- | --- |\n| a | 조사 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out := capture(t, "actual", "--home", home, "--from", plan)
	if !strings.Contains(out, "예상·실제 모두 total") {
		t.Fatalf("기준 안내가 없다 :\n%s", out)
	}
	_, out = capture(t, "actual", "--home", home, "--metric", "wall", "--from", plan)
	if !strings.Contains(out, "예상·실제 모두 wall") {
		t.Fatalf("wall 기준 안내가 없다 :\n%s", out)
	}
}

// 같은 분류가 두 줄이어도 표본은 같으니 걸림 수는 한 번만 센다.
func TestCappedSamplesCountsClassOnce(t *testing.T) {
	rows := []estimate.Row{
		{Item: estimate.Item{Class: model.ClassResearch}, Capped: 3},
		{Item: estimate.Item{Class: model.ClassResearch}, Capped: 3},
		{Item: estimate.Item{Class: model.ClassBuild}, Capped: 2},
	}
	if got := cappedSamples(rows); got != 5 {
		t.Fatalf("걸림 수 = %d, 바란 값 5", got)
	}
}

// total 꼬리말 — 서브 몫 상한은 단위 이름으로, 서브 안 대기는 상한까지 든다고 말한다.
func TestEstimateTotalFooterWording(t *testing.T) {
	home := scanForGroups(t)
	_, out := capture(t, "estimate", "--home", home, "조사:S", "조사:M")
	for _, want := range []string{"서브 몫은 묶음 하나당 최대 120분", "메인 쪽 사람 대기는 안 들어가지만 서브 구간 안의 대기(승인 등)는 상한까지 들어간다", "사람 시간 칸에 그대로 옮겨 적지 않는다"} {
		if !strings.Contains(out, want) {
			t.Fatalf("꼬리말에 %q 가 없다 :\n%s", want, out)
		}
	}
	_, out = capture(t, "estimate", "--home", home, "--unit", "task", "조사:S")
	if !strings.Contains(out, "서브 몫은 작업 하나당 최대 120분") {
		t.Fatalf("작업 단위 꼬리말이 아니다 :\n%s", out)
	}
}
