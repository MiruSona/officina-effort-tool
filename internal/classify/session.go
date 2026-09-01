package classify

import (
	"sort"
	"time"

	"github.com/mirusona/efforttool/internal/model"
)

// ClassifySession 은 한 세션의 작업을 시각 차례로 분류한다.
// 알림이 앞 작업의 분류를 물려받으므로 작업 하나만 봐서는 못 정한다.
func (r *Rules) ClassifySession(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Start.Before(tasks[j].Start)
	})
	idx := agentIndex(tasks)
	var prev model.Class
	var prevEnd time.Time
	for i := range tasks {
		c, by := r.TaskWith(&tasks[i], Ctx{Prev: prev, PrevEnd: prevEnd, Agents: idx})
		tasks[i].Class = c
		tasks[i].ClassBy = by
		// 물려받은 값은 다시 물려주지 않는다. 세션이 한 분류로 물드는 것을 막는다.
		if c.IsWork() && !isInherited(by) {
			prev = c
		}
		// 시각 창은 바로 앞 작업 기준이다. 일 칸이 아니어도 갱신한다.
		prevEnd = tasks[i].End
	}
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
