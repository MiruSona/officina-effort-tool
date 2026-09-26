package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 모르는 종류가 든 rules.txt 로도 scan 은 끝까지 돈다. 경고는 stderr 로.
func TestScanWarnsButContinues(t *testing.T) {
	home := t.TempDir()
	if _, err := os.Stat(testdataSessions); err != nil {
		t.Skip("시험 세션 자료가 없다")
	}
	// 기본 규칙 한 벌 + 이 exe 가 모르는 종류 한 줄.
	code, out := capture(t, "rules", "--home", home)
	if code != exitOK {
		t.Fatalf("rules.txt 를 못 만들었다 : %d\n%s", code, out)
	}
	rulesPath := filepath.Join(home, "rules.txt")
	raw, _ := os.ReadFile(rulesPath)
	raw = append(raw, []byte("\n# --- 2026-12-01 자동으로 더한 기본값 (zzz) · effort fffffff ---\nzzz\tmax\t3\n")...)
	if err := os.WriteFile(rulesPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	code, so, se := captureBoth(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	if code != exitOK {
		t.Fatalf("scan 이 멈췄다 : %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(se, `모르는 종류 "zzz"`) || !strings.Contains(se, "effort fffffff 가 더한 줄") {
		t.Fatalf("stderr 에 경고가 없다 :\n%s", se)
	}
	if strings.Contains(so, `"zzz"`) {
		t.Fatalf("경고가 stdout 에 섞였다 :\n%s", so)
	}
	if _, err := os.Stat(filepath.Join(home, "cache", "tasks.jsonl")); err != nil {
		t.Fatalf("캐시가 안 써졌다 : %v", err)
	}
	// 읽기 명령은 한 줄로 줄인다.
	_, _, se = captureBoth(t, "list", "--home", home, "--group", "mark")
	if n := strings.Count(se, "모르는 줄"); n != 1 {
		t.Fatalf("읽기 명령 경고가 %d줄이다 :\n%s", n, se)
	}
}

// 캐시 판이 이 exe 보다 크면 덮지 않고 종료 1 + 다시 빌드 안내.
func TestScanRefusesNewerCache(t *testing.T) {
	home := t.TempDir()
	args := []string{"scan", "--home", home, "--projects", testdataSessions, "--all"}
	if code, out := capture(t, args...); code != exitOK {
		t.Fatalf("첫 scan : %d\n%s", code, out)
	}
	statePath := filepath.Join(home, "cache", "scanstate.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	bumped := strings.Replace(string(raw), `"schema":"4"`, `"schema":"99"`, 1)
	if bumped == string(raw) {
		t.Fatalf("판 글을 못 바꿨다 : %s", raw)
	}
	if err := os.WriteFile(statePath, []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, n := range []string{"cache/scanstate.json", "cache/tasks.jsonl", "cache/sessions.jsonl", "rules.txt"} {
		before[n], _ = os.ReadFile(filepath.Join(home, n))
	}
	// 빠진 종류가 있어도 덧붙이기 전에 멈춰야 한다 — rules.txt 도 그대로여야 한다.
	os.WriteFile(filepath.Join(home, "rules.txt"), []byte("word\t조사\t조사\n"), 0o644)
	before["rules.txt"] = []byte("word\t조사\t조사\n")
	code, _, se := captureBoth(t, args...)
	if code != exitUsage {
		t.Fatalf("새 판 캐시인데 종료 %d\n%s", code, se)
	}
	if !strings.Contains(se, "build.ps1") || !strings.Contains(se, "이 exe 는") {
		t.Fatalf("다시 빌드 안내가 없다 :\n%s", se)
	}
	for n, b := range before {
		after, _ := os.ReadFile(filepath.Join(home, n))
		if string(after) != string(b) {
			t.Fatalf("%s 가 바뀌었다", n)
		}
	}
}

// rules --check 는 엄격하다 (R4). 모르는 종류에서 종료 1.
func TestRulesCheckStrict(t *testing.T) {
	home := t.TempDir()
	if code, out := capture(t, "rules", "--home", home, "--check"); code != exitOK {
		t.Fatalf("기본 규칙이 check 를 못 지났다 : %d\n%s", code, out)
	}
	rulesPath := filepath.Join(home, "rules.txt")
	raw, _ := os.ReadFile(rulesPath)
	if err := os.WriteFile(rulesPath, append(raw, []byte("zzz\tmax\t3\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, se := captureBoth(t, "rules", "--home", home, "--check")
	if code != exitUsage || !strings.Contains(se, "zzz") {
		t.Fatalf("check 가 모르는 종류를 안 막았다 : %d\n%s", code, se)
	}
	// check 가 아닌 rules 는 경고만 하고 표를 찍는다.
	code, so, se := captureBoth(t, "rules", "--home", home)
	if code != exitOK || !strings.Contains(se, "zzz") || !strings.Contains(so, "규칙 파일") {
		t.Fatalf("rules 보기가 멈췄거나 경고가 없다 : %d\n%s\n%s", code, so, se)
	}
}
