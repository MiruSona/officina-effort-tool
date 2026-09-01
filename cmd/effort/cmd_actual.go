package main

import (
	"fmt"
	"strings"

	"github.com/mirusona/efforttool/internal/estimate"
	"github.com/mirusona/efforttool/internal/group"
	"github.com/mirusona/efforttool/internal/render"
)

// actualRow 는 예상 한 줄과 거기 붙은 실제 한 줄이다.
type actualRow struct {
	Name    string  `json:"name"`
	Class   string  `json:"class"`
	PlanMs  int64   `json:"plan_ms"`
	RealMs  int64   `json:"real_ms"`
	Ratio   float64 `json:"ratio"`
	GroupID string  `json:"group_id"`
	Marked  bool    `json:"marked"`
	Matched bool    `json:"matched"`
}

func cmdActual(args []string) error {
	fs := newFlags("actual")
	home := fs.String("home", "", "파생 저장소 자리")
	from := fs.String("from", "", "예상 표 파일 (- 면 표준입력)")
	since := fs.String("since", "", "이 날부터 (YYYY-MM-DD)")
	session := fs.String("session", "", "세션 하나만")
	keyStr := fs.String("group", "", "묶기 열쇠 : mark|gap:30m|class|session|none")
	match := fs.String("match", "name", "name|order")
	metric := fs.String("metric", "wall", "wall|pure")
	wide := fs.Bool("wide", false, "화면 정렬 표")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *from == "" {
		return fail(exitUsage, "예상 표를 주세요 : effort actual --from <공수표.md>")
	}
	if *match != "name" && *match != "order" {
		return fail(exitUsage, "--match 는 name 또는 order 입니다 : %s", *match)
	}
	if *metric != "wall" && *metric != "pure" {
		return fail(exitUsage, "--metric 은 wall 또는 pure 입니다 : %s", *metric)
	}
	items, err := readItems(nil, *from)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fail(exitNoData, "예상 표에서 읽은 소단계가 0건입니다.")
	}

	st, tasks, err := readCache(*home)
	if err != nil {
		return err
	}
	tasks, _, err = filterTasks(tasks, "", *since, *session, true)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 작업이 0건입니다.")
	}
	rules, err := st.LoadRules()
	if err != nil {
		return fail(exitUsage, "%v", err)
	}
	if err := estimate.CheckItems(items, rules); err != nil {
		return fail(exitUsage, "%v", err)
	}
	groups, missing, err := buildGroups(st, tasks, *keyStr)
	if err != nil {
		return err
	}
	opt := estimate.DefaultOptions()
	opt.Metric = *metric
	e := estimate.New(rules, estimate.SamplesFromGroups(groups), opt)

	rows := matchActual(items, groups, e, opt, *match, *metric)
	if *asJSON {
		return printJSON(rows)
	}
	printMissingMarks(missing)
	printActualTable(rows, *wide)
	return nil
}

// matchActual 은 예상 줄에 묶음을 붙인다. 못 찾은 줄은 비워 둔다 — 없는 값을 지어내지 않는다.
func matchActual(items []estimate.Item, groups []group.Group, e *estimate.Estimator,
	opt estimate.Options, match, metric string) []actualRow {

	used := map[int]bool{}
	out := make([]actualRow, 0, len(items))
	for i, it := range items {
		r := actualRow{Name: it.Name, Class: string(it.Class), PlanMs: e.Estimate(it, opt).P50Ms}
		j := -1
		if match == "name" {
			j = findByName(groups, it.Name, used)
		} else if i < len(groups) {
			j = i
		}
		if j >= 0 && !used[j] {
			used[j] = true
			g := groups[j]
			r.Matched = true
			r.GroupID = g.ID
			r.Marked = g.Marked
			r.RealMs = g.WallMs
			if metric == "pure" {
				r.RealMs = g.PureMs
			}
			if r.PlanMs > 0 {
				r.Ratio = float64(r.RealMs) / float64(r.PlanMs)
			}
		}
		out = append(out, r)
	}
	return out
}

// findByName 은 이름이 같은 묶음을 찾는다. 앞뒤 빈칸만 접고 글자 그대로 맞춘다.
func findByName(groups []group.Group, name string, used map[int]bool) int {
	want := strings.TrimSpace(name)
	for i := range groups {
		if used[i] || strings.TrimSpace(groups[i].Name) != want {
			continue
		}
		return i
	}
	return -1
}

func printActualTable(rows []actualRow, wide bool) {
	fmt.Println(render.DataNotice)
	head := []string{"소단계", "분류", "예상", "실제", "배율", "묶음"}
	var planSum, realSum int64
	out := make([][]string, 0, len(rows)+1)
	for _, r := range rows {
		planSum += r.PlanMs
		cells := []string{r.Name, r.Class, render.Minutes(r.PlanMs)}
		if !r.Matched {
			out = append(out, append(cells, "—", "—", "못 찾음"))
			continue
		}
		realSum += r.RealMs
		out = append(out, append(cells,
			render.Minutes(r.RealMs), fmt.Sprintf("%.2f", r.Ratio), markLabel(r)))
	}
	total := "—"
	if planSum > 0 && realSum > 0 {
		total = fmt.Sprintf("%.2f", float64(realSum)/float64(planSum))
	}
	out = append(out, []string{"**합**", "",
		"**" + render.Minutes(planSum) + "**", "**" + render.Minutes(realSum) + "**",
		"**" + total + "**", ""})

	style := render.Pipe
	if wide {
		style = render.Wide
	}
	fmt.Print(render.Table(head, out, style))
	fmt.Println("\n배율 = 실제 ÷ 예상. 1 보다 크면 예상이 짧았다. 못 찾은 소단계는 합에서 뺐다.")
	fmt.Println("못 찾았으면 `effort group --add \"<이름>\" <작업id…>` 로 경계를 표시한 뒤 다시 돌린다.")
	fmt.Println("어긋난 까닭과 다음에 쓸 눈금은 mem 에 남긴다 (mem add --type howto --scope efforttool).")
}

func markLabel(r actualRow) string {
	if r.Marked {
		return "사람 " + r.GroupID
	}
	return "자동 " + r.GroupID
}
