package classify

import (
	"strings"
	"testing"
)

// 종류별 기본 줄 덩어리가 기본글과 어긋나면 안 된다. 새 종류를 표에 안 넣는 실수도 여기서 잡힌다.
func TestKindDefaultsMatchDefaultRulesText(t *testing.T) {
	for _, kind := range KindOrder {
		block := KindDefault(kind)
		if block == "" {
			t.Fatalf("%s 의 기본 줄 덩어리가 없다", kind)
		}
		if !strings.Contains(DefaultRulesText, block) {
			t.Fatalf("%s 덩어리가 기본글에 그대로 안 들어 있다", kind)
		}
	}
	// 덩어리만 모아 읽어도 빠진 종류가 없어야 한다.
	all := ""
	for _, kind := range KindOrder {
		all += KindDefault(kind)
	}
	r, err := ParseRules(strings.NewReader(all))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.MissingKinds(); len(got) != 0 {
		t.Fatalf("표에 없는 종류가 있다 : %v", got)
	}
}

func TestDefaultRulesTextHasEveryKind(t *testing.T) {
	r, err := ParseRules(strings.NewReader(DefaultRulesText))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.MissingKinds(); len(got) != 0 {
		t.Fatalf("기본글에 빠진 종류가 있다 : %v", got)
	}
}

// 종류를 하나씩 지운 글에서 그 종류만 빠졌다고 알려야 한다.
func TestMissingKindsFindsEachKind(t *testing.T) {
	strip := map[string][]string{
		KindTool:  {"tool"},
		KindRun:   {"runpre", "runin"},
		KindChore: {"chore"},
		KindCont:  {"contfirst", "contstop"},
	}
	for kind, prefixes := range strip {
		text := dropLines(DefaultRulesText, prefixes)
		r, err := ParseRules(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		got := r.MissingKinds()
		if len(got) != 1 || got[0] != kind {
			t.Fatalf("%s 를 지웠는데 빠진 종류 = %v", kind, got)
		}
	}
}

func dropLines(text string, prefixes []string) string {
	var keep []string
	for _, line := range strings.Split(text, "\n") {
		drop := false
		for _, p := range prefixes {
			if strings.HasPrefix(line, p+"\t") {
				drop = true
			}
		}
		if !drop {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}
