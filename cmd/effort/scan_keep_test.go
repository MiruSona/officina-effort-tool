package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 전부 다시 읽어야 하는 판(규칙 바뀜)에 한 프로젝트만 훑어도
// 다른 프로젝트의 옛 기록은 캐시에 그대로 있어야 한다.
func TestScanKeepsOtherProjectsOnFullReread(t *testing.T) {
	home := t.TempDir()
	all := []string{"scan", "--home", home, "--projects", testdataSessions, "--all"}
	if code, out := capture(t, all...); code != exitOK {
		t.Fatalf("첫 스캔 종료 코드 %d\n%s", code, out)
	}
	before := sessionIDs(t, home)
	if len(before) < 2 {
		t.Fatalf("시험 전제가 깨졌다 : 세션이 %d개뿐이다", len(before))
	}

	// 규칙을 고쳐 전면 재읽기 판을 만든다.
	rules := filepath.Join(home, "rules.txt")
	raw, err := os.ReadFile(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, append(raw, []byte("contfirst\t어어\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	one := []string{"scan", "--home", home, "--projects", testdataSessions, "--project", "small"}
	code, out := capture(t, one...)
	if code != exitOK {
		t.Fatalf("한 프로젝트 스캔 종료 코드 %d\n%s", code, out)
	}
	if !strings.Contains(out, "옛 분류 그대로 둡니다") {
		t.Fatalf("안 훑는 프로젝트 안내가 없다 :\n%s", out)
	}
	after := sessionIDs(t, home)
	for id := range before {
		if !after[id] {
			t.Fatalf("다른 프로젝트 세션 %s 이(가) 캐시에서 사라졌다", id)
		}
	}
}

// 옛 캐시가 깨졌으면 조용히 버리지 말고 멈추고 --rebuild 를 안내해야 한다.
func TestScanFailsOnBrokenCache(t *testing.T) {
	home := t.TempDir()
	all := []string{"scan", "--home", home, "--projects", testdataSessions, "--all"}
	if code, out := capture(t, all...); code != exitOK {
		t.Fatalf("첫 스캔 종료 코드 %d\n%s", code, out)
	}
	broken := filepath.Join(home, "cache", "tasks.jsonl")
	if err := os.WriteFile(broken, []byte("{이건 JSON 이 아니다\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, all...)
	if code != exitRead {
		t.Fatalf("깨진 캐시인데 종료 코드가 %d 다 (기대 %d)\n%s", code, exitRead, out)
	}
	// 안내 글은 표준오류로 나가 여기서는 안 잡힌다. 종료 코드로만 본다.
}

func sessionIDs(t *testing.T, home string) map[string]bool {
	t.Helper()
	sessions, err := store.New(home).ReadSessions()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for i := range sessions {
		out[sessions[i].SessionID] = true
	}
	return out
}
