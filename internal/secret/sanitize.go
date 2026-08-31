package secret

import (
	"strings"
	"unicode"
)

const maxLen = 200

// Sanitize 는 남이 쓴 글을 화면에 찍어도 되게 다듬는다. 살균하는 자리는 이 하나뿐이다.
func Sanitize(s string) string {
	var b strings.Builder
	skipANSI := false
	for _, r := range s {
		if skipANSI {
			if r == 'm' || unicode.IsLetter(r) {
				skipANSI = false
			}
			continue
		}
		if r == 0x1b {
			skipANSI = true
			continue
		}
		b.WriteRune(replaceRune(r))
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	r := []rune(out)
	if len(r) > maxLen {
		return string(r[:maxLen])
	}
	return string(r)
}

func replaceRune(r rune) rune {
	switch r {
	case '\t', '\n', '\r':
		return ' '
	case '|':
		return '｜'
	case '`':
		return '｀'
	case '<':
		return '‹'
	case '>':
		return '›'
	}
	if r < 0x20 || r == 0x7f {
		return ' '
	}
	return r
}
