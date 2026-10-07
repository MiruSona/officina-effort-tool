package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 미뤄 묶기 시험. 장면은 Docs/Research/2026-10-07-묶기없음원인조사.md 「되짚어 돌리기」를 옮겼다 :
// 메인 세션 파일에 줄 196(생각)까지만 있을 때 mark start → 묶기없음.
// 그 뒤 줄 197(mark start tool_use, exe 시각보다 0.521초 앞)과 198(도구 결과)이 써진다 → stop·show 때 묶인다.

const lateName = "2-effort-steps 손 시험"

func (fx *markFixture) mainPath() string {
	return filepath.Join(fx.root, markSlug, markSession+".jsonl")
}

func thinkingLine(at time.Time) map[string]any {
	return map[string]any{
		"type": "assistant", "timestamp": tsText(at),
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "thinking", "thinking": "…"}}},
	}
}

func toolResultLine(at time.Time) map[string]any {
	return map[string]any{
		"type": "user", "timestamp": tsText(at),
		"message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "content": "묶기없음"}}},
	}
}

func (fx *markFixture) bindLines() int {
	fx.t.Helper()
	raw, err := os.ReadFile(filepath.Join(fx.home, "marks.txt"))
	if err != nil {
		fx.t.Fatal(err)
	}
	return strings.Count(string(raw), "\nbind\t")
}

// startLate 는 줄 196 까지 쓴 메인 파일에서 mark start 를 불러 묶기없음 mark 를 만들고,
// 그 뒤 줄 197·198 을 덧붙인다 (메인이 tool_use 줄을 늦게 쓰는 것).
func startLate(t *testing.T, fx *markFixture) store.TimeMark {
	t.Helper()
	at := fx.clock // exe 가 찍는 mark 시각 (조사 문서 08:25:45.201)
	fx.appendLines(fx.mainPath(), mainLine(at.Add(-time.Minute), "user"), thinkingLine(at.Add(-1660*time.Millisecond)))
	code, so, se := fx.run("start", lateName)
	if code != exitOK {
		t.Fatalf("start 종료 %d\n%s\n%s", code, so, se)
	}
	ms := fx.marks()
	if len(ms) != 1 || ms[0].Bind != store.BindNone || ms[0].File != "" {
		t.Fatalf("줄 196 까지는 묶기없음이어야 한다 : %+v\n%s", ms, so)
	}
	if !strings.Contains(so, "다시 찾아 묶습니다") {
		t.Fatalf("다시 묶는다는 안내가 없다 :\n%s", so)
	}
	fx.appendLines(fx.mainPath(),
		toolUseLine(at.Add(-521*time.Millisecond), `./EffortTool/bin/effort.exe mark start "`+lateName+`" 2>&1 | tail -5`),
		toolResultLine(at.Add(267*time.Millisecond)))
	return ms[0]
}

