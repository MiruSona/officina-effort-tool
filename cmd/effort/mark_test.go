package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 가짜 원본 뿌리 : <root>/<slug>/<세션>/subagents/agent-*.jsonl
const markSlug = "C--fake-proj"
const markSession = "sess1"

type markFixture struct {
	t     *testing.T
	home  string
	root  string
	clock time.Time
}

func newMarkFixture(t *testing.T) *markFixture {
	t.Helper()
	fx := &markFixture{t: t, home: t.TempDir(), root: t.TempDir(), clock: time.Now().UTC().Truncate(time.Second)}
	if err := os.MkdirAll(filepath.Join(fx.root, markSlug, markSession, "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	nowFunc = func() time.Time { return fx.clock }
	t.Cleanup(func() { nowFunc = old })
	return fx
}

func (fx *markFixture) agentPath(id string) string {
	return filepath.Join(fx.root, markSlug, markSession, "subagents", "agent-"+id+".jsonl")
}

// appendLines 는 JSONL 줄을 파일 끝에 더한다. 도는 갈래 파일에 줄이 붙는 것과 같다.
func (fx *markFixture) appendLines(path string, lines ...map[string]any) {
	fx.t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fx.t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		raw, _ := json.Marshal(l)
		f.Write(append(raw, '\n'))
	}
}

func tsText(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func toolUseLine(at time.Time, command string) map[string]any {
	return map[string]any{
		"type": "assistant", "timestamp": tsText(at),
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": map[string]any{"command": command}},
		}},
	}
}

func mainLine(at time.Time, typ string) map[string]any {
	return map[string]any{"type": typ, "timestamp": tsText(at), "message": map[string]any{"role": typ, "content": "…"}}
}

func sideLine(at time.Time, typ, subtype string) map[string]any {
	return map[string]any{"type": typ, "subtype": subtype, "timestamp": tsText(at)}
}

func (fx *markFixture) run(args ...string) (int, string, string) {
	fx.t.Helper()
	full := append([]string{"mark", args[0], "--home", fx.home, "--projects", fx.root, "--project", markSlug}, args[1:]...)
	return captureBoth(fx.t, full...)
}

func (fx *markFixture) marks() []store.TimeMark {
	fx.t.Helper()
	ms, _, err := store.New(fx.home).LoadMarks()
	if err != nil {
		fx.t.Fatal(err)
	}
	return ms
}

