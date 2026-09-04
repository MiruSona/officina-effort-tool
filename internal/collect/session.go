package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/jsonl"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

const preambleID = "__preamble"

// 경고 낱말.
const (
	// 세션에 cost-state 줄이 없다. 도는 중일 수도, 그냥 안 적힌 것일 수도 있다 (실물의 91%).
	WarnInProgress = "cost-state없음"
	WarnNoSubagent = "서브기록없음"
	WarnNoPureMs   = "순수시간없음"
	// turn_duration 합이 벽시계를 넘어 잘라 냈다 — 배경 갈래 수명이 통째로 붙은 것이다.
	WarnPureClamped = "순수시간잘림"
	// 배경 갈래가 도는 동안 닫힌 턴이 있었다.
	WarnBgAgent = "배경갈래"
)

// SessionResult 는 세션 파일 하나를 읽은 결과다.
type SessionResult struct {
	Session  model.Session
	Tasks    []model.Task
	Rollback int
	Bad      int
	TooLong  int
	Total    int
	// 본줄도 아는 곁줄도 아닌 줄 종류 → 건수. 벽시계에서 뺀 것을 사람이 볼 수 있어야 한다.
	Unknown map[string]int
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

	res.Unknown = map[string]int{}
	rd := jsonl.NewReader(f)
	var line jsonl.Line
	for rd.Next(&line) {
		if line.PromptID != "" {
			cur = line.PromptID
		}
		if unknownLineType(&line) {
			res.Unknown[line.Type]++
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
	// 곁줄은 작업 구간을 못 늘린다. 늘리면 사람이 다음 말을 하기까지 기다린 시간이 앞 작업에 붙는다.
	if !line.Timestamp.IsZero() && isMainLine(line) {
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
		applyUserLine(t, line)
	}
	if line.Type == "system" && line.Subtype == "turn_duration" {
		t.PureMs += line.DurationMs
		if line.PendingBgAgents > 0 {
			t.AddWarn(WarnBgAgent)
		}
	}
	if line.Type == "assistant" {
		putAssistant(b.d, line)
		countTools(t, line)
	}
}

// applyUserLine 은 user 줄에서 제목·출처·알림 열쇠를 뽑는다. 알림 본문은 저장하지 않는다.
func applyUserLine(t *model.Task, line *jsonl.Line) {
	text := line.UserText()
	if t.Title == "" {
		t.Title = firstLineOf(text, 60)
	}
	if t.Origin == "" {
		t.Origin = line.Origin.Kind
	}
	if t.PromptSource == "" {
		t.PromptSource = model.NormalizeSource(line.PromptSource)
	}
	if t.NotifyTaskID != "" || !isNotifyText(text) {
		return
	}
	id, toolUse := parseNotifyTags(text)
	// 꼬리표가 아예 없는 알림은 열쇠도 종류도 못 정한다. 옛 이어받기로 떨어뜨린다.
	if id == "" {
		return
	}
	t.NotifyTaskID = id
	// task-id 는 있는데 tool-use-id 가 없으면 서브에이전트 완료가 아니라 Monitor 감시 신호다.
	if toolUse == "" {
		t.NotifyKind = model.NotifyMonitor
		return
	}
	t.NotifyKind = model.NotifyAgent
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
	applyWallTime(t)
	if t.PureMs == 0 {
		t.AddWarn(WarnNoPureMs)
	}
	// 순수시간은 벽시계를 못 넘는다. 넘었다면 배경 갈래 수명이 통째로 붙은 것이다.
	if t.WallMs > 0 && t.PureMs > t.WallMs {
		t.PureMs = t.WallMs
		t.AddWarn(WarnPureClamped)
	}
	if noSubagentDir {
		t.AddWarn(WarnNoSubagent)
	}
	if t.Class == "" {
		t.Class = model.ClassUnknown
	}
}

// applyWallTime 은 본줄 구간만 벽시계로 삼는다 (결정 2026-09-04).
// 서브에이전트 구간은 AgentWallMs 에 따로 담아 참고값으로만 쓴다 — 권한 승인을 안 눌러
// 갈래가 몇 시간씩 살아 있던 예외가 섞이면 공수 눈금이 그 예외에 끌려가기 때문이다.
func applyWallTime(t *model.Task) {
	t.MainWallMs = 0
	if !t.Start.IsZero() && !t.End.IsZero() {
		t.MainWallMs = t.End.Sub(t.Start).Milliseconds()
	}
	agentSpans := make([]Span, 0, len(t.Agents))
	for _, a := range t.Agents {
		agentSpans = append(agentSpans, Span{Start: a.Start, End: a.End})
	}
	t.AgentWallMs = UnionMs(agentSpans)
	t.WallMs = t.MainWallMs
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
