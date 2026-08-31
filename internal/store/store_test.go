package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/efforttool/internal/model"
)

func TestEnsureRulesDoesNotOverwrite(t *testing.T) {
	s := New(t.TempDir())
	created, err := s.EnsureRules()
	if err != nil || !created {
		t.Fatalf("처음 만들기 실패 : %v %v", created, err)
	}
	if err := os.WriteFile(s.RulesPath(), []byte("word\t조사\t조사\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = s.EnsureRules()
	if err != nil || created {
		t.Fatalf("있는 파일을 다시 만들었다 : %v %v", created, err)
	}
	raw, _ := os.ReadFile(s.RulesPath())
	if string(raw) != "word\t조사\t조사\n" {
		t.Fatal("사람이 고친 rules.txt 를 덮어썼다")
	}
}

// 덧붙이기는 사람이 쓴 줄을 그대로 두고 빠진 종류만 끝에 더한다.
func TestAppendMissingKindsPreservesUserLines(t *testing.T) {
	s := New(t.TempDir())
	user := "word\t조사\t내낱말\n"
	if err := os.MkdirAll(s.Home(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.RulesPath(), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	added, err := s.AppendMissingKinds(rules.MissingKinds())
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 3 {
		t.Fatalf("더한 종류 = %v", added)
	}
	raw, err := os.ReadFile(s.RulesPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), user) {
		t.Fatalf("사람이 쓴 줄이 사라졌다 :\n%s", raw)
	}
	got, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if got.Words["내낱말"] != model.ClassResearch {
		t.Fatalf("사람 낱말 규칙이 죽었다 : %v", got.Words)
	}
	if len(got.Set("쓰기")) == 0 || len(got.Prefixes) == 0 || len(got.ChoreWord) == 0 {
		t.Fatalf("tool·runpre·chore 가 안 생겼다 : %+v", got)
	}
	if left := got.MissingKinds(); len(left) != 0 {
		t.Fatalf("아직 빠진 종류가 있다 : %v", left)
	}
}

// 방아쇠는 「그 종류가 0줄」일 때뿐이라 두 번 돌려도 파일이 그대로여야 한다.
func TestAppendMissingKindsIdempotent(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.EnsureRules(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.RulesPath(), []byte("word\t조사\t내낱말\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		rules, err := s.LoadRules()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AppendMissingKinds(rules.MissingKinds()); err != nil {
			t.Fatal(err)
		}
	}
	first, err := os.ReadFile(s.RulesPath())
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	added, err := s.AppendMissingKinds(rules.MissingKinds())
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Fatalf("다 있는데 또 더했다 : %v", added)
	}
	second, _ := os.ReadFile(s.RulesPath())
	if string(first) != string(second) {
		t.Fatalf("두 번 돌렸더니 파일이 달라졌다 :\n%s\n---\n%s", first, second)
	}
}

// 끝에 개행이 없는 파일도 줄이 붙어 버리면 안 된다.
func TestAppendMissingKindsNoTrailingNewline(t *testing.T) {
	s := New(t.TempDir())
	if err := os.MkdirAll(s.Home(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.RulesPath(), []byte("word\t조사\t내낱말"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendMissingKinds(rules.MissingKinds()); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.RulesPath())
	if !strings.HasPrefix(string(raw), "word\t조사\t내낱말\n") {
		t.Fatalf("줄이 붙어 버렸다 :\n%s", raw)
	}
	got, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if got.Words["내낱말"] != model.ClassResearch {
		t.Fatalf("사람 낱말 규칙이 깨졌다 : %v", got.Words)
	}
}

func TestSanitizeIsSinglePlace(t *testing.T) {
	s := New(t.TempDir())
	tasks := []model.Task{{
		PromptID: "p1", SessionID: "s1",
		Title:  "제목 | 표 깨기 `백틱` <tag>",
		Agents: []model.Agent{{AgentID: "a1", Description: "설명\n줄바꿈\t탭"}},
	}}
	if err := s.WriteTasks(tasks); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got[0].Title, "|`<") {
		t.Fatalf("살균 안 된 제목이 캐시에 들어갔다 : %q", got[0].Title)
	}
	if strings.ContainsAny(got[0].Agents[0].Description, "\n\t") {
		t.Fatalf("살균 안 된 설명이 캐시에 들어갔다 : %q", got[0].Agents[0].Description)
	}
}

func TestSecretNeverReachesCache(t *testing.T) {
	s := New(t.TempDir())
	value := "ghp_" + strings.Repeat("B", 30)
	if err := s.WriteTasks([]model.Task{{PromptID: "p1", Title: "열쇠 " + value}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.cacheDir(), "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), value) {
		t.Fatal("비밀 값이 캐시 파일에 남았다")
	}
	if !strings.Contains(string(raw), "가려짐") {
		t.Fatal("가림 표시가 없다")
	}
}

func TestAtomicWriteNoPartialFile(t *testing.T) {
	s := New(t.TempDir())
	if err := s.WriteTasks([]model.Task{{PromptID: "old"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.cacheDir(), "tasks.jsonl")
	// tmp 파일만 남기고 죽은 흉내 — 본 파일은 옛 판 그대로여야 한다.
	if err := os.WriteFile(path+".tmp", []byte("반쯤 쓰다 만 것"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PromptID != "old" {
		t.Fatalf("본 파일이 망가졌다 : %+v", got)
	}
}

func TestReadTasksNoCache(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.ReadTasks(); err != ErrNoCache {
		t.Fatalf("err = %v, 바란 값 ErrNoCache", err)
	}
}

func TestLockNotStolenWhileAlive(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := AcquireLock(dir); err == nil {
		t.Fatal("산 락을 뺏었다")
	}
}

func TestLockStalePIDReused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scan.lock")
	// PID 는 같은데 시작시각이 다르면 재사용된 번호다 — 죽은 락이다.
	body := fmt.Sprintf("%d %d", os.Getpid(), processStartMs-999999)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("죽은 락을 못 치웠다 : %v", err)
	}
	lock.Release()
}

func TestLockReleaseOnlyOwn(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "scan.lock")
	if err := os.WriteFile(path, []byte("999999 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("남의 락을 지웠다")
	}
}

func TestScanStateSchemaBump(t *testing.T) {
	s := New(t.TempDir())
	st := NewScanState()
	st.Files["a"] = FileState{Size: 1, ModTime: 2}
	if err := s.SaveScanState(st); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(s.cacheDir(), "scanstate.json"))
	bumped := strings.Replace(string(raw), `"schema":"`+SchemaVersion+`"`, `"schema":"옛판"`, 1)
	if err := os.WriteFile(filepath.Join(s.cacheDir(), "scanstate.json"), []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}
	got := s.LoadScanState()
	if len(got.Files) != 0 {
		t.Fatal("스키마 판이 다른데 옛 기록을 그대로 썼다")
	}
}

func TestScanStateUnchanged(t *testing.T) {
	st := NewScanState()
	st.Files["a"] = FileState{Size: 10, ModTime: 100}
	if !st.Unchanged("a", 10, 100) {
		t.Fatal("안 바뀐 파일을 바뀐 것으로 봤다")
	}
	if st.Unchanged("a", 20, 100) {
		t.Fatal("커진 파일을 그대로로 봤다")
	}
	if !st.NeedsFullRead("a", 5, 100) {
		t.Fatal("줄어든 파일은 전체 재읽기다")
	}
}