// 갈래 파일 셋 중 mark start "2-타일" tool_use 가 든 하나에만 묶인다.
// 「2-타일그림」을 부른 옆 갈래는 이름이 앞부분만 같아도 안 잡힌다.
func TestMarkBindsOwnFile(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.agentPath("a1"), mainLine(fx.clock.Add(-time.Minute), "user"), toolUseLine(fx.clock.Add(-time.Second), "ls"))
	fx.appendLines(fx.agentPath("a2"), mainLine(fx.clock.Add(-time.Minute), "user"), toolUseLine(fx.clock.Add(-time.Second), `.\bin\effort.exe mark start "2-타일"`))
	fx.appendLines(fx.agentPath("a3"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start 2-타일그림`))

	code, so, se := fx.run("start", "2-타일")
	if code != exitOK {
		t.Fatalf("start 종료 %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 1 || ms[0].File != markSession+"/agent-a2" || ms[0].Slug != markSlug || ms[0].Bind != "" {
		t.Fatalf("엉뚱한 파일에 묶였다 : %+v\n%s", ms, so)
	}
	if !strings.Contains(so, "effort mark stop "+ms[0].ID) {
		t.Fatalf("끝낼 명령 안내가 없다 :\n%s", so)
	}
}

// 두 파일에 같은 이름 → 묶기모호. mark 는 찍되 기록 구간은 비운다. 종료 0.
func TestMarkAmbiguous(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "2-타일"`))
	fx.appendLines(fx.agentPath("a2"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start '2-타일'`))
	code, so, se := fx.run("start", "2-타일")
	if code != exitOK {
		t.Fatalf("start 종료 %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 1 || ms[0].Bind != store.BindAmbiguous || ms[0].File != "" {
		t.Fatalf("묶기모호가 아니다 : %+v", ms)
	}
	fx.clock = fx.clock.Add(5 * time.Minute)
	code, so, _ = fx.run("stop", ms[0].ID)
	if code != exitOK || !strings.Contains(so, "기록 구간 : —") || !strings.Contains(so, store.BindAmbiguous) {
		t.Fatalf("기록 구간이 비어야 한다 : %d\n%s", code, so)
	}
}

// 아무 데도 없으면 묶기없음. 찍은 구간만 나온다.
func TestMarkNoBind(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), "ls"))
	code, so, se := fx.run("start", "3-없는판")
	if code != exitOK || !strings.Contains(so, store.BindNone) {
		t.Fatalf("start : %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 1 || ms[0].Bind != store.BindNone {
		t.Fatalf("묶기없음이 아니다 : %+v", ms)
	}
	fx.clock = fx.clock.Add(7 * time.Minute)
	code, so, _ = fx.run("stop", "3-없는판")
	if code != exitOK || !strings.Contains(so, "찍은 구간 : 7분") || !strings.Contains(so, "기록 구간 : —") {
		t.Fatalf("찍은 구간만 나와야 한다 : %d\n%s", code, so)
	}
}

// startBound 는 a1 파일에 mark start tool_use 를 적고 start 를 불러 묶인 mark 를 준다.
func startBound(t *testing.T, fx *markFixture, name string) store.TimeMark {
	t.Helper()
	fx.appendLines(fx.agentPath("a1"), mainLine(fx.clock.Add(-2*time.Minute), "user"),
		toolUseLine(fx.clock.Add(-time.Second), `effort mark start "`+name+`"`))
	code, so, se := fx.run("start", name)
	if code != exitOK {
		t.Fatalf("start 종료 %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	m := ms[len(ms)-1]
	if m.File == "" {
		t.Fatalf("안 묶였다 : %+v\n%s", m, so)
	}
	return m
}

// start~stop 안에 곁줄만 있는 끝부분은 기록 구간에 안 든다.
func TestMarkRecordSpanUsesMainLinesOnly(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "4-본줄")
	t0 := m.Start
	fx.appendLines(fx.agentPath("a1"),
		mainLine(t0.Add(1*time.Minute), "assistant"),
		mainLine(t0.Add(5*time.Minute), "user"),
		sideLine(t0.Add(9*time.Minute), "queue-operation", ""),
		sideLine(t0.Add(9*time.Minute+30*time.Second), "system", "away_summary"),
	)
	fx.clock = t0.Add(10 * time.Minute)
	code, so, se := fx.run("stop", m.ID)
	if code != exitOK {
		t.Fatalf("stop 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "기록 구간 : 4분") {
		t.Fatalf("곁줄이 기록 구간에 들었다 (4분이어야 한다) :\n%s", so)
	}
	if !strings.Contains(so, "찍은 구간 : 10분") || !strings.Contains(so, "어긋남") {
		t.Fatalf("찍은 구간·어긋남이 틀렸다 :\n%s", so)
	}
	// 두 번 닫지 않는다.
	if code, _, _ := fx.run("stop", m.ID); code != exitUsage {
		t.Fatalf("닫힌 mark 를 또 닫았다 : %d", code)
	}
}

// stop 없이 show → 진행중, 파일의 지금까지 마지막 본줄까지.
func TestMarkOpenShowsLive(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "5-도는판")
	t0 := m.Start
	fx.appendLines(fx.agentPath("a1"), mainLine(t0.Add(1*time.Minute), "assistant"), mainLine(t0.Add(3*time.Minute), "user"))
	fx.clock = t0.Add(5 * time.Minute)
	code, so, se := fx.run("show", "5-도는판")
	if code != exitOK {
		t.Fatalf("show 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "진행중") || !strings.Contains(so, "기록 구간 : 2분") || !strings.Contains(so, "찍은 구간 : 5분") {
		t.Fatalf("live 값이 틀렸다 :\n%s", so)
	}
	if !fx.marks()[0].Open() {
		t.Fatal("show 가 mark 를 닫았다")
	}
}

// 파일이 끝났는데 stop 이 없으면 파일의 마지막 본줄을 끝으로 보고 끝자동을 단다.
func TestMarkAutoEnd(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "6-잊은판")
	t0 := m.Start
	fx.appendLines(fx.agentPath("a1"), mainLine(t0.Add(1*time.Minute), "assistant"), mainLine(t0.Add(3*time.Minute), "assistant"))
	fx.clock = t0.Add(2 * time.Hour)
	code, so, se := fx.run("show", m.ID)
	if code != exitOK {
		t.Fatalf("show 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "끝자동") || strings.Contains(so, "진행중") {
		t.Fatalf("끝자동이 아니다 :\n%s", so)
	}
	if !strings.Contains(so, "기록 구간 : 2분") || !strings.Contains(so, "찍은 구간 : 3분") {
		t.Fatalf("끝자동 값이 틀렸다 :\n%s", so)
	}
	// 목록에도 같은 표시.
	code, so, _ = fx.run("list")
	if code != exitOK || !strings.Contains(so, "끝자동") || !strings.Contains(so, m.ID) {
		t.Fatalf("list 에 끝자동이 없다 : %d\n%s", code, so)
	}
}

// 시각·길이를 넣는 인자는 전부 사용법 오류다. 파일도 안 생긴다.
func TestMarkNoTimeArgs(t *testing.T) {
	fx := newMarkFixture(t)
	for _, args := range [][]string{
		{"start", "2-타일", "--at", "10:00"},
		{"start", "--at", "10:00", "2-타일"},
		{"start", "--min", "5", "2-타일"},
		{"start", "15"},
		{"start", "10:30"},
		{"start", "20분"},
		{"start", "2-타일", "15"},
		{"stop", "30m"},
		{"stop", "--at", "10:00"},
		{"show", "1.5"},
		{"start", "1h30m"},
		{"start", "1시간30분"},
	} {
		if code, so, se := fx.run(args...); code != exitUsage {
			t.Fatalf("%v : 종료 %d (1 이어야 한다)\n%s\n%s", args, code, so, se)
		}
	}
	if _, err := os.Stat(filepath.Join(fx.home, "marks.txt")); !os.IsNotExist(err) {
		t.Fatalf("막힌 명령이 marks.txt 를 만들었다 : %v", err)
	}
}

// 두 번 start/stop 해도 앞줄 바이트가 그대로다 — 덧붙이기만 한다.
func TestMarksFileAppendOnly(t *testing.T) {
	fx := newMarkFixture(t)
	path := filepath.Join(fx.home, "marks.txt")
	if code, so, se := fx.run("start", "7-첫판"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	fx.clock = fx.clock.Add(time.Minute)
	if code, so, se := fx.run("stop", "7-첫판"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	first, _ := os.ReadFile(path)
	fx.run("start", "7-둘째판")
	fx.clock = fx.clock.Add(time.Minute)
	fx.run("stop", "7-둘째판")
	second, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(second), string(first)) || len(second) <= len(first) {
		t.Fatalf("앞줄이 바뀌었거나 안 늘었다 :\n%s\n---\n%s", first, second)
	}
	if n := strings.Count(string(second), "\nstart\t"); n != 2 {
		t.Fatalf("start 줄 수 = %d", n)
	}
	if ms := fx.marks(); len(ms) != 2 || ms[0].Open() || ms[1].Open() {
		t.Fatalf("다시 읽은 mark 가 틀렸다 : %+v", ms)
	}
}

// scan --rebuild 뒤에도 marks.txt 가 그대로 있다.
func TestMarksSurviveRebuild(t *testing.T) {
	fx := newMarkFixture(t)
	if code, so, se := fx.run("start", "8-남는판"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	path := filepath.Join(fx.home, "marks.txt")
	before, _ := os.ReadFile(path)
	if code, out := capture(t, "scan", "--home", fx.home, "--projects", testdataSessions, "--all", "--rebuild"); code != exitOK {
		t.Fatalf("scan --rebuild 종료 %d\n%s", code, out)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("rebuild 가 marks.txt 를 건드렸다 : %v", err)
	}
}

// marks.txt 의 모르는 머리 줄은 경고하고 건너뛴다 (rules.txt R1 과 같은 규칙).
func TestMarkUnknownHeadSkipped(t *testing.T) {
	fx := newMarkFixture(t)
	body := "# effort marks\n" +
		"start\tm0926-abcd\t" + tsText(fx.clock) + "\t-\t-\t9-옛판\n" +
		"wait\tm0926-abcd\t3\n" +
		"stop\tm0926-abcd\t" + tsText(fx.clock.Add(4*time.Minute)) + "\n"
	if err := os.WriteFile(filepath.Join(fx.home, "marks.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, so, se := fx.run("list")
	if code != exitOK {
		t.Fatalf("list 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(se, `모르는 줄 머리 "wait"`) || !strings.Contains(se, "3번째 줄") {
		t.Fatalf("경고가 없다 :\n%s", se)
	}
	if !strings.Contains(so, "m0926-abcd") || !strings.Contains(so, "4분") {
		t.Fatalf("나머지 줄을 못 읽었다 :\n%s", so)
	}
}

func agentToolUseLine(at time.Time, prompt string) map[string]any {
	return map[string]any{
		"type": "assistant", "timestamp": tsText(at),
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_2", "name": "Agent", "input": map[string]any{"description": "갈래", "prompt": prompt}},
		}},
	}
}

// 부모가 Agent 프롬프트에 같은 명령을 적어도 부모 세션 파일은 안 잡힌다. 셸 도구 command 만 본다.
func TestMarkIgnoresParentAgentPrompt(t *testing.T) {
	fx := newMarkFixture(t)
	parent := filepath.Join(fx.root, markSlug, markSession+".jsonl")
	fx.appendLines(parent, agentToolUseLine(fx.clock.Add(-time.Minute), "시작할 때 `effort mark start \"2-타일\"` 을 부르고 끝에 mark stop"))
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "2-타일"`))
	code, so, se := fx.run("start", "2-타일")
	if code != exitOK {
		t.Fatalf("start 종료 %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 1 || ms[0].File != markSession+"/agent-a1" || ms[0].Bind != "" {
		t.Fatalf("갈래 파일 하나만 잡혀야 한다 : %+v\n%s", ms, so)
	}
}

// 인자 없는 stop : 이 판 기록 파일에서 mark stop 을 부른 줄로 제 mark 를 고른다.
func TestMarkStopWithoutArg(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "10-인자없음")
	fx.appendLines(fx.agentPath("a1"), mainLine(m.Start.Add(time.Minute), "assistant"))
	// 시험 시계를 묶기 창(2분) 안에 둔다. 파일 mtime 은 실제 시계라 창 밖으로 가면 후보에서 빠진다.
	fx.clock = m.Start.Add(90 * time.Second)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), `effort mark stop`))
	code, so, se := fx.run("stop")
	if code != exitOK || !strings.Contains(so, m.ID) {
		t.Fatalf("인자 없는 stop : %d\n%s\n%s", code, so, se)
	}
	if fx.marks()[0].Open() {
		t.Fatal("mark 가 안 닫혔다")
	}
}

// 두 갈래가 각자 mark 를 열고 동시에 인자 없이 stop·show 를 부르면 못 고른다 — 목록을 보이고 종료 1.
func TestMarkNoArgAmbiguousListsOpen(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "11-가"`))
	fx.appendLines(fx.agentPath("a2"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "11-나"`))
	if code, so, se := fx.run("start", "11-가"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	if code, so, se := fx.run("start", "11-나"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 2 || ms[0].File == "" || ms[1].File == "" || ms[0].File == ms[1].File {
		t.Fatalf("두 갈래가 각자 묶여야 한다 : %+v", ms)
	}
	fx.clock = fx.clock.Add(time.Minute)
	for _, verb := range []string{"stop", "show"} {
		fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), "effort mark "+verb))
		fx.appendLines(fx.agentPath("a2"), toolUseLine(fx.clock.Add(-time.Second), "effort mark "+verb))
		code, so, se := fx.run(verb)
		if code != exitUsage || !strings.Contains(se, ms[0].ID) || !strings.Contains(se, ms[1].ID) {
			t.Fatalf("%s : 목록을 보이고 종료 1 이어야 한다 : %d\n%s\n%s", verb, code, so, se)
		}
	}
	for _, m := range fx.marks() {
		if !m.Open() {
			t.Fatalf("못 고른 stop 이 mark 를 닫았다 : %+v", m)
		}
	}
}

// 못 쟀을 때 표시를 까닭 글로 푼다. 넷 다 제 까닭이 나오고 표시 이름도 괄호에 남는다.
func TestMarkFailReasons(t *testing.T) {
	cases := []struct {
		flag, want string
	}{
		{store.BindNone, "다시 찾아도"},
		{store.BindAmbiguous, "여럿"},
		{"파일못엶", "못 엶"},
		{flagNoMainInSpan, "본줄이 없음"},
	}
	for _, c := range cases {
		got := failReason([]string{flagOpen, c.flag})
		if !strings.Contains(got, c.want) || !strings.Contains(got, "("+c.flag+")") {
			t.Fatalf("%s 까닭 = %q", c.flag, got)
		}
	}
	if got := failReason([]string{flagReadFail}); !strings.Contains(got, "읽다가") || !strings.Contains(got, "("+flagReadFail+")") {
		t.Fatalf("%s 까닭 = %q", flagReadFail, got)
	}
	if got := failReason([]string{flagOpen}); got != "" {
		t.Fatalf("까닭 표시가 없는데 %q", got)
	}
	// 진행 중이면 stop 을 말하지 않는다. 닫힌 판은 start~stop 을 말한다.
	open := failReason([]string{flagOpen, flagNoMainInSpan})
	if !strings.Contains(open, "아직 start 뒤 본줄이 없음") || strings.Contains(open, "stop") {
		t.Fatalf("진행 중 까닭 = %q", open)
	}
	if closed := failReason([]string{flagNoMainInSpan}); !strings.Contains(closed, "start~stop") {
		t.Fatalf("닫힌 판 까닭 = %q", closed)
	}
}

// 진행 중 mark 는 stop 이 없으니 「stop 뒤 —」. 0 인 앞·뒤는 「없음」.
func TestMarkBranchLineOpen(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "16-도는갈래")
	fx.appendLines(fx.agentPath("a1"), mainLine(m.Start.Add(time.Minute), "assistant"))
	fx.clock = m.Start.Add(2 * time.Minute)
	code, so, se := fx.run("show", m.ID)
	if code != exitOK {
		t.Fatalf("show 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "mark 앞 2분 · stop 뒤 — ·") || strings.Contains(so, "stop 뒤 1분 미만") {
		t.Fatalf("진행 중 갈래 줄이 틀렸다 :\n%s", so)
	}
}

// 끝자동 mark 도 stop 이 없으니 「stop 뒤 —」.
func TestMarkBranchLineAutoEnd(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "17-잊은갈래")
	fx.appendLines(fx.agentPath("a1"), mainLine(m.Start.Add(time.Minute), "assistant"))
	fx.clock = m.Start.Add(2 * time.Hour)
	_, so, _ := fx.run("show", m.ID)
	if !strings.Contains(so, "끝자동") || !strings.Contains(so, "stop 뒤 —") {
		t.Fatalf("끝자동 갈래 줄이 틀렸다 :\n%s", so)
	}
}

// 같은 파일의 다른 mark 구간은 「mark 앞」·「stop 뒤」에서 뺀다. 겹친 것은 한 번만, 자기 구간과 겹친 몫은 안 센다.
func TestBranchSpansSubtractOtherMarks(t *testing.T) {
	t0 := time.Date(2026, 10, 5, 18, 0, 0, 0, time.Local)
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }
	span := func(a, b int) markSpan { return markSpan{from: at(a), to: at(b)} }
	min := int64(60 * 1000)
	cases := []struct {
		name                  string
		start, stop, last     int
		others                []markSpan
		before, after, other  int64
		warn                  bool
	}{
		{"이어 붙은 것", 10, 12, 13, []markSpan{span(1, 10)}, 1 * min, 1 * min, 9 * min, false},
		{"틈 3분", 9, 11, 12, []markSpan{span(1, 6)}, 4 * min, 1 * min, 5 * min, true},
		{"서로 겹친 것", 8, 12, 13, []markSpan{span(1, 10), span(2, 5)}, 1 * min, 1 * min, 7 * min, false},
		{"뒤에 이어 붙은 것", 2, 4, 15, []markSpan{span(4, 14)}, 2 * min, 1 * min, 10 * min, false},
	}
	for _, c := range cases {
		m := &store.TimeMark{ID: "m1005-aaaa", Start: at(c.start), Stop: at(c.stop), File: "s/agent-x"}
		v := markValue{end: m.Stop, fileFirst: t0, fileLast: at(c.last), others: c.others}
		total, before, after, other := branchSpans(m, v)
		if before != c.before || after != c.after || other != c.other {
			t.Fatalf("%s : 앞 %d · 뒤 %d · 다른 %d (바란 값 %d · %d · %d)", c.name, before, after, other, c.before, c.after, c.other)
		}
		if got := outsideWarn(total, before+after); got != c.warn {
			t.Fatalf("%s : 경고 %v", c.name, got)
		}
		if !strings.Contains(branchTotalText(m, v), fmt.Sprintf("· 다른 mark %s ·", spanText(c.other))) {
			t.Fatalf("%s : 다른 mark 칸이 없다 : %s", c.name, branchTotalText(m, v))
		}
	}
	// 다른 mark 가 없으면 칸을 안 찍는다.
	m := &store.TimeMark{ID: "m1005-aaaa", Start: at(2), Stop: at(4), File: "s/agent-x"}
	if got := branchTotalText(m, markValue{end: m.Stop, fileFirst: t0, fileLast: at(5)}); strings.Contains(got, "다른 mark") {
		t.Fatalf("다른 mark 가 없는데 칸을 찍었다 : %s", got)
	}
}

// 다른 mark 는 같은 파일에 묶인 것만 고른다. 자기 · 다른 파일 · 묶기없음은 빼고, 진행 중은 지금까지.
func TestOtherMarkSpans(t *testing.T) {
	now := time.Date(2026, 10, 5, 19, 0, 0, 0, time.UTC)
	me := store.TimeMark{ID: "m1", Slug: "p", File: "s/agent-a", Start: now.Add(-time.Hour), Stop: now.Add(-50 * time.Minute)}
	marks := []store.TimeMark{
		me,
		{ID: "m2", Slug: "p", File: "s/agent-a", Start: now.Add(-40 * time.Minute), Stop: now.Add(-30 * time.Minute)},
		{ID: "m3", Slug: "p", File: "s/agent-a", Start: now.Add(-10 * time.Minute)}, // 진행 중
		{ID: "m4", Slug: "p", File: "s/agent-b", Start: now.Add(-40 * time.Minute), Stop: now},
		{ID: "m5", Bind: store.BindNone, Start: now.Add(-40 * time.Minute), Stop: now},
	}
	got := otherMarkSpans(marks, &me, now)
	if len(got) != 2 || !got[0].to.Equal(now.Add(-30*time.Minute)) || !got[1].to.Equal(now) {
		t.Fatalf("다른 mark 구간 = %+v", got)
	}
}

// 한 갈래 파일에서 stop → 바로 다음 start 를 하면 앞 mark 의 「stop 뒤」에 뒤 mark 시간이 안 든다.
func TestMarkTwoMarksOneFile(t *testing.T) {
	fx := newMarkFixture(t)
	a := startBound(t, fx, "18-첫판")
	t0 := a.Start
	fx.appendLines(fx.agentPath("a1"), mainLine(t0.Add(time.Minute), "assistant"))
	fx.clock = t0.Add(90 * time.Second) // 묶기 창(2분) 안. 파일 mtime 은 실제 시계다
	if code, so, se := fx.run("stop", a.ID); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "18-둘째판"`))
	if code, so, se := fx.run("start", "18-둘째판"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	fx.appendLines(fx.agentPath("a1"), mainLine(t0.Add(5*time.Minute), "assistant"), mainLine(t0.Add(10*time.Minute), "assistant"))
	fx.clock = t0.Add(10 * time.Minute)
	if code, so, se := fx.run("stop", "18-둘째판"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	fx.appendLines(fx.agentPath("a1"), mainLine(t0.Add(11*time.Minute), "assistant"))
	_, so, _ := fx.run("show", a.ID)
	if !strings.Contains(so, "stop 뒤 1분 · 다른 mark 9분") || strings.Contains(so, "주의 :") {
		t.Fatalf("첫판 갈래 줄이 틀렸다 :\n%s", so)
	}
}

// 0 은 「없음」, 0 초과~1분 미만만 「1분 미만」. 첫·끝 날짜가 다르면 날짜를 붙인다.
func TestBranchTotalText(t *testing.T) {
	loc := time.Local
	first := time.Date(2026, 10, 5, 23, 58, 0, 0, loc)
	m := &store.TimeMark{ID: "m1005-aaaa", Start: first, Stop: first.Add(3 * time.Minute), File: "s/agent-x"}
	v := markValue{end: m.Stop, fileFirst: first, fileLast: first.Add(3*time.Minute + 20*time.Second)}
	got := branchTotalText(m, v)
	if !strings.Contains(got, "mark 앞 없음") || !strings.Contains(got, "stop 뒤 1분 미만") {
		t.Fatalf("없음·1분 미만 구분이 틀렸다 : %q", got)
	}
	if !strings.Contains(got, "(10-05 23:58:00~10-06 00:01:20)") {
		t.Fatalf("날짜가 다른데 날짜가 없다 : %q", got)
	}
	same := first.Add(-time.Hour)
	m.Start, v.fileFirst = same.Add(time.Minute), same
	m.Stop, v.end, v.fileLast = same.Add(5*time.Minute), same.Add(5*time.Minute), same.Add(5*time.Minute)
	got = branchTotalText(m, v)
	if !strings.Contains(got, "(22:58:00~23:03:00)") || !strings.Contains(got, "stop 뒤 없음") {
		t.Fatalf("같은 날은 시각만 · 뒤 0 은 없음 : %q", got)
	}
}

// start 바로 뒤 stop 해 구간 안 본줄이 없으면 구간안본줄없음을 달고 까닭 줄을 찍는다.
func TestMarkNoMainInSpanReason(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "13-빈판")
	fx.clock = m.Start.Add(30 * time.Second)
	code, so, se := fx.run("stop", m.ID)
	if code != exitOK {
		t.Fatalf("stop 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, flagNoMainInSpan) || !strings.Contains(so, "기록 구간을 못 쟀습니다 — 까닭 : ") {
		t.Fatalf("까닭 줄이 없다 :\n%s", so)
	}
}

// 10분 미만은 괄호에 분·초를 단다. 10분부터는 분만.
func TestSpanDetail(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{-1, "—"},
		{59 * 1000, "1분 미만 (59초)"},
		{(2*60 + 29) * 1000, "2분 (2분 29초)"},
		{(2*60 + 43) * 1000, "3분 (2분 43초)"},
		{(9*60 + 59) * 1000, "10분 (9분 59초)"},
		{10 * 60 * 1000, "10분"},
	}
	for _, c := range cases {
		if got := spanDetail(c.ms); got != c.want {
			t.Fatalf("spanDetail(%d) = %q, 원한 것 %q", c.ms, got, c.want)
		}
	}
}

