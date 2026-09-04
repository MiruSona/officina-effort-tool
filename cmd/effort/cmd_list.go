package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/estimate"
	"github.com/mirusona/officina-effort-tool/internal/group"
	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/render"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

func cmdList(args []string) error {
	fs := newFlags("list")
	home := fs.String("home", "", "파생 저장소 자리")
	class := fs.String("class", "", "분류 하나만")
	since := fs.String("since", "", "이 날부터 (YYYY-MM-DD)")
	session := fs.String("session", "", "세션 하나만")
	limit := fs.Int("limit", 20, "몇 줄까지")
	sortBy := fs.String("sort", "wall", "wall|pure|tok")
	keyStr := fs.String("group", "", "묶음으로 본다 : mark|gap:30m|class|session|none")
	all := fs.Bool("all", false, "일 아닌 칸(대화·도구실행·잡무)까지 보기")
	wide := fs.Bool("wide", false, "화면 정렬 표")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	st, tasks, err := readCache(*home)
	if err != nil {
		return err
	}
	if *keyStr != "" {
		return listAsGroups(st, tasks, groupPick{
			key: *keyStr, class: *class, since: *since, session: *session, all: *all,
		}, *limit, *asJSON, *wide)
	}
	tasks, _, err = filterTasks(tasks, *class, *since, *session, *all)
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
			shortID(t.PromptID), t.Start.Format("01-02 15:04"), classLabel(&t),
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

// groupPick 은 묶음을 고르는 조건이다.
type groupPick struct {
	key     string
	class   string
	since   string
	session string
	all     bool
}

// listAsGroups 는 작업 대신 소단계 묶음을 한 줄씩 찍는다.
// 작업을 분류로 먼저 거르면 사이에 낀 대화가 묶음을 쪼개서, 배율을 잰 표본과 보는 표본이 갈린다.
// 그래서 estimate 와 같은 차례로 간다 — 날짜·세션만 먼저 거르고, 분류는 묶은 「뒤」에 대표 분류로 본다.
func listAsGroups(st *store.Store, tasks []model.Task, pick groupPick, limit int, asJSON, wide bool) error {
	if pick.class != "" && !model.IsClass(pick.class) {
		return fail(exitUsage, "모르는 분류 : %s", pick.class)
	}
	tasks, _, err := filterTasks(tasks, "", pick.since, pick.session, true)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 작업이 0건입니다.")
	}
	groups, missing, err := buildGroups(st, tasks, pick.key)
	if err != nil {
		return err
	}
	groups = pickGroups(groups, pick.class, pick.all)
	if len(groups) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 묶음이 0건입니다. (--all 로 일 아닌 묶음까지 볼 수 있습니다)")
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].WallMs > groups[j].WallMs })
	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}
	if asJSON {
		return printJSON(groups)
	}
	printMissingMarks(missing)
	fmt.Println(render.DataNotice)
	fmt.Print(groupTable(groups, wide))
	return nil
}

// pickGroups 는 볼 묶음을 고른다. --all 이면 전부고, 아니면 estimate 표본과 똑같은 잣대다
// (대표 분류가 일 칸 · 「진행 중」 아님). --class 를 같이 주면 그중 대표 분류가 그것인 묶음만이다.
func pickGroups(gs []group.Group, class string, all bool) []group.Group {
	out := make([]group.Group, 0, len(gs))
	for i := range gs {
		if !all && !estimate.KeepSample(gs[i].Class, gs[i].Warn) {
			continue
		}
		if class != "" && string(gs[i].Class) != class {
			continue
		}
		out = append(out, gs[i])
	}
	return out
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
