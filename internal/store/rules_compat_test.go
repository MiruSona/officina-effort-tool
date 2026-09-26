package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
