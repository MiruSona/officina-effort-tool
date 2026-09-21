package collect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// writeSession 은 시험용 세션 파일 하나를 임시 폴더에 쓰고 읽는다. 실제 자료는 안 건드린다.
func writeSession(t *testing.T, lines ...string) SessionResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sess-wall0001.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ReadSession(path)
	if err != nil {
		t.Fatalf("ReadSession : %v", err)
	}
	return res
}

func userLine(id, ts string) string {
	return `{"type":"user","promptId":"` + id + `","uuid":"u-` + id +
		`","message":{"role":"user","content":"안녕"},"timestamp":"` + ts + `"}`
}

func TestSideLineDoesNotExtendEnd(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		`{"type":"assistant","message":{"role":"assistant","id":"m1","model":"opus"},"uuid":"a1","timestamp":"2026-08-30T10:00:20.000Z"}`,
		// 다음 프롬프트가 큐에 들어오는 순간의 곁줄. 이것이 끝을 밀면 사람 대기가 앞 작업에 붙는다.
		`{"type":"queue-operation","subtype":"enqueue","timestamp":"2026-08-30T10:30:00.000Z"}`,
	)
	if len(res.Tasks) != 1 {
		t.Fatalf("작업 수 = %d", len(res.Tasks))
	}
	if res.Tasks[0].WallMs != 20000 {
		t.Fatalf("벽시계 = %d ms, 바란 값 20000 (곁줄이 끝을 밀었다)", res.Tasks[0].WallMs)
	}
}

func TestAwaySummaryIsSideLine(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		`{"type":"assistant","message":{"role":"assistant","id":"m1","model":"opus"},"uuid":"a1","timestamp":"2026-08-30T10:00:10.000Z"}`,
		`{"type":"system","subtype":"away_summary","timestamp":"2026-08-30T10:20:00.000Z"}`,
	)
	if res.Tasks[0].WallMs != 10000 {
		t.Fatalf("벽시계 = %d ms, 바란 값 10000 (자리비움 요약이 끝을 밀었다)", res.Tasks[0].WallMs)
	}
	if len(res.Unknown) != 0 {
		t.Fatalf("아는 곁줄인데 모르는 종류로 셌다 : %v", res.Unknown)
	}
}

func TestUnknownLineTypeCounted(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		`{"type":"x-foo","timestamp":"2026-08-30T10:40:00.000Z"}`,
		`{"type":"x-foo","timestamp":"2026-08-30T10:41:00.000Z"}`,
	)
	if res.Unknown["x-foo"] != 2 {
		t.Fatalf("모르는 줄 종류 = %v, x-foo 2건을 바랐다", res.Unknown)
	}
	if res.Tasks[0].WallMs != 0 {
		t.Fatalf("모르는 줄이 벽시계를 늘렸다 : %d ms", res.Tasks[0].WallMs)
	}
}

// UnionMs 는 이제 서브 구간(AgentWallMs)과 묶음 벽시계를 잴 때만 쓴다. 작업 벽시계는 본줄만이다.
func TestUnionCountsOverlapOnce(t *testing.T) {
	base := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	at := func(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }
	// 0~10 과 5~15 가 겹친다. 합이면 20분, 합집합이면 15분이다.
	got := UnionMs([]Span{{Start: at(0), End: at(10)}, {Start: at(5), End: at(15)}})
	if got != 15*60000 {
		t.Fatalf("합집합 = %d ms, 바란 값 15분", got)
	}
	// 떨어진 구간은 그대로 더한다.
	got = UnionMs([]Span{{Start: at(0), End: at(10)}, {Start: at(20), End: at(30)}})
	if got != 20*60000 {
		t.Fatalf("떨어진 구간 합집합 = %d ms, 바란 값 20분", got)
	}
	if UnionMs(nil) != 0 {
		t.Fatal("빈 목록인데 0 이 아니다")
	}
}

// 겹치는 서브 구간 둘은 AgentWallMs 에서 한 번만 센다. 벽시계는 그것과 상관없이 본줄 그대로다.
func TestAgentWallCountsOverlapOnce(t *testing.T) {
	base := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	at := func(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }
	task := model.Task{
		Start: at(0), End: at(10),
		Agents: []model.Agent{
			{AgentID: "sub1", Start: at(2), End: at(20)},
			{AgentID: "sub2", Start: at(15), End: at(30)},
		},
	}
	applyWallTime(&task)
	if task.AgentWallMs != 28*60000 {
		t.Fatalf("서브 구간 합집합 = %d ms, 바란 값 28분 (2~30 에서 겹친 15~20 은 한 번만)", task.AgentWallMs)
	}
	if task.WallMs != 10*60000 {
		t.Fatalf("벽시계 = %d ms, 바란 값 10분 (본줄만)", task.WallMs)
	}
}

