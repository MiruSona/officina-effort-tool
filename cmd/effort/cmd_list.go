package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/render"
)

func cmdList(args []string) error {
	fs := newFlags("list")
	home := fs.String("home", "", "파생 저장소 자리")
	class := fs.String("class", "", "분류 하나만")
	since := fs.String("since", "", "이 날부터 (YYYY-MM-DD)")
	session := fs.String("session", "", "세션 하나만")
	limit := fs.Int("limit", 20, "몇 줄까지")
	sortBy := fs.String("sort", "wall", "wall|pure|tok")
	wide := fs.Bool("wide", false, "화면 정렬 표")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	_, tasks, err := readCache(*home)
	if err != nil {
		return err
	}
	tasks, err = filterTasks(tasks, *class, *since, *session)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 작업이 0건입니다.")
	}
	if err := sortTasks(tasks, *sortBy); err != nil {
		return err
	}
	if *limit > 0 && len(tasks) > *limit {
		tasks = tasks[:*limit]
	}
	if *asJSON {
		return printJSON(tasks)
	}
	fmt.Println(render.DataNotice)
	head := []string{"작업", "날짜", "분류", "벽시계", "순수", "토큰", "에이전트", "제목"}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{
			shortID(t.PromptID), t.Start.Format("01-02 15:04"), string(t.Class),
			render.Minutes(t.WallMs), render.Minutes(t.PureMs),
			render.Tokens(t.Usage.Sum().Total()),
			fmt.Sprintf("%d", len(t.Agents)),
			t.Title,
		})
	}
	style := render.Pipe
	if *wide {
		style = render.Wide
	}
	fmt.Print(render.Table(head, rows, style))
	return nil
}

func sortTasks(tasks []model.Task, by string) error {
	switch by {
	case "wall":
		sort.Slice(tasks, func(i, j int) bool { return tasks[i].WallMs > tasks[j].WallMs })
	case "pure":
		sort.Slice(tasks, func(i, j int) bool { return tasks[i].PureMs > tasks[j].PureMs })
	case "tok":
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Usage.Sum().Total() > tasks[j].Usage.Sum().Total()
		})
	default:
		return fail(exitUsage, "--sort 는 wall|pure|tok 중 하나입니다 : %s", by)
	}
	return nil
}

func shortID(id string) string {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i]
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
