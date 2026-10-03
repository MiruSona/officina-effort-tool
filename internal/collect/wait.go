package collect

import (
	"time"

	"github.com/mirusona/officina-effort-tool/internal/jsonl"
)

// DefaultWaitMin 은 「대기」로 보는 도구 틈의 문턱이다. rules.txt `wait min` 이 없을 때 쓴다.
// 설계 : Docs/Design/2026-09-23-mark·live·대기·규칙호환설계.md 3절.
const DefaultWaitMin = 3 * time.Minute

// waitTracker 는 tool_use → tool_result 짝의 틈 중 문턱을 넘는 것을 모은다. 도구 이름으로 가르지 않는다.
// 나란히 부른 도구의 틈은 겹치므로 합집합으로 센다 — 대기가 벽시계를 넘지 않게.
type waitTracker struct {
	min   time.Duration
	open  map[string]time.Time
	spans []Span
}

func newWaitTracker(min time.Duration) *waitTracker {
	if min <= 0 {
		min = DefaultWaitMin
	}
	return &waitTracker{min: min, open: map[string]time.Time{}}
}

// see 는 본줄 하나를 본다. 짝 없는 tool_use(끊긴 판)·tool_result 는 끝내 안 센다.
func (w *waitTracker) see(line *jsonl.Line) {
	if line.Timestamp.IsZero() || !isMainLine(line) {
		return
	}
	uses, results := line.ToolIDs()
	for _, id := range uses {
		if _, ok := w.open[id]; !ok {
			w.open[id] = line.Timestamp
		}
	}
	for _, id := range results {
		start, ok := w.open[id]
		if !ok {
			continue
		}
		delete(w.open, id)
		if line.Timestamp.Sub(start) > w.min {
			w.spans = append(w.spans, Span{Start: start, End: line.Timestamp})
		}
	}
}

// Ms 는 모은 틈의 합집합 길이다.
func (w *waitTracker) Ms() int64 { return UnionMs(w.spans) }
