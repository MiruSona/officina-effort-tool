package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 미뤄 묶기 코드 리뷰(2026-10-07)에서 나온 장면들.

// writeMarks 는 marks.txt 를 통째로 쓴다. start 때 이미 어떻게 됐는지를 꾸며 둘 때 쓴다.
func (fx *markFixture) writeMarks(lines ...string) {
	fx.t.Helper()
	body := "# effort marks\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(fx.home, "marks.txt"), []byte(body), 0o644); err != nil {
		fx.t.Fatal(err)
	}
}

func startText(id string, at time.Time, slug, file, name string) string {
	if slug == "" {
		slug, file = "-", "-"
		return strings.Join([]string{"start", id, tsText(at), slug, file, name, store.BindNone}, "\t")
	}
	return strings.Join([]string{"start", id, tsText(at), slug, file, name}, "\t")
}

func stopText(id string, at time.Time) string {
	return strings.Join([]string{"stop", id, tsText(at)}, "\t")
}

func (fx *markFixture) mark(id string) *store.TimeMark {
	fx.t.Helper()
	ms := fx.marks()
	for i := range ms {
		if ms[i].ID == id {
			return &ms[i]
		}
	}
	fx.t.Fatalf("mark %s 가 없다", id)
	return nil
}

// 리뷰 1(가) · 시험 13(e) : 다른 프로젝트 메인 파일의 안 닫힌 mark 를 인자 없는 stop 이 닫지 않는다.
func TestReviewNoArgIgnoresOtherProjectMain(t *testing.T) {
	fx := newMarkFixture(t)
	other := filepath.Join(fx.root, "C--other-proj")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	fx.appendLines(filepath.Join(other, "sessX.jsonl"), mainLine(fx.clock.Add(-10*time.Second), "assistant"))
	fx.writeMarks(startText("m1007-0e01", fx.clock.Add(-time.Minute), "C--other-proj", "sessX", "1-남의판"))
	for _, verb := range []string{"stop", "show"} {
		if code, so, se := fx.run(verb); code != exitUsage {
			t.Fatalf("%s : 다른 프로젝트 mark 를 골랐다 : %d\n%s\n%s", verb, code, so, se)
		}
	}
	if !fx.mark("m1007-0e01").Open() {
		t.Fatal("다른 프로젝트 mark 가 닫혔다")
	}
}

// 리뷰 1(나) · 시험 13(d) : 갈래의 stop 줄이 15초보다 오래됐고 그 갈래 mark 가 이미 닫혔어도 메인 mark 를 대신 닫지 않는다.
func TestReviewNoArgOldAgentStopDoesNotTakeMain(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.mainPath(), mainLine(fx.clock.Add(-10*time.Second), "assistant"))
	fx.appendLines(fx.agentPath("a1"), toolUseLine(fx.clock.Add(-40*time.Second), "effort mark stop"))
	fx.writeMarks(
		startText("m1007-0a01", fx.clock.Add(-5*time.Minute), markSlug, markSession+"/agent-a1", "1-갈래"),
		stopText("m1007-0a01", fx.clock.Add(-40*time.Second)),
		startText("m1007-0b01", fx.clock.Add(-time.Minute), markSlug, markSession, "2-메인"),
	)
	code, so, se := fx.run("stop")
	if code != exitUsage || !fx.mark("m1007-0b01").Open() {
		t.Fatalf("갈래의 묵은 stop 이 메인 mark 를 닫았다 : %d\n%s\n%s", code, so, se)
	}
}

// 리뷰 1 : 메인 대체 길로 고른 것은 show 만 된다. stop(쓰기)은 id 를 받으라고 한다.
func TestReviewNoArgFallbackStopNeedsID(t *testing.T) {
	fx := newMarkFixture(t)
	fx.appendLines(fx.mainPath(), mainLine(fx.clock.Add(-10*time.Second), "assistant"))
	fx.writeMarks(startText("m1007-0c01", fx.clock.Add(-time.Minute), markSlug, markSession, "2-메인"))
	code, so, se := fx.run("show")
	if code != exitOK || !strings.Contains(so, "m1007-0c01") {
		t.Fatalf("show 는 메인 mark 를 골라야 한다 : %d\n%s\n%s", code, so, se)
	}
	code, so, se = fx.run("stop")
	if code != exitUsage || !strings.Contains(se, "m1007-0c01") || !fx.mark("m1007-0c01").Open() {
		t.Fatalf("stop 은 id 를 받으라고 해야 한다 : %d\n%s\n%s", code, so, se)
	}
}