// mark 밖 본줄이 2분 넘고 갈래 전체의 30% 넘을 때만 경고한다.
func TestOutsideWarnThreshold(t *testing.T) {
	min := int64(60 * 1000)
	cases := []struct {
		total, outside int64
		want           bool
	}{
		{4 * min, 77 * 1000, false},      // 밖 1:17 — 2분 안 (실물 m1005-6b1a 꼴)
		{4 * min, 2 * min, false},        // 밖 꼭 2분 — 넘어야 한다
		{10 * min, 150 * 1000, false},    // 밖 2:30 · 25% — 30% 안
		{6 * min, 150 * 1000, true},      // 밖 2:30 · 42%
		{10 * min, 3 * min, false},       // 밖 3분 · 꼭 30% — 넘어야 한다
		{20 * min, 3 * min, false},       // 밖 3분 · 15%
		{10 * min, 4 * min, true},        // 밖 4분 · 40%
	}
	for _, c := range cases {
		if got := outsideWarn(c.total, c.outside); got != c.want {
			t.Fatalf("outsideWarn(%d, %d) = %v", c.total, c.outside, got)
		}
	}
}

// 갈래 파일에 묶이면 「갈래 기록 전체」 줄을 찍는다 : 앞·뒤 길이, 압축 시각, 문턱 넘으면 경고.
func TestMarkBranchTotalLine(t *testing.T) {
	fx := newMarkFixture(t)
	m := startBound(t, fx, "14-갈래") // 파일 첫 본줄 = start 2분 앞 → 전체 = -2분~8분 = 10분, 밖 = 앞 2 + 뒤 5
	t0 := m.Start
	fx.appendLines(fx.agentPath("a1"),
		mainLine(t0.Add(1*time.Minute), "assistant"),
		sideLine(t0.Add(90*time.Second), "system", "compact_boundary"),
		mainLine(t0.Add(3*time.Minute), "assistant"),
		mainLine(t0.Add(8*time.Minute), "assistant"), // stop 뒤 보고 쓰기
	)
	// stop 은 3분에 쳤다. 그 뒤 줄은 시험이 미리 적어 둔 것 — show 로 나중에 보는 것과 같다.
	fx.clock = t0.Add(3 * time.Minute)
	code, so, se := fx.run("stop", m.ID)
	if code != exitOK {
		t.Fatalf("stop 종료 %d\n%s\n%s", code, so, se)
	}
	for _, want := range []string{"갈래 기록 전체 : 10분", "mark 앞 2분", "stop 뒤 5분", "압축 1번 (", "주의 : 이 기록 파일에 mark 밖 본줄이"} {
		if !strings.Contains(so, want) {
			t.Fatalf("%q 가 없다 :\n%s", want, so)
		}
	}
	if !strings.Contains(so, "mark show") {
		t.Fatalf("stop 때 「stop 뒤」 안내가 없다 :\n%s", so)
	}
}