// 서브에이전트 구간은 벽시계를 안 늘린다. AgentWallMs 에만 참고값으로 잡힌다.
func TestAgentSpanDoesNotExtendWall(t *testing.T) {
	res := readSmall(t)
	first := res.Tasks[0]
	// 서브 구간(10:00:06~10:00:10)이 본줄 구간 안에 있다.
	if first.MainWallMs != 12500 || first.WallMs != 12500 {
		t.Fatalf("본줄 %d · 벽시계 %d", first.MainWallMs, first.WallMs)
	}
	if first.AgentWallMs != 4000 {
		t.Fatalf("서브 구간 = %d ms, 바란 값 4000", first.AgentWallMs)
	}

	// 본줄 밖으로 삐져나간 서브 구간도 벽시계를 못 늘린다.
	base := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	task := model.Task{
		Start: base, End: base.Add(10 * time.Minute),
		Agents: []model.Agent{{AgentID: "sub1", Start: base.Add(5 * time.Minute), End: base.Add(9 * time.Hour)}},
	}
	applyWallTime(&task)
	if task.WallMs != 10*60000 {
		t.Fatalf("벽시계 = %d ms, 바란 값 10분 (9시간짜리 서브 구간은 안 든다)", task.WallMs)
	}
	if task.MainWallMs != 10*60000 {
		t.Fatalf("본줄 = %d ms, 바란 값 10분", task.MainWallMs)
	}
	if task.AgentWallMs != (9*60-5)*60000 {
		t.Fatalf("서브 구간 = %d ms, 바란 값 535분", task.AgentWallMs)
	}
}

func TestPureClampedToWall(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		// 벽시계 10초인데 턴 시간이 135분이다 — 배경 갈래 수명이 통째로 붙은 꼴.
		`{"type":"system","subtype":"turn_duration","durationMs":8106840,"timestamp":"2026-08-30T10:00:10.000Z"}`,
	)
	task := res.Tasks[0]
	if task.PureMs != task.WallMs {
		t.Fatalf("순수 %d · 벽시계 %d — 벽시계로 잘라야 한다", task.PureMs, task.WallMs)
	}
	if !task.HasWarn(WarnPureClamped) {
		t.Fatalf("잘랐는데 표시가 없다 : %v", task.Warn)
	}
}

func TestBgAgentWarned(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		`{"type":"system","subtype":"turn_duration","durationMs":5000,"pendingBackgroundAgentCount":2,"timestamp":"2026-08-30T10:00:10.000Z"}`,
	)
	if !res.Tasks[0].HasWarn(WarnBgAgent) {
		t.Fatalf("배경 갈래 표시가 없다 : %v", res.Tasks[0].Warn)
	}
}

// Claude Code 새 판의 줄 종류들도 「알고 뺀」 곁줄이어야 한다. 모르는 종류로 세면 안 된다.
func TestArtifactLinesAreKnownSide(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-08-30T10:00:00.000Z"),
		`{"type":"assistant","message":{"role":"assistant","id":"m1","model":"opus"},"uuid":"a1","timestamp":"2026-08-30T10:00:10.000Z"}`,
		`{"type":"artifact-autoreact-ledger","timestamp":"2026-08-30T10:30:00.000Z"}`,
		`{"type":"artifact-comment-monitor","timestamp":"2026-08-30T10:31:00.000Z"}`,
		`{"type":"frame-link","timestamp":"2026-08-30T10:32:00.000Z"}`,
		`{"type":"custom-title","timestamp":"2026-08-30T10:33:00.000Z"}`,
	)
	if len(res.Unknown) != 0 {
		t.Fatalf("아는 곁줄인데 모르는 종류로 셌다 : %v", res.Unknown)
	}
	if res.Tasks[0].WallMs != 10000 {
		t.Fatalf("벽시계 = %d ms, 바란 값 10000 (곁줄이 끝을 밀었다)", res.Tasks[0].WallMs)
	}
}