// start 때 묶기없음 → show 때 묶임 → 기록 구간이 찬다. bind 줄은 한 번만 쓴다.
func TestMarkLateBindOnShowAndStop(t *testing.T) {
	fx := newMarkFixture(t)
	m := startLate(t, fx)
	fx.appendLines(fx.mainPath(), mainLine(m.Start.Add(time.Minute), "assistant"), mainLine(m.Start.Add(90*time.Second), "user"))
	fx.clock = m.Start.Add(100 * time.Second)

	code, so, se := fx.run("show", m.ID)
	if code != exitOK {
		t.Fatalf("show 종료 %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "묶은 기록 : "+markSlug+"/"+markSession) || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("show 때 메인 파일에 묶여야 한다 :\n%s", so)
	}
	if strings.Contains(so, "기록 구간 : —") || strings.Contains(so, store.BindNone) {
		t.Fatalf("기록 구간이 차야 한다 :\n%s", so)
	}
	ms := fx.marks()
	if !ms[0].Late || ms[0].File != markSession || ms[0].Slug != markSlug || ms[0].Bind != "" {
		t.Fatalf("다시 읽은 mark 가 묶이지 않았다 : %+v", ms[0])
	}
	if n := fx.bindLines(); n != 1 {
		t.Fatalf("bind 줄 수 = %d", n)
	}

	// 두 번째부터는 bind 줄로 바로 묶인다 — 다시 찾지도, 줄을 더 쓰지도 않는다.
	code, so, _ = fx.run("stop", m.ID)
	if code != exitOK || !strings.Contains(so, "기록 구간 : 1분 (1분 29초)") {
		t.Fatalf("stop 의 기록 구간이 틀렸다 (줄 198(+0.267초)~90초 = 1분 29초) : %d\n%s", code, so)
	}
	if n := fx.bindLines(); n != 1 {
		t.Fatalf("stop 뒤 bind 줄 수 = %d", n)
	}
	if fx.marks()[0].Open() {
		t.Fatal("mark 가 안 닫혔다")
	}
}

// tool_use 줄이 끝 256KB 밖으로 밀려나도 통째 읽기로 찾는다.
func TestMarkLateBindFullRead(t *testing.T) {
	fx := newMarkFixture(t)
	m := startLate(t, fx)
	pad := strings.Repeat("가", 4000)
	var lines []map[string]any
	for i := 0; i < 40; i++ {
		lines = append(lines, map[string]any{"type": "assistant", "timestamp": tsText(m.Start.Add(time.Duration(i+1) * time.Second)),
			"message": map[string]any{"role": "assistant", "content": pad}})
	}
	fx.appendLines(fx.mainPath(), lines...)
	if info, _ := os.Stat(fx.mainPath()); info.Size() < collect.LiveTailBytes {
		t.Fatalf("시험 파일이 너무 작다 : %d", info.Size())
	}
	// 줄 197 이 끝 256KB 밖에 있는지 본다 — 안 그러면 통째 읽기 길을 안 탄다.
	f, _ := os.Open(fx.mainPath())
	info, _ := f.Stat()
	_, inTail, _ := collect.FindToolUse(f, info.Size(), func(s string) bool { return collect.MatchMarkCommand(s, "start", lateName) })
	f.Close()
	if inTail {
		t.Fatal("시험 준비가 틀렸다 : tool_use 줄이 아직 끝 256KB 안에 있다")
	}
	fx.clock = m.Start.Add(60 * time.Second)
	code, so, se := fx.run("show", m.ID)
	if code != exitOK || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("통째 읽기로 못 묶었다 : %d\n%s\n%s", code, so, se)
	}
}

// 같은 이름이라도 mark 시각 +5초 뒤의 줄은 안 잡는다 (다음 mark 의 줄이다).
// 열린 동안은 포기를 안 적고, 닫힌 뒤 2분이 지나 못 찾으면 포기 줄을 한 번만 쓴다. list 는 포기를 안 쓴다.
func TestMarkLateBindWindowAndSettle(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.mainPath(), mainLine(fx.clock.Add(-2*time.Minute), "user"))
	code, so, se := fx.run("start", "3-다른판")
	if code != exitOK || !strings.Contains(so, store.BindNone) {
		t.Fatalf("start : %d\n%s\n%s", code, so, se)
	}
	id := fx.marks()[0].ID
	start := fx.clock
	fx.appendLines(fx.mainPath(), toolUseLine(start.Add(30*time.Second), `effort mark start "3-다른판"`))
	fx.clock = start.Add(time.Minute)
	if code, so, _ := fx.run("show", id); code != exitOK || strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("mark 뒤 줄에 묶였다 :\n%s", so)
	}
	fx.clock = start.Add(5 * time.Minute)
	fx.run("show", id)
	fx.run("list")
	if n := fx.bindLines(); n != 0 {
		t.Fatalf("열린 mark 에 포기를 적었다 : %d", n)
	}
	fx.run("stop", id)
	fx.run("show", id)
	fx.run("list")
	if n := fx.bindLines(); n != 1 {
		t.Fatalf("포기 줄은 한 번만 : %d", n)
	}
	if m := fx.marks()[0]; !m.Settled || m.File != "" || m.Bind != store.BindNone {
		t.Fatalf("포기가 안 적혔다 : %+v", m)
	}
}

