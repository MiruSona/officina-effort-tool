package collect

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirusona/efforttool/internal/jsonl"
	"github.com/mirusona/efforttool/internal/model"
)

// AgentResult 는 서브에이전트 파일 하나를 읽은 결과다.
type AgentResult struct {
	PromptID string
	Agent    model.Agent
	Rollback int
	Bad      int
	Total    int
}

// ReadAgentFile 은 agent-*.jsonl 하나와 짝 meta.json 을 읽는다.
func ReadAgentFile(path string) (AgentResult, error) {
	var res AgentResult
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()

	res.Agent.AgentID = agentIDFromPath(path)
	res.Agent.Usage = model.ModelUsage{}
	readAgentMeta(path, &res.Agent)

	d := NewDedup()
	rd := jsonl.NewReader(f)
	var line jsonl.Line
	for rd.Next(&line) {
		if res.PromptID == "" && line.PromptID != "" {
			res.PromptID = line.PromptID
		}
		if line.AgentID != "" && res.Agent.AgentID == "" {
			res.Agent.AgentID = line.AgentID
		}
		// 갈래 파일에도 곁줄이 섞일 수 있다. 구간을 늘리는 줄 종류는 본줄 하나로 묶어 둔다.
		if !line.Timestamp.IsZero() && isMainLine(&line) {
			if res.Agent.Start.IsZero() || line.Timestamp.Before(res.Agent.Start) {
				res.Agent.Start = line.Timestamp
			}
			if line.Timestamp.After(res.Agent.End) {
				res.Agent.End = line.Timestamp
			}
		}
		if line.Type == "assistant" {
			putAssistant(d, &line)
		}
	}
	if err := rd.Err(); err != nil && err != io.EOF {
		return res, err
	}
	res.Agent.Usage = d.SumByModel()
	res.Agent.Turns = d.Count()
	res.Rollback = d.Rollback
	res.Bad = rd.Bad
	res.Total = rd.Total
	return res, nil
}

// putAssistant 는 assistant 줄 하나를 중복 제거기에 넣는다.
func putAssistant(d *Dedup, line *jsonl.Line) {
	name := model.Normalize(line.Message.Model)
	k := usageKey{RequestID: line.RequestID, MessageID: line.Message.ID, Model: name}
	if k.RequestID == "" {
		// requestId 가 없는 줄은 uuid 로 열쇠를 만들어 절대 안 뭉친다.
		k.MessageID = line.UUID
	}
	u := line.Message.Usage
	d.Put(k, model.Usage{
		Input:       u.InputTokens,
		Output:      u.OutputTokens,
		CacheRead:   u.CacheReadTokens,
		CacheCreate: u.CacheCreationTokens,
		Thinking:    u.OutputDetails.ThinkingTokens,
	})
}

func agentIDFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".jsonl")
	return strings.TrimPrefix(base, "agent-")
}

func readAgentMeta(jsonlPath string, a *model.Agent) {
	metaPath := strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json"
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		a.MetaMissing = true
		a.AgentType = "미상"
		return
	}
	var m jsonl.Meta
	if json.Unmarshal(raw, &m) != nil {
		a.MetaMissing = true
		a.AgentType = "미상"
		return
	}
	a.AgentType = m.AgentType
	a.Description = m.Description
	a.ToolUseID = m.ToolUseID
	a.ParentAgentID = m.ParentAgentID
	a.SpawnDepth = m.SpawnDepth
	a.Model = model.Normalize(m.Model)
	if a.AgentType == "" {
		a.AgentType = "미상"
	}
}
