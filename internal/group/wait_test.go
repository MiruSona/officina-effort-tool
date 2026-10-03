package group

import (
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 묶음 대기는 안에 든 작업 본줄 대기의 합이다. 서브 대기는 안 든다 (벽시계와 같은 잣대).
func TestGroupWaitSumsTasks(t *testing.T) {
	base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{PromptID: "p1", SessionID: "s1", Class: model.ClassBuild, Start: base, End: base.Add(10 * time.Minute),
			WallMs: 10 * 60000, WaitMs: 8 * 60000,
			Agents: []model.Agent{{AgentID: "a", Start: base, End: base.Add(5 * time.Minute), WaitMs: 4 * 60000}}},
		{PromptID: "p2", SessionID: "s1", Class: model.ClassBuild, Start: base.Add(12 * time.Minute), End: base.Add(20 * time.Minute),
			WallMs: 8 * 60000, WaitMs: 5 * 60000},
	}
	gs := Build(tasks, nil, Key{Mode: ModeGap, Gap: 30 * time.Minute})
	if len(gs) != 1 {
		t.Fatalf("묶음 %d개", len(gs))
	}
	if gs[0].WaitMs != 13*60000 {
		t.Fatalf("묶음 대기 = %d ms, 바란 값 13분", gs[0].WaitMs)
	}
}

// 정본 묶음에 다른 세션 작업이 겹치면 대기 합이 합집합 벽시계를 넘을 수 있다 — 벽시계로 자른다.
func TestGroupWaitCappedAtWall(t *testing.T) {
	base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	tasks := []model.Task{
		{PromptID: "p1", SessionID: "s1", Class: model.ClassBuild, Start: base, End: base.Add(10 * time.Minute),
			WallMs: 10 * 60000, WaitMs: 9 * 60000},
		{PromptID: "p2", SessionID: "s2", Class: model.ClassBuild, Start: base, End: base.Add(10 * time.Minute),
			WallMs: 10 * 60000, WaitMs: 9 * 60000},
	}
	marks := []store.Mark{{ID: "g1", Name: "겹친 정본", Tasks: []string{"p1", "p2"}}}
	gs := Build(tasks, marks, Key{Mode: ModeMark, Gap: 30 * time.Minute})
	if len(gs) != 1 {
		t.Fatalf("묶음 %d개", len(gs))
	}
	if gs[0].WallMs != 10*60000 || gs[0].WaitMs != 10*60000 {
		t.Fatalf("벽시계 %d · 대기 %d ms, 대기는 벽시계 10분으로 잘려야 한다", gs[0].WallMs, gs[0].WaitMs)
	}
}
