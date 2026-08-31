package classify

import (
	"sort"

	"github.com/mirusona/efforttool/internal/model"
)

// ClassifySession 은 한 세션의 작업을 시각 차례로 분류한다.
// 알림이 앞 작업의 분류를 물려받으므로 작업 하나만 봐서는 못 정한다.
func (r *Rules) ClassifySession(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Start.Before(tasks[j].Start)
	})
	var prev model.Class
	for i := range tasks {
		c, by := r.TaskWith(&tasks[i], Ctx{Prev: prev})
		tasks[i].Class = c
		tasks[i].ClassBy = by
		// 물려받은 값은 다시 물려주지 않는다. 세션이 한 분류로 물드는 것을 막는다.
		if c.IsWork() && by != ByInherit {
			prev = c
		}
	}
}
