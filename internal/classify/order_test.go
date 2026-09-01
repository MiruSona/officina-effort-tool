package classify

import (
	"strings"
	"testing"
)

// 낱말 차례는 map 에서 나오므로 가르는 규칙이 없으면 돌릴 때마다 분류가 달라진다.
func TestWordOrderIsStableAcrossParses(t *testing.T) {
	var first []string
	for i := 0; i < 20; i++ {
		r, err := ParseRules(strings.NewReader(DefaultRulesText))
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = r.WordOrder
			continue
		}
		if len(first) != len(r.WordOrder) {
			t.Fatalf("낱말 수가 달라졌다 : %d → %d", len(first), len(r.WordOrder))
		}
		for j := range first {
			if first[j] != r.WordOrder[j] {
				t.Fatalf("%d번째 낱말이 달라졌다 : %q → %q", j, first[j], r.WordOrder[j])
			}
		}
	}
}

// 긴 낱말이 먼저다 (「남은 작업」이 「조사」보다 먼저 걸려야 한다).
func TestWordOrderLongestFirst(t *testing.T) {
	r, err := ParseRules(strings.NewReader(DefaultRulesText))
	if err != nil {
		t.Fatal(err)
	}
	prev := 0
	for i, w := range r.WordOrder {
		n := len([]rune(w))
		if i > 0 && n > prev {
			t.Fatalf("%q 가 앞 낱말보다 길다", w)
		}
		prev = n
	}
}
