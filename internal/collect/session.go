package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirusona/efforttool/internal/jsonl"
	"github.com/mirusona/efforttool/internal/model"
)

const preambleID = "__preamble"

// 경고 낱말.
const (
	// 세션에 cost-state 줄이 없다. 도는 중일 수도, 그냥 안 적힌 것일 수도 있다 (실물의 91%).
	WarnInProgress = "cost-state없음"
	WarnNoSubagent = "서브기록없음"
	WarnNoPureMs   = "순수시간없음"
	// turn_duration 합이 벽시계보다 두 배 넘게 크다 — 사람을 기다린 시간이 섞였다.
	WarnPureOverWall = "순수시간과다"
)

// 순수시간이 벽시계의 몇 배를 넘으면 믿지 않는지.
const pureOverWallLimit = 2

// SessionResult 는 세션 파일 하나를 읽은 결과다.
type SessionResult struct {
	Session  model.Session
	Tasks    []model.Task
	Rollback int
	Bad      int
	TooLong  int
	Total    int
}

type taskBuild struct {
	task model.Task
	d    *Dedup
}

// ReadSession 은 세션 JSONL 하나와 그 세션의 서브에이전트 폴더를 읽는다.
func ReadSession(path string) (SessionResult, error) {
	var res SessionResult
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	res.Session.SessionID = sessionID
	res.Session.StateUsage = model.ModelUsage{}
	res.Session.SumUsage = model.ModelUsage{}

	builds := map[string]*taskBuild{}
	var order []string
	cur := preambleID
	sawCostState := false

	rd := jsonl.NewReader(f)
	var line jsonl.Line
	for rd.Next(&line) {
		if line.PromptID != "" {
			cur = line.PromptID
		}
		if line.Type == "cost-state" {
			sawCostState = true
			applyCostState(&res.Session, &line)
			continue
		}
		if line.Type == "ai-title" && line.AITitle != "" {
			res.Session.Title = line.AITitle
			continue
		}
		touchSessionTime(&res.Session, &line)
		if cur == preambleID {
			continue
		}
		b := builds[cur]
		if b == nil {
			b = &taskBuild{d: NewDedup()}
			b.task.PromptID = cur
			b.task.SessionID = sessionID
			builds[cur] = b
			order = append(order, cur)
		}
		applyLine(b, &line)
	}
	if err := rd.Err(); err != nil {
		return res, err
	}
	res.Bad = rd.Bad
	res.TooLong = rd.TooLong
	res.Total = rd.Total

	agentsByPrompt, agentRollback, hasSubDir := readSubagents(path)
	res.Session.InProgress = !sawCostState

	for _, id := range order {
		b := builds[id]
		res.Rollback += b.d.Rollback
		finishTask(b, agentsByPrompt, !hasSubDir)
		res.Session.SumUsage.Merge(b.task.Usage)
		res.Tasks = append(res.Tasks, b.task)
	}
	res.Rollback += agentRollback

	if res.Session.InProgress && len(res.Tasks) > 0 {
		res.Tasks[len(res.Tasks)-1].AddWarn(WarnInProgress)
	}
	res.Session.TaskCount = len(res.Tasks)
	res.Session.CoverPct = coverPct(res.Session.SumUsage, res.Session.StateUsage)
	if len(res.Tasks) > 0 {
		res.Session.Project = res.Tasks[0].Project
	}
	return res, nil
}

func applyLine(b *taskBuild, line *jsonl.Line) {
	t := &b.task
	if !line.Timestamp.IsZero() {
		if t.Start.IsZero() || line.Timestamp.Before(t.Start) {
			t.Start = line.Timestamp
		}
		if line.Timestamp.After(t.End) {
			t.End = line.Timestamp
		}
	}
	if line.CWD != "" && t.Project == "" {
		t.Project = line.CWD
	}
	if line.Version != "" {
		t.Version = line.Version
	}
	if line.Type == "user" && line.PromptID != "" {
		if t.Title == "" {
			t.Title = firstLineOf(line.UserText(), 60)
		}
		if t.Origin == "" {
			t.Origin = line.Origin.Kind
		}
	}
	if line.Type == "system" && line.Subtype == "turn_duration" {
		t.PureMs += line.DurationMs
	}
	if line.Type == "assistant" {
		putAssistant(b.d, line)
		countTools(t, line)
	}
}

