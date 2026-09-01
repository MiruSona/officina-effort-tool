package group

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mirusona/efforttool/internal/collect"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/store"
)

// 묶는 열쇠 이름.
const (
	ModeMark    = "mark"
	ModeGap     = "gap"
	ModeClass   = "class"
	ModeSession = "session"
	ModeNone    = "none"
)

// Key 는 묶는 열쇠다.
type Key struct {
	Mode string
	Gap  time.Duration // Mode 가 mark·gap 일 때 쓰는 간격 컷
}

// ParseKey 는 "mark" · "gap:30m" · "class" · "session" · "none" 을 읽는다.
// 모르는 값이면 쓸 수 있는 것을 다 적어 실패한다.
func ParseKey(s string, defGap time.Duration) (Key, error) {
	if s == "" {
		return Key{Mode: ModeMark, Gap: defGap}, nil
	}
	if v, ok := strings.CutPrefix(s, "gap:"); ok {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Key{}, fmt.Errorf("gap 간격이 잘못됐습니다 : %q (보기 : gap:30m)", s)
		}
		return Key{Mode: ModeGap, Gap: d}, nil
	}
	switch s {
	case ModeMark:
		return Key{Mode: ModeMark, Gap: defGap}, nil
	case ModeGap:
		return Key{Mode: ModeGap, Gap: defGap}, nil
	case ModeClass, ModeSession, ModeNone:
		return Key{Mode: s, Gap: defGap}, nil
	}
	return Key{}, fmt.Errorf("모르는 묶기 열쇠 %q (mark · gap:30m · class · session · none)", s)
}

// Group 은 소단계 하나다.
type Group struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Marked   bool        `json:"marked"` // 사람이 표시한 정본인가
	Class    model.Class `json:"class"`
	Sessions []string    `json:"sessions"`
	Tasks    []string    `json:"tasks"`
	Start    time.Time   `json:"start"`
	End      time.Time   `json:"end"`
	// WallMs 는 안에 든 작업 구간의 합집합이다. 작업 사이의 사람 대기는 안 든다.
	WallMs int64    `json:"wall_ms"`
	PureMs int64    `json:"pure_ms"`
	Tokens int64    `json:"tokens"`
	Warn   []string `json:"warn"`
}

// Build 는 작업을 묶는다. 정본이 먼저 이기고, 남은 작업만 열쇠로 묶는다.
func Build(tasks []model.Task, marks []store.Mark, key Key) []Group {
	sorted := make([]model.Task, len(tasks))
	copy(sorted, tasks)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	var out []Group
	taken := map[string]bool{}
	if key.Mode == ModeMark {
		out, taken = buildMarked(sorted, marks)
	}
	out = append(out, buildAuto(sorted, taken, key)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// buildMarked 는 정본에 적힌 묶음을 만든다. 어느 정본에도 없는 작업은 자동 묶기로 넘긴다.
func buildMarked(tasks []model.Task, marks []store.Mark) ([]Group, map[string]bool) {
	taken := map[string]bool{}
	var out []Group
	for _, m := range marks {
		g := Group{ID: m.ID, Name: m.Name, Marked: true, Class: m.Class}
		var members []model.Task
		for i := range tasks {
			if taken[tasks[i].PromptID] || !markHasTask(m, tasks[i].PromptID) {
				continue
			}
			taken[tasks[i].PromptID] = true
			members = append(members, tasks[i])
		}
		if len(members) == 0 {
			continue
		}
		fill(&g, members)
		out = append(out, g)
	}
	return out, taken
}

// markHasTask 는 정본의 작업 id 목록에 그 작업이 드는지다. 앞자리만 적어도 된다.
func markHasTask(m store.Mark, promptID string) bool {
	for _, id := range m.Tasks {
		if id == promptID || strings.HasPrefix(promptID, id) {
			return true
		}
	}
	return false
}

// buildAuto 는 정본에 안 든 작업을 열쇠로 묶는다.
func buildAuto(tasks []model.Task, taken map[string]bool, key Key) []Group {
	var out []Group
	var members []model.Task
	seq := map[string]int{}
	flush := func() {
		if len(members) == 0 {
			return
		}
		g := Group{}
		sid := members[0].SessionID
		seq[sid]++
		g.ID = fmt.Sprintf("자동-%s-%d", shortID(sid), seq[sid])
		g.Name = members[0].Title
		fill(&g, members)
		out = append(out, g)
		members = nil
	}
	var prev *model.Task
	for i := range tasks {
		t := tasks[i]
		if taken[t.PromptID] {
			// 정본에 든 작업은 자동 묶기 사슬을 끊지 않고 건너뛴다.
			continue
		}
		if prev != nil && !sameGroup(*prev, t, key) {
			flush()
		}
		members = append(members, t)
		prev = &tasks[i]
	}
	flush()
	return out
}

// sameGroup 은 앞 작업과 같은 묶음에 넣을지다.
func sameGroup(prev, cur model.Task, key Key) bool {
	if prev.SessionID != cur.SessionID {
		return false
	}
	switch key.Mode {
	case ModeSession:
		return true
	case ModeNone:
		return false
	case ModeClass:
		return prev.Class == cur.Class
	}
	if prev.Start.IsZero() || cur.Start.IsZero() {
		return false
	}
	return cur.Start.Sub(prev.Start) <= key.Gap
}

// fill 은 묶음의 시간·토큰·분류를 안에 든 작업에서 채운다.
func fill(g *Group, members []model.Task) {
	spans := make([]collect.Span, 0, len(members))
	seenSession := map[string]bool{}
	seenWarn := map[string]bool{}
	byClass := map[model.Class]int64{}
	for _, t := range members {
		g.Tasks = append(g.Tasks, t.PromptID)
		if !seenSession[t.SessionID] {
			seenSession[t.SessionID] = true
			g.Sessions = append(g.Sessions, t.SessionID)
		}
		for _, w := range t.Warn {
			if seenWarn[w] {
				continue
			}
			seenWarn[w] = true
			g.Warn = append(g.Warn, w)
		}
		spans = append(spans, collect.Span{Start: t.Start, End: t.End})
		// 작업 벽시계와 같은 뜻이 되게 서브에이전트 구간도 같이 합집합한다.
		for _, a := range t.Agents {
			if a.Start.IsZero() || a.End.IsZero() {
				continue
			}
			spans = append(spans, collect.Span{Start: a.Start, End: a.End})
		}
		if g.Start.IsZero() || (!t.Start.IsZero() && t.Start.Before(g.Start)) {
			g.Start = t.Start
		}
		if t.End.After(g.End) {
			g.End = t.End
		}
		g.PureMs += t.PureMs
		g.Tokens += t.Usage.Sum().Total()
		if t.Class.IsWork() {
			byClass[t.Class] += t.WallMs
		}
	}
	g.WallMs = collect.UnionMs(spans)
	if g.Class == "" {
		g.Class = topClass(byClass)
	}
	if g.Name == "" {
		g.Name = members[0].Title
	}
}

// topClass 는 벽시계 합이 가장 큰 일 칸이다. 일 칸이 없으면 미분류다.
func topClass(byClass map[model.Class]int64) model.Class {
	best := model.ClassUnknown
	var bestMs int64 = -1
	for _, c := range model.WorkClasses {
		ms, ok := byClass[c]
		if !ok {
			continue
		}
		if ms > bestMs {
			best, bestMs = c, ms
		}
	}
	return best
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
