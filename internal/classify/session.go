package classify

import (
	"sort"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// ClassifySession 은 한 세션의 작업을 시각 차례로 분류한다.
// 알림이 앞 작업의 분류를 물려받으므로 작업 하나만 봐서는 못 정한다.
func (r *Rules) ClassifySession(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Start.Before(tasks[j].Start)
	})
	idx := agentIndex(tasks)
	var prevWork model.Class
	var prevEnd time.Time
	chainAlive := false
	gap := time.Duration(r.ContMax) * time.Minute
	for i := range tasks {
		if chainAlive && !continuous(prevEnd, tasks[i].Start, gap) {
			chainAlive = false
		}
		c, by := r.TaskWith(&tasks[i], Ctx{PrevWork: prevWork, ChainAlive: chainAlive, Agents: idx})
		tasks[i].Class = c
		tasks[i].ClassBy = by
		if c.IsWork() {
			// 알림도 사슬을 잇는다 — 서브에이전트를 기다린 시간은 대화가 끊긴 것이 아니다.
			chainAlive = true
			// 다만 물려받은 값은 다시 물려주지 않는다. 세션이 한 분류로 물드는 것을 막는다.
			if !isInherited(by) {
				prevWork = c
			}
		}
		prevEnd = tasks[i].End
	}
}

// continuous 는 앞 작업이 끝난 뒤 gap 안에 다음 작업이 시작했는지다.
// 시각이 없거나 거꾸로 가면 사슬이 끊긴 것으로 본다 — 없는 값을 이어졌다고 치지 않는다.
func continuous(prevEnd, start time.Time, gap time.Duration) bool {
	if prevEnd.IsZero() || start.IsZero() || start.Before(prevEnd) {
		return false
	}
	return start.Sub(prevEnd) <= gap
}

// agentIndex 는 세션 안 모든 작업의 서브에이전트를 AgentID 로 색인한다.
// AgentID 는 파일 이름 agent-<task-id>.jsonl 에서 나오므로 알림 본문의 task-id 와 같은 값이다.
func agentIndex(tasks []model.Task) map[string]*model.Agent {
	out := map[string]*model.Agent{}
	for i := range tasks {
		for j := range tasks[i].Agents {
			a := &tasks[i].Agents[j]
			if a.AgentID == "" {
				continue
			}
			out[a.AgentID] = a
		}
	}
	return out
}
