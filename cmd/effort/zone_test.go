package main

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // 서머타임 시간대를 기계 설정 없이 읽는다

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// fixKST 는 시험 동안 로컬 시간대를 KST 로 고정한다. 시간대 표시가 기계마다 달라지면 안 된다.
func fixKST(t *testing.T) {
	t.Helper()
	old := time.Local
	time.Local = time.FixedZone("KST", 9*3600)
	t.Cleanup(func() { time.Local = old })
}

func TestListHeadShowsZone(t *testing.T) {
	fixKST(t)
	home := scanForGroups(t)
	code, out := capture(t, "list", "--home", home, "--limit", "3", "--all")
	if code != exitOK {
		t.Fatalf("list %d\n%s", code, out)
	}
	if !strings.Contains(out, "날짜(+09:00)") {
		t.Fatalf("머리에 시간대가 없다 :\n%s", out)
	}
}

// fixLA 는 서머타임이 있는 시간대로 고정한다. 1월(-08:00)과 7월(-07:00)이 갈린다.
func fixLA(t *testing.T) {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

func TestListDateColumnUsesRowOffsets(t *testing.T) {
	fixLA(t)
	jan := time.Date(2026, 1, 15, 9, 0, 0, 0, time.Local)
	jul := time.Date(2026, 7, 15, 9, 0, 0, 0, time.Local)

	head, layout := listDateColumn([]model.Task{{Start: jan}, {Start: jan}})
	if head != "날짜(-08:00)" || jan.Local().Format(layout) != "01-15 09:00" {
		t.Fatalf("겨울 작업만 : 머리 %q · 줄 %q", head, jan.Local().Format(layout))
	}
	head, layout = listDateColumn([]model.Task{{Start: jul}})
	if head != "날짜(-07:00)" {
		t.Fatalf("여름 작업만 : 머리 %q", head)
	}
	head, layout = listDateColumn([]model.Task{{Start: jan}, {Start: jul}})
	if head != "날짜" {
		t.Fatalf("섞였는데 머리에 시간대가 있다 : %q", head)
	}
	if got := jan.Local().Format(layout); got != "01-15 09:00 -08:00" {
		t.Fatalf("줄에 오프셋이 없다 : %q", got)
	}
	if got := jul.Local().Format(layout); got != "07-15 09:00 -07:00" {
		t.Fatalf("줄에 오프셋이 없다 : %q", got)
	}
}

func TestShowPrintsZone(t *testing.T) {
	fixKST(t)
	home := scanForGroups(t)
	code, out := capture(t, "show", "--home", home, "p-notify-0001")
	if code != exitOK {
		t.Fatalf("show %d\n%s", code, out)
	}
	if !strings.Contains(out, "+09:00") {
		t.Fatalf("시작 시각에 시간대가 없다 :\n%s", out)
	}
}
