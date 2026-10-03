package collect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 시험 줄 만들기. tool_use 는 assistant 줄, tool_result 는 user 줄에 든다 (실물 꼴 그대로).
func useLine(id, ts string, toolIDs ...string) string {
	blocks := make([]string, 0, len(toolIDs))
	for _, tid := range toolIDs {
		blocks = append(blocks, `{"type":"tool_use","id":"`+tid+`","name":"Bash","input":{"command":"sleep"}}`)
	}
	return `{"type":"assistant","promptId":"` + id + `","uuid":"a-` + ts + `","requestId":"r-` + ts +
		`","message":{"role":"assistant","id":"m-` + ts + `","model":"opus","content":[` + strings.Join(blocks, ",") +
		`]},"timestamp":"` + ts + `"}`
}

func resultLine(id, ts string, toolIDs ...string) string {
	blocks := make([]string, 0, len(toolIDs))
	for _, tid := range toolIDs {
		blocks = append(blocks, `{"tool_use_id":"`+tid+`","type":"tool_result","content":"ok"}`)
	}
	return `{"type":"user","promptId":"` + id + `","uuid":"u-` + ts + `","message":{"role":"user","content":[` +
		strings.Join(blocks, ",") + `]},"timestamp":"` + ts + `"}`
}

// 8분 틈 하나 · 1분 틈 하나 → 대기 8분. 3분 아래 틈은 안 센다.
func TestWaitPairsToolUseAndResult(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:10.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:08:10.000Z", "toolu_a"),
		useLine("p1", "2026-10-01T10:08:20.000Z", "toolu_b"),
		resultLine("p1", "2026-10-01T10:09:20.000Z", "toolu_b"),
	)
	if got := res.Tasks[0].WaitMs; got != 8*60000 {
		t.Fatalf("대기 = %d ms, 바란 값 8분", got)
	}
}

// result 없는 tool_use(끊긴 판)는 안 센다. 끝을 모르는 틈을 지어내지 않는다.
func TestWaitUnpairedIgnored(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:10.000Z", "toolu_a"),
		`{"type":"assistant","promptId":"p1","uuid":"a9","message":{"role":"assistant","id":"m9","model":"opus"},"timestamp":"2026-10-01T10:20:00.000Z"}`,
		// 짝 없는 result 도 안 센다.
		resultLine("p1", "2026-10-01T10:30:00.000Z", "toolu_zz"),
	)
	if got := res.Tasks[0].WaitMs; got != 0 {
		t.Fatalf("대기 = %d ms, 짝 없는 틈은 0 이어야 한다", got)
	}
}

// 대기는 보여 주기만 한다. 벽시계에서 빼지 않는다.
func TestWaitNotSubtractedFromWall(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:10.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:10:10.000Z", "toolu_a"),
		`{"type":"assistant","promptId":"p1","uuid":"a9","message":{"role":"assistant","id":"m9","model":"opus"},"timestamp":"2026-10-01T10:11:00.000Z"}`,
	)
	task := res.Tasks[0]
	if task.WaitMs != 10*60000 {
		t.Fatalf("대기 = %d ms, 바란 값 10분", task.WaitMs)
	}
	if task.WallMs != 11*60000 || task.MainWallMs != 11*60000 {
		t.Fatalf("벽시계 = %d · 본줄 %d, 바란 값 11분 그대로", task.WallMs, task.MainWallMs)
	}
}

// 한 턴에 도구를 나란히 부르면 틈이 겹친다. 겹친 만큼은 한 번만 센다 — 대기가 벽시계를 넘으면 안 된다.
func TestWaitParallelToolsCountedOnce(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:00.000Z", "toolu_a", "toolu_b"),
		resultLine("p1", "2026-10-01T10:05:00.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:06:00.000Z", "toolu_b"),
	)
	if got := res.Tasks[0].WaitMs; got != 6*60000 {
		t.Fatalf("대기 = %d ms, 바란 값 6분 (5분·6분 틈이 겹친다)", got)
	}
}

