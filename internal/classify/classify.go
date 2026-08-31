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
	ByOrigin    = "원인"   // 도구실행·대화
	ByInherit   = "이어받기" // 알림이 앞 작업에서 물려받음
	ByWeb       = "웹조사"
	ByChore     = "잡무"
	ByDefault   = "기본"
)

// 서브에이전트 완료 알림을 알아보는 값.
const (
	OriginNotify = "task-notification"
	OriginPeer   = "peer"
)

// 알림 제목의 두 꼴. 살균이 < 를 ‹ 로 접어서 캐시에는 ‹ 꼴로 남는다.
var notifyTitles = []string{"‹task-notification›", "<task-notification>"}

// Ctx 는 작업 하나 바깥의 사정이다. 지금은 같은 세션 직전 일 칸 분류 하나뿐이다.
type Ctx struct {
	Prev model.Class
}

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

// Task 는 작업 하나를 바깥 사정 없이 분류한다.
func (r *Rules) Task(t *model.Task) (model.Class, string) {
	return r.TaskWith(t, Ctx{})
}

// TaskWith 는 작업 하나를 분류한다. 설계 표 2 의 아홉 순위를 그대로 탄다.
func (r *Rules) TaskWith(t *model.Task, ctx Ctx) (model.Class, string) {
	if isNotification(t) {
		if ctx.Prev == "" {
			return model.ClassUnknown, ByInherit
		}
		return ctx.Prev, ByInherit
	}
	title := strings.TrimSpace(t.Title)
	if r.isToolRunTitle(title) {
		return model.ClassTool, ByOrigin
	}
	if c, by, ok := r.byAgents(t); ok {
		return c, by
	}
	if c, ok := r.matchSuffix(title); ok {
		return c, ByTitle
	}
	if c, ok := r.matchContains(title); ok {
		return c, ByTitle
	}
	return r.byTools(t, title)
}

// 곁도구는 일을 만들지 않는 도구다. 이것만 쓴 작업이 도구 셈 때문에 대화로 못 떨어지던 것을 막는다.
var sideTools = map[string]bool{
	"SendMessage":     true,
	"ToolSearch":      true,
	"AskUserQuestion": true,
	"Monitor":         true,
	"TaskStop":        true,
	"ListAgents":      true,
}

// byTools 는 도구 신호로 보는 순위 5~9 다. 곁도구를 뺀 셈으로 본다.
func (r *Rules) byTools(t *model.Task, title string) (model.Class, string) {
	work := *t
	work.Tools = withoutSideTools(t.Tools)

	write := r.Set(SetWrite)
	if r.isChore(title) && work.ToolsIn(write) == 0 {
		return model.ClassChore, ByChore
	}
	web, read, shell := r.Set(SetWeb), r.Set(SetRead), r.Set(SetShell)
	// 셸은 웹조사 중에도 곁다리로 한두 번 쓰므로 「웹·읽기 밖」 셈에서 뺀다.
	if work.ToolCalls() > 0 && work.ToolsIn(web) > 0 && work.ToolsOut(web, read, shell) == 0 {
		return model.ClassResearch, ByWeb
	}
	shellCalls := work.ToolsIn(shell)
	if len(t.Agents) == 0 && shellCalls > 0 && shellCalls <= r.ShellMax && work.ToolsOut(shell) == 0 {
		return model.ClassTool, ByOrigin
	}
	if work.ToolCalls() == 0 && len(t.Agents) == 0 {
		return model.ClassChat, ByOrigin
	}
	return model.ClassUnknown, ByDefault
}

func withoutSideTools(tools map[string]int) map[string]int {
	out := make(map[string]int, len(tools))
	for name, v := range tools {
		if sideTools[name] {
			continue
		}
		out[name] = v
	}
	return out
}

// byAgents 는 서브에이전트 분류 중 토큰을 가장 많이 쓴 것을 고른다.
func (r *Rules) byAgents(t *model.Task) (model.Class, string, bool) {
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
	return best, bestBy, best != ""
}

// isNotification 은 서브에이전트 완료 알림·다른 세션 메시지인지 본다.
func isNotification(t *model.Task) bool {
	if t.Origin == OriginNotify || t.Origin == OriginPeer {
		return true
	}
	title := strings.TrimSpace(t.Title)
	for _, p := range notifyTitles {
		if strings.HasPrefix(title, p) {
			return true
		}
	}
	return false
}

func (r *Rules) isToolRunTitle(title string) bool {
	for _, p := range r.Prefixes {
		if p != "" && strings.HasPrefix(title, p) {
			return true
		}
	}
	for _, w := range r.Contains {
		if w != "" && strings.Contains(title, w) {
			return true
		}
	}
	return false
}

func (r *Rules) isChore(title string) bool {
	low := strings.ToLower(title)
	for _, w := range r.ChoreWord {
		if w != "" && strings.Contains(low, strings.ToLower(w)) {
			return true
		}
	}
	return false
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
