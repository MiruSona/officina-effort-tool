package collect

import (
	"path/filepath"
	"testing"
)

const smallPath = "../../testdata/sessions/small/sess-small0001.jsonl"

func readSmall(t *testing.T) SessionResult {
	t.Helper()
	res, err := ReadSession(filepath.FromSlash(smallPath))
	if err != nil {
		t.Fatalf("ReadSession : %v", err)
	}
	return res
}

func TestPromptIDCarryForward(t *testing.T) {
	res := readSmall(t)
	if len(res.Tasks) != 2 {
		t.Fatalf("작업 수 = %d, 바란 값 2", len(res.Tasks))
	}
	first := res.Tasks[0]
	// assistant·turn_duration 줄에는 promptId 가 없다. 앞선 user 의 것을 받아야 한다.
	if first.Turns != 1 {
		t.Fatalf("첫 작업 턴 = %d, 바란 값 1", first.Turns)
	}
	if first.PureMs != 12000 {
		t.Fatalf("첫 작업 PureMs = %d, 바란 값 12000", first.PureMs)
	}
}

func TestPreambleLinesDropped(t *testing.T) {
	res := readSmall(t)
	for _, task := range res.Tasks {
		if task.PromptID == preambleID {
			t.Fatal("첫 user 앞 줄이 작업으로 들어갔다")
		}
	}
}

func TestTurnDurationSum(t *testing.T) {
	res := readSmall(t)
	if res.Tasks[1].PureMs != 8000 {
		t.Fatalf("둘째 작업 PureMs = %d, 바란 값 8000", res.Tasks[1].PureMs)
	}
}

func TestWallFromFirstUserToLastLine(t *testing.T) {
	res := readSmall(t)
	// 10:00:00 → 10:00:12.5
	if res.Tasks[0].WallMs != 12500 {
		t.Fatalf("WallMs = %d, 바란 값 12500", res.Tasks[0].WallMs)
	}
}

func TestSessionSumMatchesCostState(t *testing.T) {
	res := readSmall(t)
	sum := res.Session.SumUsage.Sum().Total()
	state := res.Session.StateUsage.Sum().Total()
	if sum != state {
		t.Fatalf("작업 합 %d != cost-state %d", sum, state)
	}
	if res.Session.CoverPct < 99.9 || res.Session.CoverPct > 100.1 {
		t.Fatalf("CoverPct = %.2f", res.Session.CoverPct)
	}
	if res.Session.InProgress {
		t.Fatal("cost-state 가 있는데 진행중으로 봤다")
	}
}

// cost-state 는 누계라 두 줄이 있어도 마지막 것 하나만 써야 한다 (실물에서 두 배로 세던 버그).
func TestCostStateNotSummedTwice(t *testing.T) {
	res := readSmall(t)
	if got := res.Session.StateUsage.Sum().Total(); got != 3716 {
		t.Fatalf("세션 총계 = %d, 바란 값 3716 (두 번 더하면 7432)", got)
	}
}

func TestPureOverWallWarned(t *testing.T) {
	res := readSmall(t)
	// 첫 작업 : 벽시계 12.5초 · 순수 12초 → 정상
	if res.Tasks[0].HasWarn(WarnPureOverWall) {
		t.Fatalf("정상인데 경고가 붙었다 : %v", res.Tasks[0].Warn)
	}
	// 둘째 작업 : 벽시계 28초 · 순수 8초 → 정상
	if res.Tasks[1].HasWarn(WarnPureOverWall) {
		t.Fatalf("정상인데 경고가 붙었다 : %v", res.Tasks[1].Warn)
	}
}

func TestSubagentAttachedByPromptID(t *testing.T) {
	res := readSmall(t)
	if len(res.Tasks[0].Agents) != 1 {
		t.Fatalf("서브에이전트 수 = %d, 바란 값 1", len(res.Tasks[0].Agents))
	}
	a := res.Tasks[0].Agents[0]
	if a.AgentType != "Explore" || a.Model != "claude-opus-5" {
		t.Fatalf("meta 를 못 읽었다 : %+v", a)
	}
	if len(res.Tasks[1].Agents) != 0 {
		t.Fatal("다른 작업에 서브가 붙었다")
	}
}

func TestModelNameNormalizedInTask(t *testing.T) {
	res := readSmall(t)
	if _, ok := res.Tasks[1].MainUsage["claude-opus-5"]; !ok {
		t.Fatalf("모델 이름을 안 모았다 : %+v", res.Tasks[1].MainUsage)
	}
}

func TestNoSubagentFolderWarns(t *testing.T) {
	res, err := ReadSession(filepath.FromSlash("../../testdata/sessions/nosubagents/sess-nosub0001.jsonl"))
	if err != nil {
		t.Fatalf("ReadSession : %v", err)
	}
	if len(res.Tasks) != 1 {
		t.Fatalf("작업 수 = %d", len(res.Tasks))
	}
	if !res.Tasks[0].HasWarn(WarnNoSubagent) {
		t.Fatalf("경고가 없다 : %v", res.Tasks[0].Warn)
	}
}

func TestInProgressSessionMarksLastTask(t *testing.T) {
	res, err := ReadSession(filepath.FromSlash("../../testdata/sessions/inprogress/sess-run0001.jsonl"))
	if err != nil {
		t.Fatalf("ReadSession : %v", err)
	}
	if !res.Session.InProgress {
		t.Fatal("cost-state 가 없는데 진행중이 아니다")
	}
	last := res.Tasks[len(res.Tasks)-1]
	if !last.HasWarn(WarnInProgress) {
		t.Fatalf("마지막 작업에 진행중 표시가 없다 : %v", last.Warn)
	}
}
