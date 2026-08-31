package secret

import (
	"regexp"
	"strings"
)

type rule struct {
	name string
	re   *regexp.Regexp
}

// 비밀정보로 보이는 꼴. 걸리면 값을 안 적고 규칙 이름만 남긴다.
var rules = []rule{
	{"anthropic-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{8,}`)},
	{"openai-key", regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
	{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{16,}`)},
	{"aws-key", regexp.MustCompile(`AKIA[0-9A-Z]{12,}`)},
	{"slack-token", regexp.MustCompile(`xox[abprs]-[A-Za-z0-9\-]{10,}`)},
	{"bearer", regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]{20,}`)},
	{"private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
}

// Scan 은 비밀정보 꼴이 있으면 규칙 이름을 준다. 값은 절대 안 돌려준다.
func Scan(s string) (string, bool) {
	for _, r := range rules {
		if r.re.MatchString(s) {
			return r.name, true
		}
	}
	return "", false
}

// Mask 는 살균과 비밀검사를 한 번에 한다. 캐시로 가는 글은 다 여기를 지난다.
func Mask(s string) string {
	out := Sanitize(s)
	if name, hit := Scan(out); hit {
		return "[가려짐:" + name + "]"
	}
	return out
}

// MaskAll 은 구조체가 내놓은 글 칸을 통째로 지나가게 한다.
func MaskAll(fields []*string) {
	for _, p := range fields {
		if p == nil || strings.TrimSpace(*p) == "" {
			continue
		}
		*p = Mask(*p)
	}
}
