package render

import "fmt"

// Minutes 는 밀리초를 사람이 읽는 시간 글로 바꾼다.
func Minutes(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	if ms < 60*1000 {
		return "1분 미만"
	}
	min := ms / 60000
	if min < 60 {
		return fmt.Sprintf("%d분", min)
	}
	h := min / 60
	m := min % 60
	if m == 0 {
		return fmt.Sprintf("%d시간", h)
	}
	return fmt.Sprintf("%d시간 %d분", h, m)
}

// Tokens 는 토큰 수를 12.3K · 4.5M 꼴로 줄인다.
func Tokens(n int64) string {
	if n <= 0 {
		return "—"
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000*1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

// MsPerKTok 은 토큰 1000개당 걸린 시간(ms)이다.
func MsPerKTok(ms, tok int64) string {
	if tok <= 0 || ms <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", float64(ms)/(float64(tok)/1000))
}
