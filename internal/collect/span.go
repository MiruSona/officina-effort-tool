package collect

import (
	"sort"
	"time"
)

// Span 은 시각 구간 하나다.
type Span struct {
	Start time.Time
	End   time.Time
}

// UnionMs 는 구간 여럿의 합집합 길이(ms)다. 갈래 셋을 나란히 돌려도 겹친 만큼은 한 번만 센다.
// 벽시계를 재는 자리는 작업(collect)과 묶음(group) 둘이라 셈법을 여기 하나로 둔다.
func UnionMs(sp []Span) int64 {
	ok := make([]Span, 0, len(sp))
	for _, s := range sp {
		if s.Start.IsZero() || s.End.IsZero() || !s.End.After(s.Start) {
			continue
		}
		ok = append(ok, s)
	}
	if len(ok) == 0 {
		return 0
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].Start.Before(ok[j].Start) })

	var total int64
	cur := ok[0]
	for _, s := range ok[1:] {
		if s.Start.After(cur.End) {
			total += cur.End.Sub(cur.Start).Milliseconds()
			cur = s
			continue
		}
		if s.End.After(cur.End) {
			cur.End = s.End
		}
	}
	total += cur.End.Sub(cur.Start).Milliseconds()
	return total
}