// 메인 세션 파일에 묶이면 「갈래 기록 전체」 줄을 안 찍는다 — 세션 전체라 뜻이 없다.
func TestMarkMainSessionNoBranchLine(t *testing.T) {
	fx := newMarkFixture(t)
	parent := filepath.Join(fx.root, markSlug, markSession+".jsonl")
	fx.appendLines(parent, mainLine(fx.clock.Add(-time.Hour), "user"), toolUseLine(fx.clock.Add(-time.Second), `effort mark start "15-메인"`))
	if code, so, se := fx.run("start", "15-메인"); code != exitOK {
		t.Fatalf("%d\n%s\n%s", code, so, se)
	}
	m := fx.marks()[0]
	if m.File != markSession {
		t.Fatalf("메인 파일에 안 묶였다 : %+v", m)
	}
	fx.appendLines(parent, mainLine(m.Start.Add(time.Minute), "assistant"), mainLine(m.Start.Add(2*time.Minute), "assistant"))
	fx.clock = m.Start.Add(2 * time.Minute)
	_, so, _ := fx.run("stop", m.ID)
	if strings.Contains(so, "갈래 기록 전체") || strings.Contains(so, "mark 밖") {
		t.Fatalf("메인 파일인데 갈래 줄을 찍었다 :\n%s", so)
	}
}

