package estimate

import (
	"errors"
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

func TestParseArgs(t *testing.T) {
	items, err := ParseArgs([]string{"조사:M", "구현", "시험:구현:L"})
	if err != nil {
		t.Fatalf("ParseArgs : %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("항목 수 = %d", len(items))
	}
	if items[0].Class != model.ClassResearch || items[0].Size != "M" {
		t.Fatalf("첫 항목 = %+v", items[0])
	}
	if items[1].Class != model.ClassBuild || items[1].Size != "M" {
		t.Fatalf("둘째 항목 = %+v", items[1])
	}
	if items[2].Name != "시험" || items[2].Class != model.ClassBuild || items[2].Size != "L" {
		t.Fatalf("셋째 항목 = %+v", items[2])
	}
}

func TestParseArgsUnknownClassFails(t *testing.T) {
	if _, err := ParseArgs([]string{"뭐시기:M"}); err == nil {
		t.Fatal("모르는 분류인데 안 막았다")
	}
}

func TestEstimateParsePipeTable(t *testing.T) {
	src := `| 소단계 | 분류 | 크기 | 사람눈금 |
| --- | --- | --- | --- |
| 1. JSONL 파서 | 구현 | M | 60 |
| 2. 분류기 | 구현 | S | 30 |
| 3. 실측·검산 | 실측 | M | |
`
	items, err := ParseTable(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseTable : %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("항목 수 = %d", len(items))
	}
	if items[0].Name != "1. JSONL 파서" || items[0].Human != 60 {
		t.Fatalf("첫 줄 = %+v", items[0])
	}
	if items[2].Class != model.ClassMeasure || items[2].Human != 0 {
		t.Fatalf("셋째 줄 = %+v", items[2])
	}
}

// 사람눈금 0 은 빈 칸과 구별이 안 된다. 적었는데 안내 푸터가 뜨는 일을 막는다.
func TestParseTableRejectsNonPositiveHuman(t *testing.T) {
	for _, v := range []string{"0", "-5"} {
		src := "| 소단계 | 분류 | 크기 | 사람눈금 |\n| a | 구현 | M | " + v + " |\n"
		if _, err := ParseTable(strings.NewReader(src)); err == nil {
			t.Fatalf("사람눈금 %s 인데 안 막았다", v)
		}
	}
}

func TestParseTableSkipsHeadAndSeparator(t *testing.T) {
	src := "| 소단계 | 분류 |\n|---|---|\n| a | 구현 |\n"
	items, err := ParseTable(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("항목 수 = %d", len(items))
	}
}

func defaultRules(t *testing.T) *classify.Rules {
	t.Helper()
	r, err := classify.ParseRules(strings.NewReader(classify.DefaultRulesText))
	if err != nil {
		t.Fatalf("기본 규칙 : %v", err)
	}
	return r
}

// 앞 칸이 분류가 아니면 앞 칸을 찍어야 한다. 뒤 칸(크기)을 「모르는 분류」로 찍으면 헷갈린다.
// 두 칸 다 분류가 아니면 뒤 칸이 크기일 때만 앞 칸을 찍는다.
// `리뷰스킬:검증` 은 「이름:분류」 뜻이라 뒤 칸을 찍어야 한다 (옛 동작).
func TestParseArgBlamesByWhetherSecondIsSize(t *testing.T) {
	r := defaultRules(t)
	cases := map[string]string{"검증:M": "검증", "리뷰스킬:검증": "검증", "리뷰스킬:뭐": "뭐", "문서읽기:s": "문서읽기"}
	for arg, bad := range cases {
		_, err := ParseArgs([]string{arg})
		var ae *ArgError
		if !errors.As(err, &ae) {
			t.Fatalf("%s : ArgError 가 아니다 : %v", arg, err)
		}
		if msg := ExplainArgError(err, r).Error(); !strings.HasPrefix(msg, "모르는 분류 \""+bad+"\"") {
			t.Errorf("%s : %s", arg, msg)
		}
	}
}

// word 표가 비면 목록을 전체 일 칸으로 대신한다. 빈 목록 안내는 쓸모가 없다.
func TestExplainArgErrorFallsBackToAllWorkClasses(t *testing.T) {
	r, err := classify.ParseRules(strings.NewReader("size\tM\t1.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, perr := ParseArgs([]string{"검증:M"})
	msg := ExplainArgError(perr, r).Error()
	if !strings.Contains(msg, "조사 · 설계 · 구현 · 실측 · 문서 · 검토.") {
		t.Fatalf("전체 목록이 아니다 : %s", msg)
	}
}

func TestExplainArgErrorListsClassesFromRules(t *testing.T) {
	r := defaultRules(t)
	cases := []struct {
		arg, bad  string
		sizeRight bool
	}{
		{"검증:M", "검증", true},
		{"문서읽기:S", "문서읽기", true},
		{"손님 그림 v3 시안", "손님 그림 v3 시안", false},
	}
	for _, c := range cases {
		_, err := ParseArgs([]string{c.arg})
		msg := ExplainArgError(err, r).Error()
		if !strings.Contains(msg, "모르는 분류 \""+c.bad+"\"") {
			t.Errorf("%s : 찍은 칸이 다르다 : %s", c.arg, msg)
		}
		if got := strings.Contains(msg, "분류 자리가 틀렸"); got != c.sizeRight {
			t.Errorf("%s : 크기 안내 = %v : %s", c.arg, got, msg)
		}
		for _, cl := range []string{"조사", "설계", "구현", "실측", "문서", "검토"} {
			if !strings.Contains(msg, cl) {
				t.Errorf("%s : 분류 %s 가 목록에 없다 : %s", c.arg, cl, msg)
			}
		}
		if !strings.Contains(msg, "조사:M") || !strings.Contains(msg, "이름:조사:M") {
			t.Errorf("%s : 보기가 없다 : %s", c.arg, msg)
		}
	}
}

// 목록은 규칙에서 읽는다. word 표에 없는 분류는 목록에 안 나온다.
func TestExplainArgErrorUsesRulesNotHardcoded(t *testing.T) {
	r, err := classify.ParseRules(strings.NewReader("word\t조사\t조사\nword\t구현\t구현\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, perr := ParseArgs([]string{"검증:M"})
	msg := ExplainArgError(perr, r).Error()
	if !strings.Contains(msg, "조사 · 구현") || strings.Contains(msg, "설계") {
		t.Fatalf("규칙 밖 분류가 섞였다 : %s", msg)
	}
}

// 크기가 아닌 뒷칸이면 크기 안내를 하지 않는다.
func TestExplainArgErrorNoSizeHintForNonSize(t *testing.T) {
	r := defaultRules(t)
	_, err := ParseArgs([]string{"검증:뭐"})
	msg := ExplainArgError(err, r).Error()
	if strings.Contains(msg, "분류 자리가 틀렸") || !strings.HasPrefix(msg, "모르는 분류 \"뭐\"") {
		t.Fatalf("크기가 아닌데 맞다고 했다 : %s", msg)
	}
}
