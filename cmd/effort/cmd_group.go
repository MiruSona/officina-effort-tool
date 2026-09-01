package main

import (
	"fmt"
	"time"

	"github.com/mirusona/efforttool/internal/group"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/render"
	"github.com/mirusona/efforttool/internal/store"
)

func cmdGroup(args []string) error {
	fs := newFlags("group")
	home := fs.String("home", "", "파생 저장소 자리")
	add := fs.String("add", "", "이 이름으로 소단계를 표시한다 (작업id 를 뒤에 적는다)")
	class := fs.String("class", "", "표시할 소단계의 분류")
	drop := fs.String("drop", "", "이 묶음 id 를 정본에서 뺀다")
	since := fs.String("since", "", "이 날부터 (YYYY-MM-DD)")
	session := fs.String("session", "", "세션 하나만")
	keyStr := fs.String("group", "", "mark|gap:30m|class|session|none")
	all := fs.Bool("all", false, "일 아닌 칸까지 보기")
	wide := fs.Bool("wide", false, "화면 정렬 표")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *add != "" && *drop != "" {
		return fail(exitUsage, "--add 와 --drop 은 같이 못 씁니다. 하나만 주세요.")
	}
	if *drop != "" {
		return dropGroup(*home, *drop)
	}
	if *add != "" {
		if err := addGroup(*home, *add, *class, fs.Args()); err != nil {
			return err
		}
	} else if fs.NArg() > 0 {
		return fail(exitUsage, "작업 id 는 --add 와 같이 씁니다 : effort group --add \"<이름>\" <작업id…>")
	}
	return listGroups(*home, *class, *since, *session, *keyStr, *all, *wide, *asJSON)
}

// addGroup 은 정본에 소단계 하나를 덧붙인다. 기존 줄은 안 건드린다.
func addGroup(home, name, class string, taskIDs []string) error {
	if len(taskIDs) == 0 {
		return fail(exitUsage, "표시할 작업 id 를 적어 주세요 : effort group --add \"<이름>\" <작업id…>")
	}
	if class != "" && !model.IsClass(class) {
		return fail(exitUsage, "모르는 분류 : %s", class)
	}
	st, _, err := openStore(home, "")
	if err != nil {
		return err
	}
	m := store.Mark{ID: newGroupID(time.Now()), Name: name, Class: model.Class(class), Tasks: taskIDs}
	existing, err := st.LoadGroups()
	if err != nil {
		return fail(exitUsage, "%v", err)
	}
	m.ID = freeGroupID(m.ID, existing)
	if err := st.AppendGroup(m); err != nil {
		return fail(exitWrite, "정본에 못 썼습니다 : %v", err)
	}
	fmt.Printf("소단계 %s 「%s」 에 작업 %d건을 넣었습니다. (%s)\n",
		m.ID, name, len(taskIDs), st.GroupsPath())
	return nil
}

func dropGroup(home, id string) error {
	st, _, err := openStore(home, "")
	if err != nil {
		return err
	}
	n, err := st.DropGroup(id)
	if err != nil {
		return fail(exitWrite, "정본을 못 고쳤습니다 : %v", err)
	}
	if n == 0 {
		return fail(exitNoData, "정본에 그런 묶음이 없습니다 : %s", id)
	}
	fmt.Printf("소단계 %s 를 정본에서 뺐습니다 (줄 %d개). (%s)\n", id, n, st.GroupsPath())
	return nil
}

func listGroups(home, class, since, session, keyStr string, all, wide, asJSON bool) error {
	st, tasks, err := readCache(home)
	if err != nil {
		return err
	}
	tasks, _, err = filterTasks(tasks, class, since, session, all)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 작업이 0건입니다.")
	}
	groups, missing, err := buildGroups(st, tasks, keyStr)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSON(groups)
	}
	printMissingMarks(missing)
	fmt.Println(render.DataNotice)
	fmt.Print(groupTable(groups, wide))
	return nil
}

// groupTable 은 묶음 표 글을 만든다. list --group 도 같은 표를 쓴다.
func groupTable(groups []group.Group, wide bool) string {
	head := []string{"묶음", "표시", "분류", "작업", "벽시계", "순수", "토큰", "이름"}
	rows := make([][]string, 0, len(groups))
	for _, g := range groups {
		mark := "자동"
		if g.Marked {
			mark = "사람"
		}
		rows = append(rows, []string{
			g.ID, mark, string(g.Class), fmt.Sprintf("%d", len(g.Tasks)),
			render.Minutes(g.WallMs), render.Minutes(g.PureMs),
			render.Tokens(g.Tokens), g.Name,
		})
	}
	style := render.Pipe
	if wide {
		style = render.Wide
	}
	return render.Table(head, rows, style)
}

// newGroupID 는 g<날짜>-01 꼴 묶음 id 를 만든다.
func newGroupID(now time.Time) string {
	return "g" + now.Format("20060102") + "-01"
}

// freeGroupID 는 이미 쓰는 id 를 피해 뒷번호를 올린다.
func freeGroupID(want string, existing []store.Mark) string {
	used := map[string]bool{}
	for _, m := range existing {
		used[m.ID] = true
	}
	if !used[want] {
		return want
	}
	base := want[:len(want)-2]
	for i := 2; i < 100; i++ {
		id := fmt.Sprintf("%s%02d", base, i)
		if !used[id] {
			return id
		}
	}
	return want + "-x"
}
