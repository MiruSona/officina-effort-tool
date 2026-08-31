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
)

// AllClasses 는 표를 찍는 차례다.
var AllClasses = []Class{
	ClassResearch, ClassDesign, ClassBuild,
	ClassMeasure, ClassDoc, ClassReview, ClassUnknown,
}

func IsClass(s string) bool {
	for _, c := range AllClasses {
		if string(c) == s {
			return true
		}
	}
	return false
}

// Agent 는 서브에이전트 한 건이다.
type Agent struct {
	AgentID     string     `json:"agent_id"`
	AgentType   string     `json:"agent_type"`
	Description string     `json:"desc"`
	Model       string     `json:"model"`
	SpawnDepth  int        `json:"depth"`
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
	PromptID  string     `json:"prompt_id"`
	SessionID string     `json:"session_id"`
	Project   string     `json:"project"`
	Title     string     `json:"title"`
	Start     time.Time  `json:"start"`
	End       time.Time  `json:"end"`
	WallMs    int64      `json:"wall_ms"`
	PureMs    int64      `json:"pure_ms"`
	Usage     ModelUsage `json:"usage"`
	MainUsage ModelUsage `json:"main_usage"`
	Agents    []Agent    `json:"agents"`
	Turns     int        `json:"turns"`
	Class     Class      `json:"class"`
	ClassBy   string     `json:"class_by"`
	Version   string     `json:"version"`
	Origin    string     `json:"origin"`
	Warn      []string   `json:"warn"`
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