// 문턱은 바꿀 수 있다. 1분으로 낮추면 1분 넘는 틈도 센다. 문턱과 같은 틈은 「넘는」 게 아니라 안 센다.
func TestWaitThresholdOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess-wait0001.jsonl")
	body := strings.Join([]string{
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:00.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:02:00.000Z", "toolu_a"),
		useLine("p1", "2026-10-01T10:02:00.000Z", "toolu_b"),
		resultLine("p1", "2026-10-01T10:03:00.000Z", "toolu_b"),
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ReadSessionWith(path, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Tasks[0].WaitMs; got != 2*60000 {
		t.Fatalf("문턱 1분 대기 = %d ms, 바란 값 2분 (딱 1분 틈은 안 센다)", got)
	}
	def, err := ReadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if def.Tasks[0].WaitMs != 0 {
		t.Fatalf("기본 문턱 3분인데 대기 = %d ms", def.Tasks[0].WaitMs)
	}
}

// 서브에이전트 파일도 같은 잣대로 따로 잰다. 본줄 대기에는 안 섞인다.
func TestAgentWaitSeparate(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, "sess-agent0001.jsonl")
	sub := filepath.Join(dir, "sess-agent0001", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	main := strings.Join([]string{
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		`{"type":"assistant","promptId":"p1","uuid":"a1","message":{"role":"assistant","id":"m1","model":"opus"},"timestamp":"2026-10-01T10:20:00.000Z"}`,
	}, "\n") + "\n"
	agent := strings.Join([]string{
		userLine("p1", "2026-10-01T10:01:00.000Z"),
		useLine("p1", "2026-10-01T10:01:00.000Z", "toolu_s"),
		resultLine("p1", "2026-10-01T10:09:00.000Z", "toolu_s"),
	}, "\n") + "\n"
	if err := os.WriteFile(sess, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "agent-x1.jsonl"), []byte(agent), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ReadSession(sess)
	if err != nil {
		t.Fatal(err)
	}
	task := res.Tasks[0]
	if task.WaitMs != 0 {
		t.Fatalf("본줄 대기 = %d ms, 서브 대기가 섞였다", task.WaitMs)
	}
	if len(task.Agents) != 1 || task.Agents[0].WaitMs != 8*60000 {
		t.Fatalf("서브 대기가 틀렸다 : %+v", task.Agents)
	}
}

// mark 기록 구간 안의 대기만 센다. 구간 밖에서 시작한 도구 틈은 안 든다.
func TestMainSpanWaitInsideRange(t *testing.T) {
	body := strings.Join([]string{
		useLine("p1", "2026-10-01T09:50:00.000Z", "toolu_out"),
		resultLine("p1", "2026-10-01T10:05:00.000Z", "toolu_out"),
		useLine("p1", "2026-10-01T10:06:00.000Z", "toolu_in"),
		resultLine("p1", "2026-10-01T10:14:00.000Z", "toolu_in"),
	}, "\n") + "\n"
	from := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 10, 20, 0, 0, time.UTC)
	span, err := MainSpan(strings.NewReader(body), from, to, DefaultWaitMin)
	if err != nil {
		t.Fatal(err)
	}
	if span.WaitMs != 8*60000 {
		t.Fatalf("구간 안 대기 = %d ms, 바란 값 8분", span.WaitMs)
	}
}

// 같은 tool_use id 가 두 번 오면(스트리밍 중복 줄) 첫 시각을 쓴다.
func TestWaitDuplicateUseKeepsFirst(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:00.000Z", "toolu_a"),
		useLine("p1", "2026-10-01T10:02:00.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:06:00.000Z", "toolu_a"),
	)
	if got := res.Tasks[0].WaitMs; got != 6*60000 {
		t.Fatalf("대기 = %d ms, 바란 값 6분 (첫 tool_use 시각부터)", got)
	}
}

// 결과 시각이 tool_use 보다 앞인 짝(시계 뒤틀림)은 안 센다.
func TestWaitResultBeforeUseIgnored(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:20:00.000Z", "toolu_a"),
		resultLine("p1", "2026-10-01T10:05:00.000Z", "toolu_a"),
	)
	if got := res.Tasks[0].WaitMs; got != 0 {
		t.Fatalf("대기 = %d ms, 거꾸로 된 짝은 0 이어야 한다", got)
	}
}

// 작업(promptId)을 건너는 짝은 어느 작업의 대기인지 못 정하므로 안 센다.
func TestWaitCrossTaskPairIgnored(t *testing.T) {
	res := writeSession(t,
		userLine("p1", "2026-10-01T10:00:00.000Z"),
		useLine("p1", "2026-10-01T10:00:10.000Z", "toolu_a"),
		userLine("p2", "2026-10-01T10:01:00.000Z"),
		resultLine("p2", "2026-10-01T10:10:00.000Z", "toolu_a"),
	)
	if len(res.Tasks) != 2 {
		t.Fatalf("작업 수 = %d", len(res.Tasks))
	}
	for _, task := range res.Tasks {
		if task.WaitMs != 0 {
			t.Fatalf("%s 대기 = %d ms, 작업을 건넌 짝은 0 이어야 한다", task.PromptID, task.WaitMs)
		}
	}
}
