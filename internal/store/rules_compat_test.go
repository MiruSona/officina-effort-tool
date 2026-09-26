package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// R7 : 자동 추가 머리 주석에 더한 exe 판을 적는다.
func TestAppendHeaderHasVersion(t *testing.T) {
	s := New(t.TempDir())
	if err := os.MkdirAll(s.Home(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.RulesPath(), []byte("word\t조사\t조사\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, _, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendMissingKinds(rules.MissingKinds(), "35864f5-dirty"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.RulesPath())
	if !strings.Contains(string(raw), "자동으로 더한 기본값") || !strings.Contains(string(raw), "· effort 35864f5-dirty ---") {
		t.Fatalf("머리 주석에 판 글이 없다 :\n%s", raw)
	}
	// 판 글이 비면 dev 로 적는다 (설계 U6).
	s2 := New(t.TempDir())
	os.MkdirAll(s2.Home(), 0o755)
	os.WriteFile(s2.RulesPath(), []byte("word\t조사\t조사\n"), 0o644)
	r2, _, _ := s2.LoadRules()
	if _, err := s2.AppendMissingKinds(r2.MissingKinds(), ""); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(s2.RulesPath())
	if !strings.Contains(string(raw2), "· effort dev ---") {
		t.Fatalf("빈 판 글이 dev 로 안 적혔다 :\n%s", raw2)
	}
}

// 캐시 판이 내 판보다 크면 Newer. 수가 아니면 새것으로 안 본다.
func TestScanStateNewer(t *testing.T) {
	s := New(t.TempDir())
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{"99": true, "5": true, SchemaVersion: false, "3": false, "옛판": false}
	for schema, want := range cases {
		body := `{"schema":"` + schema + `","files":{}}`
		if err := os.WriteFile(filepath.Join(s.Home(), "cache", "scanstate.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := s.LoadScanState().Newer(); got != want {
			t.Fatalf("판 %q : Newer = %v, %v 이어야 한다", schema, got, want)
		}
	}
}

// 동시 start 여럿이 같은 id 를 내지 않는다 — id 는 락 안에서 뽑는다.
func TestMarkStartConcurrentUniqueIDs(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now()
	const n = 16
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			_, err := s.AppendMarkStart(TimeMark{Start: now, Name: fmt.Sprintf("판%d", i)})
			errs <- err
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	marks, warns, err := s.LoadMarks()
	if err != nil || len(warns) != 0 || len(marks) != n {
		t.Fatalf("marks %d · 경고 %v · %v", len(marks), warns, err)
	}
}

// marks.lock 은 몇 초만 지나도 죽은 락으로 보고 가로챈다 (scan 락의 30분이 아니다).
func TestMarksLockStaleIsShort(t *testing.T) {
	s := New(t.TempDir())
	if err := os.MkdirAll(s.Home(), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(s.Home(), "marks.lock")
	old := time.Now().Add(-20 * time.Second)
	if err := os.WriteFile(lock, []byte(fmt.Sprintf("999999 %d", old.UnixMilli())), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(lock, old, old)
	start := time.Now()
	if _, err := s.AppendMarkStart(TimeMark{Start: time.Now(), Name: "락"}); err != nil {
		t.Fatalf("죽은 락을 못 가로챘다 : %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("죽은 락 앞에서 너무 오래 기다렸다 : %v", time.Since(start))
	}
}
