package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// waitProjects 는 도구 한 번이 8분 걸린 작업 하나가 든 가짜 원본 뿌리를 만든다.
func waitProjects(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "C--wait-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"type":"user","promptId":"pwait01","uuid":"u1","cwd":"C:\\wait\\proj","message":{"role":"user","content":"그림 만들기 구현"},"timestamp":"2026-10-01T10:00:00.000Z"}`,
		`{"type":"assistant","promptId":"pwait01","uuid":"a1","requestId":"r1","message":{"role":"assistant","id":"m1","model":"claude-opus-5-5","content":[{"type":"tool_use","id":"toolu_w1","name":"Bash","input":{"command":"gen"}}]},"timestamp":"2026-10-01T10:00:30.000Z"}`,
		`{"type":"user","promptId":"pwait01","uuid":"u2","message":{"role":"user","content":[{"tool_use_id":"toolu_w1","type":"tool_result","content":"ok"}]},"timestamp":"2026-10-01T10:08:30.000Z"}`,
		`{"type":"assistant","promptId":"pwait01","uuid":"a2","requestId":"r2","message":{"role":"assistant","id":"m2","model":"claude-opus-5-5","content":[{"type":"text","text":"끝"}]},"timestamp":"2026-10-01T10:10:00.000Z"}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "sess-wait0001.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// scan 이 대기를 캐시에 남기고 show 가 벽시계 줄에 찍는다. 벽시계는 그대로 10분이다.
func TestShowPrintsWait(t *testing.T) {
	home := t.TempDir()
	root := waitProjects(t)
	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("scan 종료 %d\n%s", code, out)
	}
	code, out := capture(t, "show", "--home", home, "pwait01")
	if code != exitOK {
		t.Fatalf("show 종료 %d\n%s", code, out)
	}
	if !strings.Contains(out, "벽시계 10분") || !strings.Contains(out, "대기 8분") {
		t.Fatalf("벽시계 10분 · 대기 8분이 아니다 :\n%s", out)
	}
	// scan 이 빠진 wait 줄을 기본값으로 더해 둔다 (처음 만든 rules.txt 에는 이미 있다).
	raw, _ := os.ReadFile(filepath.Join(home, "rules.txt"))
	if !strings.Contains(string(raw), "wait\tmin\t3") {
		t.Fatalf("rules.txt 에 wait 줄이 없다 :\n%s", raw)
	}
}

// rules.txt 의 wait min 을 올리면 8분 틈도 대기로 안 센다. 규칙이 바뀌면 scan 이 다시 매긴다.
func TestWaitRuleChangesScan(t *testing.T) {
	home := t.TempDir()
	root := waitProjects(t)
	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("scan 종료 %d\n%s", code, out)
	}
	path := filepath.Join(home, "rules.txt")
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), "wait\tmin\t3", "wait\tmin\t10", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("다시 scan 종료 %d\n%s", code, out)
	}
	_, out := capture(t, "show", "--home", home, "pwait01")
	if !strings.Contains(out, "대기 —") {
		t.Fatalf("문턱 10분인데 대기가 잡혔다 :\n%s", out)
	}
}

// 옛 rules.txt(wait 줄 없음)로 scan 하면 기본값 줄을 덧붙인다.
func TestScanAppendsWaitKind(t *testing.T) {
	home := t.TempDir()
	root := waitProjects(t)
	st := store.New(home)
	if _, err := st.EnsureRules(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "rules.txt")
	raw, _ := os.ReadFile(path)
	old := strings.Replace(string(raw), "wait\tmin\t3\n", "", 1)
	if old == string(raw) {
		t.Fatalf("기본 rules.txt 에 wait 줄이 없다")
	}
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "scan", "--home", home, "--projects", root, "--all")
	if code != exitOK || !strings.Contains(out, "wait") {
		t.Fatalf("wait 기본값을 더했다는 말이 없다 : %d\n%s", code, out)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "자동으로 더한 기본값 (wait)") || !strings.HasSuffix(string(after), "wait\tmin\t3\n") {
		t.Fatalf("wait 줄이 덧붙지 않았다 :\n%s", after)
	}
}

// list --group 에 대기 열이 있다.
func TestListGroupHasWaitColumn(t *testing.T) {
	home := t.TempDir()
	st := store.New(home)
	if err := st.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnsureRules(); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-24 * time.Hour).Truncate(time.Minute)
	tasks := []model.Task{{
		PromptID: "w1", SessionID: "s0000009", Class: model.ClassBuild, ClassBy: "제목", Title: "그림 구현",
		Start: base, End: base.Add(10 * time.Minute), WallMs: 10 * 60000, MainWallMs: 10 * 60000, WaitMs: 8 * 60000,
	}}
	if err := st.WriteTasks(tasks); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "list", "--home", home, "--group", "gap:30m")
	if code != exitOK {
		t.Fatalf("list 종료 %d\n%s", code, out)
	}
	if !strings.Contains(out, "| 대기 |") || !strings.Contains(out, "| 8분 |") {
		t.Fatalf("대기 열이 없다 :\n%s", out)
	}
}

func waitUseLine(at time.Time, id string) map[string]any {
	return map[string]any{
		"type": "assistant", "timestamp": tsText(at),
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": "gen"}},
		}},
	}
}

func waitResultLine(at time.Time, id string) map[string]any {
	return map[string]any{
		"type": "user", "timestamp": tsText(at),
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": "ok"},
		}},
	}
}

// mark stop 이 기록 구간 안의 대기를 같이 찍는다. 기록 구간에서 빼지는 않는다.
func TestMarkShowsWait(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "13-그림대기")
	t0 := m.Start
	fx.appendLines(fx.agentPath("a1"),
		waitUseLine(t0.Add(1*time.Minute), "toolu_g1"),
		waitResultLine(t0.Add(9*time.Minute), "toolu_g1"),
		mainLine(t0.Add(10*time.Minute), "assistant"),
	)
	fx.clock = t0.Add(10 * time.Minute)
	code, so, se := fx.run("stop", m.ID)
	if code != exitOK {
		t.Fatalf("stop 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "기록 구간 : 9분") || !strings.Contains(so, "대기 : 8분") {
		t.Fatalf("기록 구간 9분 · 대기 8분이 아니다 :\n%s", so)
	}
	code, so, _ = fx.run("list")
	if code != exitOK || !strings.Contains(so, "| 대기 |") || !strings.Contains(so, "8분") {
		t.Fatalf("mark list 에 대기 열이 없다 : %d\n%s", code, so)
	}
}
