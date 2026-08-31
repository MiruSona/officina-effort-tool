package secret

import (
	"strings"
	"testing"
)

func TestSanitizeStripsControlAndTags(t *testing.T) {
	in := "앞\x1b[31m빨강\x1b[0m\t줄\n바꿈 `코드` |표| <tag>"
	out := Sanitize(in)
	if strings.ContainsAny(out, "\x1b\t\n") {
		t.Fatalf("제어문자가 남았다 : %q", out)
	}
	if strings.Contains(out, "|") || strings.Contains(out, "`") || strings.Contains(out, "<") {
		t.Fatalf("표를 깨는 글자가 남았다 : %q", out)
	}
	if !strings.Contains(out, "빨강") {
		t.Fatalf("본문이 사라졌다 : %q", out)
	}
}

func TestSanitizeCutsLongText(t *testing.T) {
	out := Sanitize(strings.Repeat("가", 500))
	if len([]rune(out)) != 200 {
		t.Fatalf("길이 = %d", len([]rune(out)))
	}
}

func TestSecretMaskedInTitle(t *testing.T) {
	value := "ghp_" + strings.Repeat("A", 30)
	out := Mask("토큰은 " + value + " 이다")
	if strings.Contains(out, value) {
		t.Fatal("비밀 값이 출력에 남았다")
	}
	if out != "[가려짐:github-token]" {
		t.Fatalf("Mask = %q", out)
	}
}

func TestSecretScanFindsEmail(t *testing.T) {
	if name, hit := Scan("연락은 a.b@example.com 으로"); !hit || name != "email" {
		t.Fatalf("Scan = %q %v", name, hit)
	}
}

func TestMaskAllPassesEveryField(t *testing.T) {
	a := "sk-ant-" + strings.Repeat("x", 20)
	b := "보통 글"
	MaskAll([]*string{&a, &b})
	if !strings.HasPrefix(a, "[가려짐:") {
		t.Fatalf("a = %q", a)
	}
	if b != "보통 글" {
		t.Fatalf("b = %q", b)
	}
}
