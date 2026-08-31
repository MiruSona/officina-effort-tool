package model

import "testing"

func TestUsageTotalExcludesThinking(t *testing.T) {
	u := Usage{Input: 10, Output: 100, CacheRead: 5, CacheCreate: 20, Thinking: 60}
	if got := u.Total(); got != 135 {
		t.Fatalf("Total = %d, 바란 값 135 (thinking 은 안 더한다)", got)
	}
	if got := u.Billable(); got != 130 {
		t.Fatalf("Billable = %d, 바란 값 130", got)
	}
}

func TestNormalizeModelStripsBracket(t *testing.T) {
	if got := Normalize("claude-opus-5[1m]"); got != "claude-opus-5" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestNormalizeModelStripsDate(t *testing.T) {
	if got := Normalize("claude-haiku-4-5-20251001"); got != "claude-haiku-4-5" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestNormalizeModelAlias(t *testing.T) {
	if got := Normalize("opus"); got != "claude-opus-5" {
		t.Fatalf("Normalize = %q", got)
	}
	if got := Normalize("Sonnet"); got != "claude-sonnet-5" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestModelUsageMerge(t *testing.T) {
	a := ModelUsage{"m": {Output: 5}}
	a.Merge(ModelUsage{"m": {Output: 7}, "n": {Output: 1}})
	if a["m"].Output != 12 || a["n"].Output != 1 {
		t.Fatalf("Merge 결과가 다르다 : %+v", a)
	}
	if a.Sum().Output != 13 {
		t.Fatalf("Sum = %d", a.Sum().Output)
	}
}