// 메인 세션이 인자 없이 show 를 부르면 그 줄이 아직 없다. 지금 프로젝트 메인 파일에 묶인 안 닫힌 mark 로 고른다.
// stop(쓰기)은 짐작으로 안 닫는다 — id 를 받으라고 한다.
func TestMarkNoArgFromMainSession(t *testing.T) {
	fx := newMarkFixture(t)
	m := startLate(t, fx)
	fx.appendLines(fx.mainPath(), mainLine(m.Start.Add(30*time.Second), "assistant"))
	// 파일 mtime 은 실제 시계라 시험 시계를 묶기 창(2분) 안에 둔다.
	fx.clock = m.Start.Add(60 * time.Second)
	code, so, se := fx.run("show")
	if code != exitOK || !strings.Contains(so, m.ID) || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("인자 없는 show 가 메인 mark 를 못 골랐다 : %d\n%s\n%s", code, so, se)
	}
	code, so, se = fx.run("stop")
	if code != exitUsage || !strings.Contains(se, m.ID) || !fx.marks()[0].Open() {
		t.Fatalf("인자 없는 stop 은 id 를 받아야 한다 : %d\n%s\n%s", code, so, se)
	}
}

// 갈래가 제 mark 없이 인자 없는 stop 을 부르면 (줄이 바로 보인다) 메인 mark 를 대신 닫지 않는다.
func TestMarkNoArgAgentDoesNotTakeMainMark(t *testing.T) {
	fx := newMarkFixture(t)
	m := startLate(t, fx)
	fx.clock = m.Start.Add(60 * time.Second)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-time.Second), "effort mark stop"))
	code, so, se := fx.run("stop")
	if code != exitUsage {
		t.Fatalf("갈래의 인자 없는 stop 은 못 골라야 한다 : %d\n%s\n%s", code, so, se)
	}
	if !fx.marks()[0].Open() {
		t.Fatal("갈래가 메인 mark 를 닫았다")
	}
}

// 옛 marks.txt (bind 줄 없음) 는 그대로 읽히고, bind 줄의 꼴이 틀리면 그 줄만 건너뛴다.
func TestMarkBindLineParse(t *testing.T) {
	fx := newMarkFixture(t)
	body := "# effort marks\n" +
		"start\tm1007-aaaa\t" + tsText(fx.clock) + "\t-\t-\t1-가\t묶기없음\n" +
		"start\tm1007-bbbb\t" + tsText(fx.clock) + "\t-\t-\t1-나\t묶기없음\n" +
		"bind\tm1007-aaaa\t" + tsText(fx.clock) + "\t" + markSlug + "\t" + markSession + "\n" +
		"bind\tm1007-aaaa\t" + tsText(fx.clock) + "\t-\t-\t묶기없음\n" + // 두 번째 bind 는 버린다
		"bind\tm1007-bbbb\t틀린시각\t-\t-\n" +
		"bind\tm1007-zzzz\t" + tsText(fx.clock) + "\t-\t-\n"
	if err := os.WriteFile(filepath.Join(fx.home, "marks.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ms, warns, err := store.New(fx.home).LoadMarks()
	if err != nil || len(ms) != 2 {
		t.Fatalf("%v %+v", err, ms)
	}
	if a := ms[0]; !a.Late || a.File != markSession || a.Slug != markSlug || a.Bind != "" || a.Settled {
		t.Fatalf("첫 bind 가 안 얹혔다 : %+v", a)
	}
	if b := ms[1]; b.Settled || b.File != "" {
		t.Fatalf("틀린 bind 줄이 얹혔다 : %+v", b)
	}
	if len(warns) != 2 {
		t.Fatalf("경고 둘(틀린 시각 · 없는 id)이어야 한다 : %+v", warns)
	}
}
