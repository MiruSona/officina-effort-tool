package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// gapTasksHome 은 「일 → 대화 → 일」 세 작업만 든 캐시를 만든다.
// 앞뒤 일 칸은 40분 떨어져 있고 사이 대화가 20분 자리에 있다 —
// 대화를 먼저 빼고 묶으면 두 묶음, 안 빼고 묶으면 한 묶음이 된다.
func gapTasksHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	st := store.New(home)
	if err := st.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnsureRules(); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-24 * time.Hour).Truncate(time.Minute)
	mk := func(id string, c model.Class, min int) model.Task {
		start := base.Add(time.Duration(min) * time.Minute)
		return model.Task{
			PromptID: id, SessionID: "s0000001", Class: c, ClassBy: "제목",
			Title: string(c) + " " + id,
			Start: start, End: start.Add(2 * time.Minute),
			WallMs: 2 * 60 * 1000, PureMs: 60 * 1000,
		}
	}
	tasks := []model.Task{
		mk("p1", model.ClassBuild, 0),
		mk("p2", model.ClassChat, 20),
		mk("p3", model.ClassBuild, 40),
	}
	if err := st.WriteTasks(tasks); err != nil {
		t.Fatal(err)
	}
	return home
}

func countGroupRows(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "자동-") {
			n++
		}
	}
	return n
}

// list --group 은 estimate 와 같이 「모든 작업을 묶은 뒤 일 칸 묶음만」 고른다.
// 사이에 낀 대화가 묶음을 쪼개면 배율을 잰 표본과 쓰는 표본이 달라진다.
func TestListGroupBuildsFromAllTasks(t *testing.T) {
	home := gapTasksHome(t)
	code, out := capture(t, "list", "--home", home, "--group", "gap:30m", "--limit", "0")
	if code != exitOK {
		t.Fatalf("종료 코드 %d\n%s", code, out)
	}
	if n := countGroupRows(out); n != 1 {
		t.Fatalf("묶음 %d개, 바란 값 1개 (대화를 먼저 빼서 쪼개졌다) :\n%s", n, out)
	}
	// 한 묶음에 작업 셋이 다 들어야 한다.
	if !strings.Contains(out, "| 3 ") && !strings.Contains(out, " 3 |") {
		t.Fatalf("작업 3건이 한 묶음이 아니다 :\n%s", out)
	}
}

// 일 칸이 아닌 묶음은 --all 일 때만 보인다.
func TestListGroupHidesNonWorkGroups(t *testing.T) {
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
		PromptID: "c1", SessionID: "s0000002", Class: model.ClassChat, Title: "대화 하나",
		Start: base, End: base.Add(time.Minute), WallMs: 60 * 1000,
	}}
	if err := st.WriteTasks(tasks); err != nil {
		t.Fatal(err)
	}
	if code, out := capture(t, "list", "--home", home, "--group", "gap:30m"); code != exitNoData {
		t.Fatalf("일 칸이 없는데 종료 코드 %d :\n%s", code, out)
	}
	code, out := capture(t, "list", "--home", home, "--group", "gap:30m", "--all")
	if code != exitOK || countGroupRows(out) != 1 {
		t.Fatalf("--all 인데 안 보인다 (%d) :\n%s", code, out)
	}
}

// 같은 캐시에서 list --group 의 묶음 수와 estimate 의 표본 수(n)가 같아야 한다.
func TestListGroupCountMatchesEstimateSamples(t *testing.T) {
	home := gapTasksHome(t)
	_, listOut := capture(t, "list", "--home", home, "--group", "gap:30m", "--class", "구현", "--limit", "0")
	got := countGroupRows(listOut)
	code, estOut := capture(t, "estimate", "--home", home, "--unit", "group", "--group", "gap:30m", "구현:M")
	if code != exitOK {
		t.Fatalf("estimate 종료 코드 %d\n%s", code, estOut)
	}
	want := "n=" + itoa(got)
	if !strings.Contains(estOut, want) {
		t.Fatalf("list 묶음 %d개인데 estimate 근거에 %s 가 없다 :\n%s", got, want, estOut)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
