package paths

import (
	"strings"
	"testing"
)

// 실제 ~/.claude/projects 폴더 이름과 맞춘 값이다. 영숫자가 아닌 글자는 모두 `-` 가 된다.
func TestSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`C:\Mirusona\Project\ClaudeProject\ProjectOfficina`, "C--Mirusona-Project-ClaudeProject-ProjectOfficina"},
		{`C:\Users\vicso\.claude\skills\localharness-usage`, "C--Users-vicso--claude-skills-localharness-usage"},
		{`C:\일감\스크래치 패드`, "C" + strings.Repeat("-", 12)},
		{`C:\a b\c`, "C--a-b-c"},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
