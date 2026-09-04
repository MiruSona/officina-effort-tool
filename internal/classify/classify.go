package classify

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mirusona/officina-effort-tool/internal/model"
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
	ByMatch     = "짝짓기"   // 알림 본문 task-id 로 서브에이전트를 정확히 찾음
	ByContinue  = "앞말이어짐" // 사람 프롬프트가 앞 일을 이어감
	ByDefault   = "기본"
)

// 물려받은 값으로 매긴 이름들. 이것으로 매긴 분류는 다음 작업에 다시 물려주지 않는다.
func isInherited(by string) bool {
	return by == ByInherit || by == ByMatch || by == ByContinue
}

// 서브에이전트 완료 알림을 알아보는 값.
const (
	OriginNotify = "task-notification"
	OriginPeer   = "peer"
)

// 알림 제목의 두 꼴. 살균이 < 를 ‹ 로 접어서 캐시에는 ‹ 꼴로 남는다.
var notifyTitles = []string{"‹task-notification›", "<task-notification>"}

// Ctx 는 작업 하나 바깥의 사정이다. 같은 세션 안에서만 채운다.
type Ctx struct {
	// PrevWork 는 가장 가까운 앞 일 칸 작업의 분류다. 사이에 대화·도구실행이 껴도 넘어간다.
	PrevWork model.Class
	// ChainAlive 는 그 일 칸 작업부터 여기까지 대화 사슬이 안 끊겼는지다.
	// 작업 사이가 한 번이라도 cont max 를 넘으면 끊긴 것으로 본다.
	// 바로 앞 작업하고만 재면 대화를 징검다리 삼아 몇 시간 전 일 칸까지 물려받고,
	// 앞 일 칸에서 바로 재면 촘촘한 대화가 오간 뒤의 「좋아 그러면」을 놓친다.
	// 사슬로 보면 둘 다 막는다 (2026-09-04 실측 : 앞말이어짐 25 → 36 · 미분류 76 → 65).
	ChainAlive bool
	Agents     map[string]*model.Agent // 세션 전체 서브에이전트, 열쇠는 AgentID(= task-id)
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

// TaskWith 는 작업 하나를 분류한다. 설계의 열한 순위를 그대로 탄다.
func (r *Rules) TaskWith(t *model.Task, ctx Ctx) (model.Class, string) {
	if isNotification(t) {
		return r.byNotify(t, ctx)
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
	c, by := r.byTools(t, title)
	if by != ByDefault {
		return c, by
	}
	// 미분류로 떨어질 뻔한 것만 「앞말 이어짐」으로 건진다. 앞 순위는 하나도 안 건드린다.
	if cc, ok := r.contInherit(t, title, ctx); ok {
		return cc, ByContinue
	}
	return c, by
}

// byNotify 는 알림 작업을 분류한다 (순위 1~3).
func (r *Rules) byNotify(t *model.Task, ctx Ctx) (model.Class, string) {
	// tool-use-id 가 없는 알림은 Monitor 감시 신호다. 일이 아니다.
	if t.NotifyKind == model.NotifyMonitor {
		return model.ClassTool, ByOrigin
	}
	if a := ctx.Agents[t.NotifyTaskID]; a != nil && t.NotifyTaskID != "" {
		if c, _ := r.Agent(a); c != "" {
			return c, ByMatch
		}
	}
	if ctx.PrevWork == "" {
		return model.ClassUnknown, ByInherit
	}
	return ctx.PrevWork, ByInherit
}

// contInherit 는 사람이 앞말을 이어 말한 것인지 본다. 조건을 다 채워야 물려받는다.
func (r *Rules) contInherit(t *model.Task, title string, ctx Ctx) (model.Class, bool) {
	if ctx.PrevWork == "" || !ctx.PrevWork.IsWork() {
		return "", false
	}
	if t.PromptSource != model.SourceTyped {
		return "", false
	}
	if !r.hasContFirst(title) {
		return "", false
	}
	// 잡무 낱말이 있으면 이어짐을 막기만 한다. 일 칸으로 잘못 새는 것보다 미분류가 낫다.
	if r.hasContStop(title) || r.isChore(title) {
		return "", false
	}
	if !ctx.ChainAlive {
		return "", false
	}
	return ctx.PrevWork, true
}

// hasContFirst 는 제목이 이어짐 글머리 낱말로 시작하는지다.
// 한 글자 낱말(「아」「어」)은 뒤에 빈칸이나 부호가 와야 잡는다 —
// 그냥 앞머리로만 보면 「아직」「어디」까지 물어 이어짐이 너무 넓어진다.
// rules.txt 는 칸 앞뒤 빈칸을 떼므로 낱말에 빈칸을 붙여 적는 수는 못 쓴다.
func (r *Rules) hasContFirst(title string) bool {
	low := strings.ToLower(strings.TrimSpace(title))
	for _, w := range r.ContFirst {
		if w == "" {
			continue
		}
		lw := strings.ToLower(w)
		if !strings.HasPrefix(low, lw) {
			continue
		}
		if utf8.RuneCountInString(lw) > 1 {
			return true
		}
		next, _ := utf8.DecodeRuneInString(low[len(lw):])
		if unicode.IsLetter(next) || unicode.IsDigit(next) {
			continue
		}
		return true
	}
	return false
}

func (r *Rules) hasContStop(title string) bool {
	low := strings.ToLower(title)
	for _, w := range r.ContStop {
		if w != "" && strings.Contains(low, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// 곁도구는 일을 만들지 않는 도구다. 이것만 쓴 작업이 도구 셈 때문에 대화로 못 떨어지던 것을 막는다.
var sideTools = map[string]bool{
	"SendMessage":     true,
	"ToolSearch":      true,
	"AskUserQuestion": true,
	"Monitor":         true,
	"TaskStop":        true,
	"ListAgents":      true,
	"ScheduleWakeup":  true,
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
