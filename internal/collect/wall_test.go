package collect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAgentSpanExtendsWall(t *testing.T) {
	res := readSmall(t)
	first := res.Tasks[0]
	// 서브 구간(10:00:06~10:00:10)이 본줄 구간 안에 들어 있어 벽시계는 그대로다.
	if first.MainWallMs != 12500 || first.WallMs != 12500 {
		t.Fatalf("본줄 %d · 합집합 %d", first.MainWallMs, first.WallMs)
	}
	if first.AgentWallMs != 4000 {
		t.Fatalf("서브 구간 = %d ms, 바란 값 4000", first.AgentWallMs)
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