// 리뷰 2 : 권한 확인을 기다리느라 tool_use 줄이 mark 시각보다 3분 앞이어도 묶인다.
func TestReviewLateBindPermissionWait(t *testing.T) {
	fx := newMarkFixture(t)
	at := fx.clock
	fx.appendLines(fx.mainPath(), mainLine(at.Add(-5*time.Minute), "user"))
	fx.writeMarks(startText("m1007-0d01", at, "", "", "3-권한대기"))
	fx.appendLines(fx.mainPath(),
		toolUseLine(at.Add(-3*time.Minute), `effort mark start "3-권한대기"`),
		toolResultLine(at.Add(time.Second)))
	fx.clock = at.Add(30 * time.Second)
	code, so, se := fx.run("show", "m1007-0d01")
	if code != exitOK || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("권한 대기 장면을 못 묶었다 : %d\n%s\n%s", code, so, se)
	}
}

// 리뷰 4 · 시험 13(b) : 같은 세션에서 같은 이름을 두 번 써도 둘째 mark 가 묶인다.
func TestReviewSameNameTwiceInOneSession(t *testing.T) {
	fx := newMarkFixture(t)
	t1 := fx.clock.Add(-10 * time.Minute)
	t2 := fx.clock
	fx.appendLines(fx.mainPath(),
		toolUseLine(t1.Add(-500*time.Millisecond), `effort mark start "4-같은이름"`),
		mainLine(t1.Add(time.Minute), "assistant"),
		toolUseLine(t2.Add(-500*time.Millisecond), `effort mark start "4-같은이름"`),
		toolResultLine(t2.Add(time.Second)))
	fx.writeMarks(
		startText("m1007-1a01", t1, markSlug, markSession, "4-같은이름"),
		stopText("m1007-1a01", t1.Add(5*time.Minute)),
		startText("m1007-1b01", t2, "", "", "4-같은이름"),
	)
	fx.clock = t2.Add(30 * time.Second)
	code, so, se := fx.run("show", "m1007-1b01")
	if code != exitOK || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("둘째 mark 를 못 묶었다 : %d\n%s\n%s", code, so, se)
	}
}

// 리뷰 5 · 시험 13(c) : 같은 이름 mark 둘이 30초 사이로 안 묶였을 때, 뒤 것 하나만 rebind 해도 제 줄에 묶인다.
func TestReviewSameNameTwoUnboundRebindOne(t *testing.T) {
	fx := newMarkFixture(t)
	s1 := fx.clock.Add(-30 * time.Second)
	s2 := fx.clock
	fx.appendLines(fx.agentPath("a1"), toolUseLine(s1.Add(-500*time.Millisecond), `effort mark start "5-겹침"`))
	fx.appendLines(fx.mainPath(), toolUseLine(s2.Add(-500*time.Millisecond), `effort mark start "5-겹침"`))
	fx.writeMarks(startText("m1007-2a01", s1, "", "", "5-겹침"), startText("m1007-2b01", s2, "", "", "5-겹침"))
	fx.clock = s2.Add(20 * time.Second)
	code, so, se := fx.run("show", "m1007-2b01")
	if code != exitOK {
		t.Fatalf("show : %d\n%s\n%s", code, so, se)
	}
	if b := fx.mark("m1007-2b01"); b.File != markSession {
		t.Fatalf("뒤 mark 는 메인 파일에 묶여야 한다 : %+v\n%s", b, so)
	}
	if a := fx.mark("m1007-2a01"); a.File != markSession+"/agent-a1" {
		t.Fatalf("앞 mark 도 같이 풀려 갈래 파일에 묶여야 한다 : %+v", a)
	}
}

