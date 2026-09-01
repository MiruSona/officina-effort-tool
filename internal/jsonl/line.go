package jsonl

import (
	"encoding/json"
	"time"
)

// RawUsage 는 assistant 줄의 message.usage 다.
type RawUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
	OutputDetails       struct {
		ThinkingTokens int64 `json:"thinking_tokens"`
	} `json:"output_tokens_details"`
}

type RawMessage struct {
	ID      string          `json:"id"`
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   RawUsage        `json:"usage"`
}

// StateUsage 는 cost-state 의 modelUsage 한 칸이다.
type StateUsage struct {
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheReadTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationInputTokens"`
	CostUSD             float64 `json:"costUSD"`
}

type Origin struct {
	Kind string `json:"kind"`
}

// Line 은 JSONL 한 줄에서 이 툴이 보는 칸만 뽑은 것이다.
type Line struct {
	Type      string     `json:"type"`
	Subtype   string     `json:"subtype"`
	UUID      string     `json:"uuid"`
	PromptID  string     `json:"promptId"`
	AgentID   string     `json:"agentId"`
	SessionID string     `json:"sessionId"`
	RequestID string     `json:"requestId"`
	Timestamp time.Time  `json:"timestamp"`
	CWD       string     `json:"cwd"`
	Version   string     `json:"version"`
	Message   RawMessage `json:"message"`
	Origin    Origin     `json:"origin"`

	DurationMs int64 `json:"durationMs"`

	// PromptSource 는 프롬프트가 어디서 왔는지다 (typed·system·queued·sdk·suggestion_accepted).
	PromptSource string `json:"promptSource"`
	// PendingBgAgents 는 이 턴이 닫힐 때 아직 도는 배경 갈래 수다.
	PendingBgAgents int `json:"pendingBackgroundAgentCount"`

	AITitle string `json:"aiTitle"`

	TotalCostUSD      float64               `json:"totalCostUSD"`
	TotalAPIDuration  int64                 `json:"totalAPIDuration"`
	TotalToolDuration int64                 `json:"totalToolDuration"`
	TotalDuration     int64                 `json:"totalDuration"`
	ModelUsage        map[string]StateUsage `json:"modelUsage"`
}

// Meta 는 서브에이전트의 agent-*.meta.json 이다.
type Meta struct {
	AgentType     string `json:"agentType"`
	Description   string `json:"description"`
	ToolUseID     string `json:"toolUseId"`
	ParentAgentID string `json:"parentAgentId"`
	SpawnDepth    int    `json:"spawnDepth"`
	Model         string `json:"model"`
}

// 도구 이름이 이보다 길면 우리가 아는 도구가 아니다. 표를 깨지 않게 버린다.
const maxToolNameLen = 48

// ToolNames 는 assistant 줄 message.content 의 tool_use 블록 이름을 뽑는다.
// input 은 명령줄·경로가 들어 있어 일부러 안 읽는다.
func (l *Line) ToolNames() []string {
	if len(l.Message.Content) == 0 {
		return nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return nil
	}
	var out []string
	for _, b := range blocks {
		if b.Type != "tool_use" || !okToolName(b.Name) {
			continue
		}
		out = append(out, b.Name)
	}
	return out
}

func okToolName(s string) bool {
	if s == "" || len(s) > maxToolNameLen {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// UserText 는 user 줄의 message.content 에서 첫 글 조각을 꺼낸다.
func (l *Line) UserText() string {
	if len(l.Message.Content) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(l.Message.Content, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return ""
	}
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			return b.Text
		}
	}
	return ""
}