// countTools 는 메인 세션이 부른 도구 이름을 센다. 서브에이전트 파일은 여기로 안 온다.
func countTools(t *model.Task, line *jsonl.Line) {
	names := line.ToolNames()
	if len(names) == 0 {
		return
	}
	if t.Tools == nil {
		t.Tools = map[string]int{}
	}
	for _, n := range names {
		t.Tools[n]++
	}
}

func finishTask(b *taskBuild, agents map[string][]model.Agent, noSubagentDir bool) {
	t := &b.task
	t.MainUsage = b.d.SumByModel()
	t.Turns = b.d.Count()
	t.Usage = model.ModelUsage{}
	t.Usage.Merge(t.MainUsage)
	t.Agents = agents[t.PromptID]
	for _, a := range t.Agents {
		t.Usage.Merge(a.Usage)
	}
	if !t.End.IsZero() && !t.Start.IsZero() {
		t.WallMs = t.End.Sub(t.Start).Milliseconds()
	}
	if t.PureMs == 0 {
		t.AddWarn(WarnNoPureMs)
	}
	if t.WallMs > 0 && t.PureMs > t.WallMs*pureOverWallLimit {
		t.AddWarn(WarnPureOverWall)
	}
	if noSubagentDir {
		t.AddWarn(WarnNoSubagent)
	}
	if t.Class == "" {
		t.Class = model.ClassUnknown
	}
}

// applyCostState 는 세션 총계를 덮어쓴다. cost-state 는 그때까지의 누계라 더하면 두 배가 된다.
func applyCostState(s *model.Session, line *jsonl.Line) {
	s.StateUsage = model.ModelUsage{}
	s.CostUSD = line.TotalCostUSD
	s.TotalMs = line.TotalDuration
	s.APIMs = line.TotalAPIDuration
	s.ToolMs = line.TotalToolDuration
	for name, u := range line.ModelUsage {
		s.StateUsage.Add(model.Normalize(name), model.Usage{
			Input:       u.InputTokens,
			Output:      u.OutputTokens,
			CacheRead:   u.CacheReadTokens,
			CacheCreate: u.CacheCreationTokens,
		})
	}
}

func touchSessionTime(s *model.Session, line *jsonl.Line) {
	if line.Timestamp.IsZero() {
		return
	}
	if s.Start.IsZero() || line.Timestamp.Before(s.Start) {
		s.Start = line.Timestamp
	}
	if line.Timestamp.After(s.End) {
		s.End = line.Timestamp
	}
}

func coverPct(sum, state model.ModelUsage) float64 {
	total := state.Sum().Total()
	if total == 0 {
		return 0
	}
	return float64(sum.Sum().Total()) / float64(total) * 100
}

// readSubagents 는 <세션id>/subagents/ 를 읽어 promptId 별로 묶는다.
func readSubagents(sessionPath string) (map[string][]model.Agent, int, bool) {
	dir := strings.TrimSuffix(sessionPath, ".jsonl")
	sub := filepath.Join(dir, "subagents")
	entries, err := os.ReadDir(sub)
	if err != nil {
		return map[string][]model.Agent{}, 0, false
	}
	out := map[string][]model.Agent{}
	rollback := 0
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		r, err := ReadAgentFile(filepath.Join(sub, n))
		if err != nil {
			continue
		}
		rollback += r.Rollback
		if r.PromptID == "" {
			continue
		}
		out[r.PromptID] = append(out[r.PromptID], r.Agent)
	}
	return out, rollback, true
}

func firstLineOf(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return string(r)
}
