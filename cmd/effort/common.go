package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/mirusona/efforttool/internal/classify"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/store"
)

// readCache 는 캐시를 읽는다. 없으면 종료 2 로 안내한다.
func readCache(home string) (*store.Store, []model.Task, error) {
	st, _, err := openStore(home, "")
	if err != nil {
		return nil, nil, err
	}
	tasks, err := st.ReadTasks()
	if err == store.ErrNoCache {
		return nil, nil, fail(exitNoData, "캐시가 없습니다. 먼저 `effort scan` 을 돌리세요.")
	}
	if err != nil {
		return nil, nil, fail(exitRead, "캐시를 못 읽었습니다 : %v", err)
	}
	if len(tasks) == 0 {
		return nil, nil, fail(exitNoData, "캐시에 작업이 0건입니다. `effort scan --all` 을 돌려 보세요.")
	}
	return st, tasks, nil
}

// filterTasks 는 분류·날짜·세션으로 작업을 거른다.
// all 이 거짓이고 분류를 직접 고르지 않았으면 일 아닌 칸(대화·도구실행·잡무)은 뺀다.
func filterTasks(tasks []model.Task, class, since, session string, all bool) ([]model.Task, int, error) {
	var cut time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return nil, 0, fail(exitUsage, "--since 는 YYYY-MM-DD 꼴입니다 : %s", since)
		}
		cut = t
	}
	if class != "" && !model.IsClass(class) {
		return nil, 0, fail(exitUsage, "모르는 분류 : %s", class)
	}
	hideNonWork := !all && class == ""
	var out []model.Task
	dropped := 0
	for _, t := range tasks {
		if class != "" && string(t.Class) != class {
			continue
		}
		if !cut.IsZero() && t.Start.Before(cut) {
			continue
		}
		if session != "" && t.SessionID != session {
			continue
		}
		if hideNonWork && isNonWork(t.Class) {
			dropped++
			continue
		}
		out = append(out, t)
	}
	return out, dropped, nil
}

// isNonWork 는 일 아닌 칸인지다. 미분류는 사람이 봐야 하므로 여기 안 든다.
func isNonWork(c model.Class) bool {
	return c == model.ClassChat || c == model.ClassTool || c == model.ClassChore
}

// classLabel 은 표에 찍을 분류 이름이다. 물려받은 값이면 화살표를 붙인다.
func classLabel(t *model.Task) string {
	if t.ClassBy == classify.ByInherit && t.Class != model.ClassUnknown {
		return string(t.Class) + "←"
	}
	return string(t.Class)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
