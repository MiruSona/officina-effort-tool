package render

import "strings"

type rng struct{ lo, hi rune }

// 유니코드 EastAsianWidth 의 W·F 구간 (폭 2). 라이브러리를 안 들이려고 직접 적는다.
var wideRanges = []rng{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF},
	{0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF},
	{0xA960, 0xA97F}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF},
	{0xFE10, 0xFE19}, {0xFE30, 0xFE6F}, {0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6}, {0x1F300, 0x1F64F}, {0x1F900, 0x1F9FF},
	{0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

// 결합문자·제어문자 구간 (폭 0).
var zeroRanges = []rng{
	{0x0000, 0x001F}, {0x007F, 0x009F}, {0x0300, 0x036F},
	{0x200B, 0x200F}, {0xFE00, 0xFE0F}, {0xFEFF, 0xFEFF},
}

func inRanges(r rune, list []rng) bool {
	for _, x := range list {
		if r >= x.lo && r <= x.hi {
			return true
		}
	}
	return false
}

// RuneWidth 는 글자 하나가 터미널에서 차지하는 칸 수다.
func RuneWidth(r rune) int {
	if inRanges(r, zeroRanges) {
		return 0
	}
	if inRanges(r, wideRanges) {
		return 2
	}
	return 1
}

func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

func PadRight(s string, w int) string {
	gap := w - StringWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}
