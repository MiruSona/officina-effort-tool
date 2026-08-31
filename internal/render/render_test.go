package render

import (
	"strings"
	"testing"
)

func TestRuneWidthKorean(t *testing.T) {
	if RuneWidth('가') != 2 {
		t.Fatal("한글은 2칸이다")
	}
	if RuneWidth('a') != 1 {
		t.Fatal("영문은 1칸이다")
	}
	if RuneWidth('́') != 0 {
		t.Fatal("결합문자는 0칸이다")
	}
	if StringWidth("가a") != 3 {
		t.Fatalf("StringWidth = %d", StringWidth("가a"))
	}
}

func TestTablePipeEscapesPipe(t *testing.T) {
	out := Table([]string{"이름"}, [][]string{{"a|b"}}, Pipe)
	if !strings.Contains(out, `a\|b`) {
		t.Fatalf("파이프를 안 막았다 : %q", out)
	}
	row := strings.Split(out, "\n")[2]
	if strings.Count(strings.ReplaceAll(row, `\|`, ""), "|") != 2 {
		t.Fatalf("표가 깨졌다 : %q", out)
	}
}

func TestMinutesFormat(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, "—"},
		{30 * 1000, "1분 미만"},
		{5000 * 1000, "1시간 23분"},
		{47 * 60 * 1000, "47분"},
		{2 * 60 * 60 * 1000, "2시간"},
	}
	for _, c := range cases {
		if got := Minutes(c.ms); got != c.want {
			t.Fatalf("Minutes(%d) = %q, 바란 값 %q", c.ms, got, c.want)
		}
	}
}

func TestTokensFormat(t *testing.T) {
	if Tokens(12300) != "12.3K" {
		t.Fatalf("Tokens = %q", Tokens(12300))
	}
	if Tokens(4500000) != "4.5M" {
		t.Fatalf("Tokens = %q", Tokens(4500000))
	}
}

func TestWideTableAlignsKorean(t *testing.T) {
	out := Table([]string{"가나", "b"}, [][]string{{"a", "c"}}, Wide)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("줄 수 = %d", len(lines))
	}
	if StringWidth(lines[0]) != StringWidth(lines[1]) {
		t.Fatalf("폭이 안 맞는다 : %q / %q", lines[0], lines[1])
	}
}