// 리뷰 6 : start 때 묶기도 다른 mark 몫인 묵은 같은 이름 줄에는 안 묶인다.
func TestReviewStartIgnoresOldSameNameLine(t *testing.T) {
	fx := newMarkFixture(t)
	old := fx.clock.Add(-90 * time.Second)
	fx.appendLines(fx.agentPath("a1"), toolUseLine(old.Add(-500*time.Millisecond), `effort mark start "6-다시"`))
	fx.writeMarks(startText("m1007-3a01", old, markSlug, markSession+"/agent-a1", "6-다시"), stopText("m1007-3a01", old.Add(time.Minute)))
	code, so, se := fx.run("start", "6-다시")
	if code != exitOK || !strings.Contains(so, store.BindNone) {
		t.Fatalf("묵은 같은 이름 줄에 묶였다 : %d\n%s\n%s", code, so, se)
	}
}

// 시험 13(f) : list 에서도 묶인다. list 는 포기 줄을 안 쓴다.
func TestReviewListBindsButNeverSettles(t *testing.T) {
	fx := newMarkFixture(t)
	at := fx.clock
	fx.appendLines(fx.mainPath(), toolUseLine(at.Add(-time.Second), `effort mark start "7-목록"`))
	fx.writeMarks(
		startText("m1007-4a01", at, "", "", "7-목록"),
		startText("m1007-4b01", at.Add(-time.Hour), "", "", "7-없는판"),
		stopText("m1007-4b01", at.Add(-50*time.Minute)),
	)
	fx.clock = at.Add(time.Minute)
	if code, so, se := fx.run("list"); code != exitOK {
		t.Fatalf("list : %d\n%s\n%s", code, so, se)
	}
	if a := fx.mark("m1007-4a01"); a.File != markSession || !a.Late {
		t.Fatalf("list 에서 못 묶었다 : %+v", a)
	}
	if b := fx.mark("m1007-4b01"); b.Settled {
		t.Fatalf("list 가 포기 줄을 썼다 : %+v", b)
	}
}

// 리뷰 3 : 포기는 닫힌 mark 에만 적는다. 열린 mark 는 2분이 지나도 계속 찾는다.
// 포기 뒤에도 show --rebind 로 다시 찾고, 성공한 bind 가 포기를 덮는다.
func TestReviewSettleOnlyWhenStoppedAndRebindOverrides(t *testing.T) {
	fx := newMarkFixture(t)
	at := fx.clock
	fx.appendLines(fx.mainPath(), mainLine(at.Add(-time.Minute), "user"))
	fx.writeMarks(startText("m1007-5a01", at, "", "", "8-포기"))
	fx.clock = at.Add(10 * time.Minute)
	fx.run("show", "m1007-5a01")
	if m := fx.mark("m1007-5a01"); m.Settled {
		t.Fatal("열린 mark 에 포기를 적었다")
	}
	if code, so, se := fx.run("stop", "m1007-5a01"); code != exitOK || !strings.Contains(so, store.BindNone) {
		t.Fatalf("stop : %d\n%s\n%s", code, so, se)
	}
	if m := fx.mark("m1007-5a01"); !m.Settled {
		t.Fatal("닫힌 뒤에도 포기가 안 적혔다")
	}
	// 줄이 나중에 나타났다 (다른 뿌리에서 옮겨 왔다 치자). 그냥 show 는 다시 안 찾고, --rebind 는 찾는다.
	fx.appendLines(fx.mainPath(), toolUseLine(at.Add(-time.Second), `effort mark start "8-포기"`))
	if _, so, _ := fx.run("show", "m1007-5a01"); strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("포기 뒤 그냥 show 가 다시 찾았다 :\n%s", so)
	}
	code, so, se := fx.run("show", "--rebind", "m1007-5a01")
	if code != exitOK || !strings.Contains(so, "다시 찾아 묶음") {
		t.Fatalf("--rebind 가 못 묶었다 : %d\n%s\n%s", code, so, se)
	}
	if m := fx.mark("m1007-5a01"); m.File != markSession || m.Settled {
		t.Fatalf("성공한 bind 가 포기를 못 덮었다 : %+v", m)
	}
}

// 리뷰 10 : 포기 전이면 까닭이 「아직 못 찾음」이다.
func TestReviewNotYetReason(t *testing.T) {
	fx := newMarkFixture(t)
	fx.writeMarks(startText("m1007-6a01", fx.clock, "", "", "9-아직"))
	_, so, _ := fx.run("show", "m1007-6a01")
	if !strings.Contains(so, "아직 못 찾음") || strings.Contains(so, "다시 찾아도") {
		t.Fatalf("포기 전 까닭이 틀렸다 :\n%s", so)
	}
}
