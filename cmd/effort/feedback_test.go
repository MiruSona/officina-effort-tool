package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 인자 꼴에는 사람눈금을 적을 자리가 없다. 빈 칸이 왜 비었는지 푸터로 말해야 한다.
func TestHumanFooterWhenBlank(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "estimate", "--home", home, "--human", "조사:S")
	if code != exitOK {
		t.Fatalf("estimate %d\n%s", code, out)
	}
	if !strings.Contains(out, "사람눈금 — :") {
		t.Fatalf("빈 칸 안내가 없다 :\n%s", out)
	}
}

func TestHumanFooterHiddenWhenFilled(t *testing.T) {
	home := scanForGroups(t)
	plan := filepath.Join(t.TempDir(), "소단계.md")
	body := "| 소단계 | 분류 | 크기 | 사람눈금 |\n| --- | --- | --- | --- |\n| 1. 훑기 | 조사 | M | 60 |\n"
	if err := os.WriteFile(plan, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "estimate", "--home", home, "--human", "--from", plan)
	if code != exitOK {
		t.Fatalf("estimate %d\n%s", code, out)
	}
	if strings.Contains(out, "사람눈금 — :") {
		t.Fatalf("다 찼는데 빈 칸 안내가 나왔다 :\n%s", out)
	}
}

// 모든 표에 붙는 「본줄 시간」 푸터.
func TestEstimateMainlineFooter(t *testing.T) {
	home := scanForGroups(t)
	_, out := capture(t, "estimate", "--home", home, "조사:S")
	if !strings.Contains(out, "이 값은 본줄 시간이다") {
		t.Fatalf("본줄 안내가 없다 :\n%s", out)
	}
}

// group --add 는 없는 작업 id 를 막으므로, 캐시에 없는 줄은 정본에 직접 적어 만든다.
func TestMissingMarkNoticeTellsWhy(t *testing.T) {
	home := scanForGroups(t)
	path := filepath.Join(home, "groups.txt")
	body := store.GroupsHead + "group\tg-fake01\t9. 없는 소단계\n" + "task\tg-fake01\tp-nowhere-0001\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// 경고는 stderr 로 간다 — stdout 에 섞으면 --json 이 깨진다.
	code, out, errOut := captureBoth(t, "list", "--home", home, "--group", "mark", "--all")
	if code != exitOK {
		t.Fatalf("list --group %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(errOut, "groups.txt") || !strings.Contains(errOut, "effort scan --all") {
		t.Fatalf("왜 빠졌는지 안 알려준다 :\n%s", errOut)
	}
}
