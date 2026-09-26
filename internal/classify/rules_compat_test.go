package classify

import (
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// 설계 1절 R1 : 이 exe 가 모르는 종류는 경고하고 그 줄만 건너뛴다.
func TestParseRulesSkipsUnknownKind(t *testing.T) {
	text := "word\t조사\t조사\nzzz\tmax\t3\nsize\tM\t1.0\n"
	r, warns, err := ParseRulesOpt(strings.NewReader(text), false)
	if err != nil {
		t.Fatalf("모르는 종류에서 멈췄다 : %v", err)
	}
	if r.Words["조사"] != model.ClassResearch || r.Size("M") != 1.0 {
		t.Fatalf("다른 줄을 못 읽었다 : %+v", r)
	}
	if len(warns) != 1 {
		t.Fatalf("경고 수 = %d, 1 이어야 한다 : %+v", len(warns), warns)
	}
	msg := warns[0].String()
	if !strings.Contains(msg, "2번째 줄") || !strings.Contains(msg, `"zzz"`) {
		t.Fatalf("줄 번호·종류 이름이 없다 : %s", msg)
	}
}

// R2 : 아는 종류 안의 모르는 분류도 경고 + 건너뜀. 다른 seed 는 그대로다.
func TestParseRulesSkipsUnknownClass(t *testing.T) {
	text := "seed\t조사\t2\t1\t4\nseed\t대기\t1\t1\t2\nseed\t구현\t3\t1\t5\n"
	r, warns, err := ParseRulesOpt(strings.NewReader(text), false)
	if err != nil {
		t.Fatalf("모르는 분류에서 멈췄다 : %v", err)
	}
	if len(warns) != 1 || warns[0].What != "분류" || warns[0].Name != "대기" || warns[0].Line != 2 {
		t.Fatalf("경고가 틀렸다 : %+v", warns)
	}
	if r.Seeds[model.ClassResearch].P50 != 2 || r.Seeds[model.ClassBuild].P80 != 5 {
		t.Fatalf("다른 seed 가 흔들렸다 : %+v", r.Seeds)
	}
}

// R4 : 엄격하게 읽으면 같은 파일이 오류다.
func TestParseRulesStrictFails(t *testing.T) {
	for _, text := range []string{
		"word\t조사\t조사\nzzz\tmax\t3\n",
		"seed\t대기\t1\t1\t2\n",
		"tool\t이상한\tWrite\n",
	} {
		if _, _, err := ParseRulesOpt(strings.NewReader(text), true); err == nil {
			t.Fatalf("strict 인데 안 막았다 : %q", text)
		}
		if _, err := ParseRules(strings.NewReader(text)); err == nil {
			t.Fatalf("ParseRules 는 strict 여야 한다 : %q", text)
		}
	}
}

// R3 : 아는 종류를 잘못 쓴 줄은 strict 가 아니어도 실패한다.
func TestParseRulesBadKnownLineStillFails(t *testing.T) {
	for _, text := range []string{"sub\tmax\tx\n", "seed\t조사\t1\n", "size\tM\t아무거나\n"} {
		if _, _, err := ParseRulesOpt(strings.NewReader(text), false); err == nil {
			t.Fatalf("사람 실수를 조용히 넘겼다 : %q", text)
		}
	}
}

// R7 : 자동 추가 주석의 판 글을 읽어 누가 더한 줄인지 경고에 담는다.
func TestWarningNamesAddingExe(t *testing.T) {
	text := "word\t조사\t조사\n\n" + AutoHeader("2026-09-30", []string{"wait"}, "abc1234") + "\nwait\tmin\t3\nwait\tmin\t4\n"
	_, warns, err := ParseRulesOpt(strings.NewReader(text), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 2 || warns[0].AddedBy != "abc1234" {
		t.Fatalf("더한 exe 판을 못 읽었다 : %+v", warns)
	}
	if !strings.Contains(warns[0].String(), "effort abc1234 가 더한 줄") {
		t.Fatalf("경고에 판 글이 없다 : %s", warns[0].String())
	}
	// 종류별로 묶으면 한 줄이다 (scan 한 판에 한 번).
	g := GroupWarnings(warns)
	if len(g) != 1 || !strings.Contains(g[0], "외 1줄") {
		t.Fatalf("묶기가 틀렸다 : %v", g)
	}
	if one := WarningsOneLine(warns); !strings.Contains(one, "2개") || strings.Contains(one, "\n") {
		t.Fatalf("한 줄 요약이 틀렸다 : %q", one)
	}
}

// 판 글은 자동 추가 덩어리 안에서만 쓴다. 빈 줄·다른 주석 뒤의 사람 줄에는 안 붙는다.
func TestWarningAddedByEndsAtBlock(t *testing.T) {
	text := AutoHeader("2026-09-30", []string{"wait"}, "abc1234") + "\nwait\tmin\t3\n\nzzz\tmax\t1\n" +
		AutoHeader("2026-10-01", []string{"yyy"}, "def5678") + "\n# 사람 주석\nyyy\tmax\t1\n"
	_, warns, err := ParseRulesOpt(strings.NewReader(text), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 3 || warns[0].AddedBy != "abc1234" || warns[1].AddedBy != "" || warns[2].AddedBy != "" {
		t.Fatalf("판 글이 덩어리 밖까지 이어졌다 : %+v", warns)
	}
}