// show 에 mark id 를 주면 못 찾은 끝에 mark show 를 귀띔한다. 종료 코드는 그대로 2.
func TestShowHintsMarkID(t *testing.T) {
	home := scanForGroups(t)
	code, _, se := captureBoth(t, "show", "--home", home, "m1004-50eb")
	if code != exitNoData || !strings.Contains(se, "mark id 꼴입니다. effort mark show m1004-50eb") {
		t.Fatalf("귀띔이 없다 : %d\n%s", code, se)
	}
	code, _, se = captureBoth(t, "show", "--home", home, "p-nosuch")
	if code != exitNoData || strings.Contains(se, "mark id") {
		t.Fatalf("mark id 가 아닌데 귀띔했다 : %d\n%s", code, se)
	}
}

// marks.txt 의 잘못된 줄 하나가 명령을 막지 않는다. 경고하고 그 줄만 건너뛴다.
func TestMarkBadLinesDoNotBlock(t *testing.T) {
	fx := newMarkFixture(t)
	body := "# effort marks\n" +
		"start\tm0926-aaaa\t" + tsText(fx.clock) + "\t-\t-\t12-좋은판\n" +
		"start\tm0926-bbbb\t어제\t-\t-\t시각틀림\n" +
		"start\tm0926-cccc\n" +
		"stop\tm0926-zzzz\t" + tsText(fx.clock) + "\n" +
		"start\tm0926-aaaa\t" + tsText(fx.clock) + "\t-\t-\t겹친id\n"
	if err := os.WriteFile(filepath.Join(fx.home, "marks.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	fx.clock = fx.clock.Add(2 * time.Minute)
	code, so, se := fx.run("stop", "12-좋은판")
	if code != exitOK || !strings.Contains(so, "찍은 구간 : 2분") {
		t.Fatalf("잘못된 줄 때문에 막혔다 : %d\n%s\n%s", code, so, se)
	}
	for _, want := range []string{"3번째 줄", "4번째 줄", "5번째 줄", "6번째 줄"} {
		if !strings.Contains(se, want) {
			t.Fatalf("%s 경고가 없다 :\n%s", want, se)
		}
	}
	if code, so, _ := fx.run("start", "12-새판"); code != exitOK {
		t.Fatalf("start 가 막혔다 : %d\n%s", code, so)
	}
}
