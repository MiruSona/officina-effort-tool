package estimate

import "sort"

// Quantile 은 선형 보간으로 분위수를 구한다. 값은 밀리초다.
func Quantile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	pos := q * float64(len(s)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(s) {
		return s[len(s)-1]
	}
	frac := pos - float64(lo)
	return s[lo] + (s[hi]-s[lo])*frac
}
