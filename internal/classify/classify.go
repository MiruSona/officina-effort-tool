package classify

import (
	"strings"

	"github.com/mirusona/efforttool/internal/model"
)

// 어떤 규칙으로 분류했는지 남기는 이름.
const (
	BySuffix    = "끝말"
	ByWord      = "낱말"
	ByAgentType = "에이전트"
	ByTitle     = "제목"
	ByDefault   = "기본"
)

// Agent 는 서브에이전트 하나의 분류를 정한다 (순위 1~3).
func (r *Rules) Agent(a *model.Agent) (model.Class, string) {
	desc := strings.TrimSpace(a.Description)
	if c, ok := r.matchSuffix(desc); ok {
		return c, BySuffix
	}
	if c, ok := r.matchContains(desc); ok {
		return c, ByWord
	}
	if c, ok := r.Agents[a.AgentType]; ok {
		return c, ByAgentType
	}
	return "", ""
}

// Task 는 작업 하나의 분류를 정한다. 토큰을 가장 많이 쓴 서브에이전트가 이긴다.
func (r *Rules) Task(t *model.Task) (model.Class, string) {
	best := model.Class("")
	bestBy := ""
	var bestTok int64
	for i := range t.Agents {
		c, by := r.Agent(&t.Agents[i])
		t.Agents[i].Class = c
		if c == "" {
			t.Agents[i].Class = model.ClassUnknown
			continue
		}
		tok := t.Agents[i].Usage.Sum().Total()
		if tok > bestTok || best == "" {
			best, bestBy, bestTok = c, by, tok
		}
	}
	if best != "" {
		return best, bestBy
	}
	title := strings.TrimSpace(t.Title)
	if c, ok := r.matchSuffix(title); ok {
		return c, ByTitle
	}
	if c, ok := r.matchContains(title); ok {
		return c, ByTitle
	}
	return model.ClassUnknown, ByDefault
}

func (r *Rules) matchSuffix(s string) (model.Class, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	for _, w := range r.WordOrder {
		if strings.HasSuffix(s, strings.ToLower(w)) {
			return r.Words[w], true
		}
	}
	return "", false
}

func (r *Rules) matchContains(s string) (model.Class, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	for _, w := range r.WordOrder {
		if strings.Contains(s, strings.ToLower(w)) {
			return r.Words[w], true
		}
	}
	return "", false
}
