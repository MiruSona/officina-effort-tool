package model

import (
	"strings"
)

// 별칭 표. meta.json 은 opus/sonnet 처럼 짧게만 적는다.
var modelAlias = map[string]string{
	"opus":   "claude-opus-5",
	"sonnet": "claude-sonnet-5",
	"haiku":  "claude-haiku-4-5",
	"fable":  "claude-fable-5",
}

// Normalize 는 모델 이름을 하나로 모은다. 정규화 자리는 이 함수 하나뿐이다.
func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = stripBracket(s)
	s = stripDateSuffix(s)
	s = strings.Trim(s, "-")
	if full, ok := modelAlias[s]; ok {
		return full
	}
	return s
}

// claude-opus-5[1m] → claude-opus-5
func stripBracket(s string) string {
	i := strings.IndexByte(s, '[')
	if i < 0 {
		return s
	}
	return s[:i]
}

// claude-haiku-4-5-20251001 → claude-haiku-4-5
func stripDateSuffix(s string) string {
	i := strings.LastIndexByte(s, '-')
	if i < 0 || len(s)-i-1 != 8 {
		return s
	}
	if !allDigits(s[i+1:]) {
		return s
	}
	return s[:i]
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
