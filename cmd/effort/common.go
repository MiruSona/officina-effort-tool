package main

import (
	"encoding/json"
	"os"
	"time"

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
func filterTasks(tasks []model.Task, class, since, session string) ([]model.Task, error) {
	var cut time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return nil, fail(exitUsage, "--since 는 YYYY-MM-DD 꼴입니다 : %s", since)
		}
		cut = t
	}
	if class != "" && !model.IsClass(class) {
		return nil, fail(exitUsage, "모르는 분류 : %s", class)
	}
	var out []model.Task
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
		out = append(out, t)
	}
	return out, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
