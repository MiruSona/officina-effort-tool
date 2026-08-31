package estimate

import (
	"strings"
	"testing"

	"github.com/mirusona/efforttool/internal/model"
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
