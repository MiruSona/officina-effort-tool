package model

import "time"

// Class 는 작업 분류다.
type Class string

const (
	ClassResearch Class = "조사"
	ClassDesign   Class = "설계"
	ClassBuild    Class = "구현"
	ClassMeasure  Class = "실측"
	ClassDoc      Class = "문서"
	ClassReview   Class = "검토"
	ClassUnknown  Class = "미분류"

	// 일이 아닌 칸. 예상 공수 표본에서 뺀다.
	ClassChat  Class = "대화"   // 사람과 주고받기만 한 것
	ClassTool  Class = "도구실행" // ! 직접 명령 · 슬래시 명령 · 한 줄 셸 대행
	ClassChore Class = "잡무"   // 커밋·푸시 같은 형상관리
)

// WorkClasses 는 「일」로 세는 칸이다. 예상 공수의 표본은 여기서만 뽑는다.
var WorkClasses = []Class{
	ClassResearch, ClassDesign, ClassBuild,
	ClassMeasure, ClassDoc, ClassReview,
}

// AllClasses 는 표를 찍는 차례다 (일 → 미분류 → 일 아닌 것).
var AllClasses = []Class{
	ClassResearch, ClassDesign, ClassBuild,
	ClassMeasure, ClassDoc, ClassReview, ClassUnknown,
	ClassChat, ClassTool, ClassChore,
}

// IsWork 는 예상 공수 표본에 넣어도 되는 칸인지다.
// 미분류는 「모르는 것」이지 일의 종류가 아니라 false 다.
func (c Class) IsWork() bool {
	for _, w := range WorkClasses {
		if c == w {
			return true
		}
	}
	return false
}

func IsClass(s string) bool {
	for _, c := range AllClasses {
		if string(c) == s {
			return true
		}
	}
	return false
}

// 프롬프트가 어디서 왔는지. 아는 값만 두고 나머지는 「기타」로 접는다.
const (
	SourceTyped      = "typed"
	SourceSystem     = "system"
	SourceQueued     = "queued"
	SourceSDK        = "sdk"
	SourceSuggestion = "suggestion_accepted"
	SourceOther      = "기타"
)

var knownSources = []string{SourceTyped, SourceSystem, SourceQueued, SourceSDK, SourceSuggestion}

// 알림의 종류. 감시 신호는 일이 아니라 따로 가른다.
const (
	NotifyAgent   = "agent"
	NotifyMonitor = "monitor"
)

// NormalizeSource 는 아는 promptSource 만 그대로 두고 나머지를 「기타」로 접는다.
// 캐시에 남는 글을 아는 낱말로만 만들어, 남이 쓴 값이 그대로 저장되는 것을 막는다.
func NormalizeSource(s string) string {
	if s == "" {
		return ""
	}
	for _, k := range knownSources {
		if s == k {
			return k
		}
	}
	return SourceOther
}

// Agent 는 서브에이전트 한 건이다.
type Agent struct {
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	Description   string `json:"desc"`
	Model         string `json:"model"`
	SpawnDepth    int    `json:"depth"`
	ParentAgentID string `json:"parent_agent_id,omitempty"`

	ToolUseID   string     `json:"tool_use_id"`
	Start       time.Time  `json:"start"`
	End         time.Time  `json:"end"`
	Usage       ModelUsage `json:"usage"`
	Turns       int        `json:"turns"`
	Class       Class      `json:"class"`
	MetaMissing bool       `json:"meta_missing"`
}

// Task 는 사용자 요청 한 건(promptId 하나)이다.
type Task struct {
	PromptID  string    `json:"prompt_id"`
	SessionID string    `json:"session_id"`
	Project   string    `json:"project"`
	Title     string    `json:"title"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	// WallMs 는 본줄 구간과 서브에이전트 구간의 합집합 길이다. 갈래를 나란히 돌려도 겹친 만큼은 한 번만 센다.
	WallMs      int64 `json:"wall_ms"`
	MainWallMs  int64 `json:"main_wall_ms"`
	AgentWallMs int64 `json:"agent_wall_ms"`
	PureMs      int64 `json:"pure_ms"`
	// 프롬프트 출처와 알림 열쇠. 알림 본문 자체는 절대 저장하지 않는다.
	PromptSource string `json:"prompt_source,omitempty"`
	NotifyTaskID string `json:"notify_task_id,omitempty"`
	NotifyKind   string `json:"notify_kind,omitempty"`

	Usage     ModelUsage `json:"usage"`
	MainUsage ModelUsage `json:"main_usage"`
	Agents    []Agent    `json:"agents"`
	Turns     int        `json:"turns"`
	Class     Class      `json:"class"`
	ClassBy   string     `json:"class_by"`
	Version   string     `json:"version"`
	Origin    string     `json:"origin"`
	Warn      []string   `json:"warn"`
	// 메인 세션이 부른 도구 이름 → 횟수. 서브에이전트 것은 안 센다.
	Tools map[string]int `json:"tools,omitempty"`
}

// ToolCalls 는 도구 호출 수 합이다.
func (t *Task) ToolCalls() int {
	n := 0
	for _, v := range t.Tools {
		n += v
	}
	return n
}

// ToolsIn 은 그 집합에 든 도구의 호출 수다.
func (t *Task) ToolsIn(set map[string]bool) int {
	n := 0
	for name, v := range t.Tools {
		if set[name] {
			n += v
		}
	}
	return n
}

// ToolsOut 은 어느 집합에도 안 든 도구의 호출 수다.
func (t *Task) ToolsOut(sets ...map[string]bool) int {
	n := 0
	for name, v := range t.Tools {
		if inAnySet(name, sets) {
			continue
		}
		n += v
	}
	return n
}

func inAnySet(name string, sets []map[string]bool) bool {
	for _, s := range sets {
		if s[name] {
			return true
		}
	}
	return false
}

func (t *Task) HasWarn(w string) bool {
	for _, s := range t.Warn {
		if s == w {
			return true
		}
	}
	return false
}

func (t *Task) AddWarn(w string) {
	if t.HasWarn(w) {
		return
	}
	t.Warn = append(t.Warn, w)
}

// Session 은 JSONL 파일 하나다.
type Session struct {
	SessionID  string     `json:"session_id"`
	Project    string     `json:"project"`
	Title      string     `json:"title"`
	Start      time.Time  `json:"start"`
	End        time.Time  `json:"end"`
	CostUSD    float64    `json:"cost_usd"`
	TotalMs    int64      `json:"total_ms"`
	APIMs      int64      `json:"api_ms"`
	ToolMs     int64      `json:"tool_ms"`
	StateUsage ModelUsage `json:"state_usage"`
	SumUsage   ModelUsage `json:"sum_usage"`
	CoverPct   float64    `json:"cover_pct"`
	InProgress bool       `json:"in_progress"`
	TaskCount  int        `json:"task_count"`
}

// 살균·비밀검사를 받아야 하는 글 칸. 칸이 늘면 여기 한 줄만 는다.
func (t *Task) SanitizableFields() []*string {
	out := []*string{&t.Title}
	for i := range t.Agents {
		out = append(out, t.Agents[i].SanitizableFields()...)
	}
	return out
}

func (a *Agent) SanitizableFields() []*string {
	return []*string{&a.Description}
}

func (s *Session) SanitizableFields() []*string {
	return []*string{&s.Title}
}
