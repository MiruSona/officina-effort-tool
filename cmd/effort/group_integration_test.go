package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanForGroups 는 알림 시험 자료를 훑고 그 저장소 자리를 준다.
func scanForGroups(t *testing.T) string {
	t.Helper()
	home, _ := scanNotify(t)
	return home
}

func TestEstimateUnknownSizeFails(t *testing.T) {
	home := scanForGroups(t)
	code, _ := capture(t, "estimate", "--home", home, "조사:M=30")
	if code != exitUsage {
		t.Fatalf("모르는 크기인데 종료 코드 %d", code)
	}
	if code, _ := capture(t, "estimate", "--home", home, "조사:XXL"); code != exitUsage {
		t.Fatalf("모르는 크기인데 종료 코드 %d", code)
	}
}

// Go 의 flag 는 첫 위치 인자에서 멈춘다. 뒤에 남은 옵션을 조용히 무시하면 안 된다.
func TestFlagAfterArgFails(t *testing.T) {
	home := scanForGroups(t)
	code, _ := capture(t, "estimate", "--home", home, "조사:M", "--metric", "pure")
	if code != exitUsage {
		t.Fatalf("위치 인자 뒤 옵션인데 종료 코드 %d", code)
	}
}

func TestGroupAddThenList(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "group", "--home", home, "--add", "1. 문서 소단계",
		"--class", "문서", "p-notify-0001", "p-notify-0002")
	if code != exitOK {
		t.Fatalf("group --add %d\n%s", code, out)
	}
	if !strings.Contains(out, "1. 문서 소단계") || !strings.Contains(out, "사람") {
		t.Fatalf("사람이 표시한 묶음이 안 보인다 :\n%s", out)
	}
	raw, err := os.ReadFile(filepath.Join(home, "groups.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "task\t") {
		t.Fatalf("정본에 안 적혔다 :\n%s", raw)
	}

	// scan --rebuild 를 해도 정본은 캐시가 아니라 안 날아간다.
	capture(t, "scan", "--home", home, "--projects", testdataNotify, "--all", "--rebuild")
	again, err := os.ReadFile(filepath.Join(home, "groups.txt"))
	if err != nil || string(again) != string(raw) {
		t.Fatal("rebuild 가 정본을 건드렸다")
	}

	// --drop 하면 그 묶음만 빠지고 주석은 남는다.
	code, out = capture(t, "group", "--home", home, "--drop", firstGroupID(t, string(raw)))
	if code != exitOK {
		t.Fatalf("group --drop %d\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(home, "groups.txt"))
	if strings.Contains(string(after), "1. 문서 소단계") {
		t.Fatalf("안 지워졌다 :\n%s", after)
	}
	if !strings.Contains(string(after), "# effort 소단계 정본") {
		t.Fatal("주석이 사라졌다")
	}
}

func firstGroupID(t *testing.T, text string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "group\t") {
			continue
		}
		f := strings.Split(line, "\t")
		return f[1]
	}
	t.Fatal("정본에 group 줄이 없다")
	return ""
}

func TestListGroupTable(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "list", "--home", home, "--group", "gap:30m", "--all")
	if code != exitOK {
		t.Fatalf("list --group %d\n%s", code, out)
	}
	if !strings.Contains(out, "묶음") || !strings.Contains(out, "자동-") {
		t.Fatalf("묶음 표가 아니다 :\n%s", out)
	}
	if code, _ := capture(t, "list", "--home", home, "--group", "모르는열쇠"); code != exitUsage {
		t.Fatal("모르는 묶기 열쇠를 안 막았다")
	}
}

func TestStatsByGroup(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "stats", "--home", home, "--by", "group", "--all")
	if code != exitOK {
		t.Fatalf("stats --by group %d\n%s", code, out)
	}
	if !strings.Contains(out, "자동-") {
		t.Fatalf("묶음 칸이 없다 :\n%s", out)
	}
}

// 묶음 단위 표본은 작업 단위보다 건수가 적다 (여럿을 하나로 묶으니까).
func TestEstimateUnitGroupUsesGroups(t *testing.T) {
	home := scanForGroups(t)
	_, group := capture(t, "estimate", "--home", home, "--days", "100000",
		"--unit", "group", "--group", "session", "문서:M")
	_, task := capture(t, "estimate", "--home", home, "--days", "100000", "--unit", "task", "문서:M")
	if group == task {
		t.Fatalf("묶음 단위와 작업 단위가 같다 :\n%s", group)
	}
	if code, _ := capture(t, "estimate", "--home", home, "--unit", "없는단위", "문서:M"); code != exitUsage {
		t.Fatal("모르는 --unit 을 안 막았다")
	}
}

func TestActualMatchByName(t *testing.T) {
	home := scanForGroups(t)
	if code, out := capture(t, "group", "--home", home, "--add", "1. 문서 소단계",
		"--class", "문서", "p-notify-0001", "p-notify-0002"); code != exitOK {
		t.Fatalf("group --add %d\n%s", code, out)
	}
	plan := filepath.Join(t.TempDir(), "공수표.md")
	body := "| 소단계 | 분류 | 크기 |\n| --- | --- | --- |\n| 1. 문서 소단계 | 문서 | M |\n| 9. 없는 소단계 | 구현 | M |\n"
	if err := os.WriteFile(plan, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "actual", "--home", home, "--from", plan)
	if code != exitOK {
		t.Fatalf("actual %d\n%s", code, out)
	}
	if !strings.Contains(out, "1. 문서 소단계") || !strings.Contains(out, "사람 g") {
		t.Fatalf("사람이 표시한 묶음에 안 붙었다 :\n%s", out)
	}
	// 못 찾은 소단계는 지어내지 않고 — 로 둔다.
	if !strings.Contains(out, "못 찾음") {
		t.Fatalf("못 찾은 줄이 없다 :\n%s", out)
	}
}

func TestActualNeedsFrom(t *testing.T) {
	home := scanForGroups(t)
	if code, _ := capture(t, "actual", "--home", home); code != exitUsage {
		t.Fatal("--from 없이 돌아갔다")
	}
}

func TestShowPrintsParentAndDepth(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "show", "--home", home, "p-notify-0001")
	if code != exitOK {
		t.Fatalf("show %d\n%s", code, out)
	}
	if !strings.Contains(out, "부모") || !strings.Contains(out, "pa") {
		t.Fatalf("부모 칸이 없다 :\n%s", out)
	}
	if !strings.Contains(out, "본줄") || !strings.Contains(out, "서브") {
		t.Fatalf("벽시계 내역이 없다 :\n%s", out)
	}
}
