package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/render"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

func cmdShow(args []string) error {
	fs := newFlags("show")
	home := fs.String("home", "", "파생 저장소 자리")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fail(exitUsage, "쓰는 법 : effort show <promptId|접두사>")
	}
	want := fs.Arg(0)
	st, tasks, err := readCache(*home)
	if err != nil {
		return err
	}
	hits := matchTasks(tasks, want)
	if len(hits) == 0 {
		return fail(exitNoData, "그런 작업이 없습니다 : %s", want)
	}
	if len(hits) > 1 {
		return fail(exitUsage, "접두사가 겹칩니다 (%d건). 더 길게 적어 주세요.", len(hits))
	}
	t := hits[0]
	if *asJSON {
		return printJSON(t)
	}
	printTask(st, &t)
	return nil
}

func matchTasks(tasks []model.Task, want string) []model.Task {
	var out []model.Task
	for _, t := range tasks {
		if t.PromptID == want {
			return []model.Task{t}
		}
		if strings.HasPrefix(t.PromptID, want) {
			out = append(out, t)
		}
	}
	return out
}

func printTask(st *store.Store, t *model.Task) {
	fmt.Println(render.DataNotice)
	fmt.Printf("작업 %s\n", t.PromptID)
	fmt.Printf("제목 : %s\n", t.Title)
	fmt.Printf("세션 : %s · 프로젝트 : %s\n", t.SessionID, t.Project)
	fmt.Printf("분류 : %s (%s) · 판 : %s · 시작 : %s\n",
		t.Class, t.ClassBy, t.Version, t.Start.Format("2006-01-02 15:04:05"))
	fmt.Printf("벽시계 %s (본줄 %s · 서브 %s) · 순수시간 %s · 턴 %d\n",
		render.Minutes(t.WallMs), render.Minutes(t.MainWallMs), render.Minutes(t.AgentWallMs),
		render.Minutes(t.PureMs), t.Turns)
	fmt.Println("벽시계는 본줄 구간과 서브에이전트 구간의 합집합이다 (겹친 만큼은 한 번만 센다).")
	if len(t.Warn) > 0 {
		fmt.Printf("표시 : %s\n", strings.Join(t.Warn, ", "))
	}
	fmt.Println()
	printModelTable(t.Usage)
	if len(t.Agents) > 0 {
		fmt.Println()
		printAgentTable(t.Agents)
	}
	printCheck(st, t)
}

func printModelTable(mu model.ModelUsage) {
	names := make([]string, 0, len(mu))
	for n := range mu {
		names = append(names, n)
	}
	sort.Strings(names)
	head := []string{"모델", "입력", "출력", "캐시읽기", "캐시생성", "합"}
	rows := make([][]string, 0, len(names))
	for _, n := range names {
		u := mu[n]
		rows = append(rows, []string{
			n, render.Tokens(u.Input), render.Tokens(u.Output),
			render.Tokens(u.CacheRead), render.Tokens(u.CacheCreate),
			render.Tokens(u.Total()),
		})
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
}

func printAgentTable(agents []model.Agent) {
	head := []string{"에이전트", "부모", "종류", "모델", "깊이", "분류", "벽시계", "토큰", "설명"}
	rows := make([][]string, 0, len(agents))
	for _, a := range agents {
		parent := "—"
		if a.ParentAgentID != "" {
			parent = shortID(a.ParentAgentID)
		}
		rows = append(rows, []string{
			shortID(a.AgentID), parent, a.AgentType, a.Model,
			fmt.Sprintf("%d", a.SpawnDepth), string(a.Class),
			render.Minutes(a.End.Sub(a.Start).Milliseconds()),
			render.Tokens(a.Usage.Sum().Total()), a.Description,
		})
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
}

// printCheck 는 세션 총계와 나란히 놓아 검산하게 한다. 배분은 안 한다.
func printCheck(st *store.Store, t *model.Task) {
	sessions, err := st.ReadSessions()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.SessionID != t.SessionID {
			continue
		}
		fmt.Println()
		if s.InProgress {
			fmt.Println("검산 : — (이 세션에 cost-state 줄이 없습니다)")
			return
		}
		fmt.Printf("검산 : 작업 합 %s / 세션 총계 %s (%.0f%%) · 세션 시간 %s · $%.4f\n",
			render.Tokens(s.SumUsage.Sum().Total()),
			render.Tokens(s.StateUsage.Sum().Total()),
			s.CoverPct, render.Minutes(s.TotalMs), s.CostUSD)
		return
	}
}
