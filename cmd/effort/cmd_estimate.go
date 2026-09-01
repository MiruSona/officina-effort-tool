package main

import (
	"fmt"
	"os"

	"github.com/mirusona/efforttool/internal/estimate"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/render"
	"github.com/mirusona/efforttool/internal/store"
)

func cmdEstimate(args []string) error {
	fs := newFlags("estimate")
	home := fs.String("home", "", "파생 저장소 자리")
	from := fs.String("from", "", "표 파일 (- 면 표준입력)")
	human := fs.Bool("human", false, "사람 눈금 칸도 같이 찍는다")
	metric := fs.String("metric", "wall", "wall|pure")
	unit := fs.String("unit", estimate.UnitGroup, "표본 단위 : group|task")
	keyStr := fs.String("group", "", "묶기 열쇠 : mark|gap:30m|class|session|none")
	since := fs.Int("days", 120, "표본으로 볼 지난 날 수")
	noX2 := fs.Bool("no-x2", false, "실측 ×2 규칙 끄기")
	freeze := fs.Bool("freeze", false, "동결→재측정 한 바퀴 ×1.5")
	wide := fs.Bool("wide", false, "화면 정렬 표")
	asJSON := fs.Bool("json", false, "JSON 으로")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *metric != "wall" && *metric != "pure" {
		return fail(exitUsage, "--metric 은 wall 또는 pure 입니다 : %s", *metric)
	}
	if *unit != estimate.UnitGroup && *unit != estimate.UnitTask {
		return fail(exitUsage, "--unit 은 group 또는 task 입니다 : %s", *unit)
	}
	if fs.NArg() > 0 && *from != "" {
		return fail(exitUsage, "소단계 인자와 --from 은 같이 못 씁니다. 하나만 주세요.")
	}
	items, err := readItems(fs.Args(), *from)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fail(exitNoData, "예상할 소단계가 0건입니다.")
	}

	st, tasks, err := readCache(*home)
	if err != nil {
		return err
	}
	rules, err := st.LoadRules()
	if err != nil {
		return fail(exitUsage, "%v", err)
	}
	if err := estimate.CheckItems(items, rules); err != nil {
		return fail(exitUsage, "%v", err)
	}
	opt := estimate.DefaultOptions()
	opt.Metric = *metric
	opt.Unit = *unit
	opt.SinceDay = *since
	opt.NoX2 = *noX2
	opt.Freeze = *freeze
	opt.Human = *human
	samples, err := samplesFor(st, tasks, opt.Unit, *keyStr)
	if err != nil {
		return err
	}
	e := estimate.New(rules, samples, opt)

	rows := make([]estimate.Row, 0, len(items))
	for _, it := range items {
		rows = append(rows, e.Estimate(it, opt))
	}
	if *asJSON {
		return printJSON(rows)
	}
	printEstimateTable(rows, *human, *wide)
	return nil
}

// samplesFor 는 표본 단위에 맞는 표본을 만든다. 기본은 묶음(소단계) 단위다.
func samplesFor(st *store.Store, tasks []model.Task, unit, keyStr string) ([]estimate.Sample, error) {
	if unit == estimate.UnitTask {
		return estimate.SamplesFromTasks(tasks), nil
	}
	groups, _, err := buildGroups(st, tasks, keyStr)
	if err != nil {
		return nil, err
	}
	return estimate.SamplesFromGroups(groups), nil
}

func readItems(args []string, from string) ([]estimate.Item, error) {
	if from == "" {
		items, err := estimate.ParseArgs(args)
		if err != nil {
			return nil, fail(exitUsage, "%v", err)
		}
		return items, nil
	}
	if from == "-" {
		items, err := estimate.ParseTable(os.Stdin)
		if err != nil {
			return nil, fail(exitUsage, "%v", err)
		}
		return items, nil
	}
	f, err := os.Open(from)
	if err != nil {
		return nil, fail(exitRead, "표 파일을 못 읽었습니다 : %v", err)
	}
	defer f.Close()
	items, err := estimate.ParseTable(f)
	if err != nil {
		return nil, fail(exitUsage, "%v", err)
	}
	return items, nil
}

func printEstimateTable(rows []estimate.Row, human, wide bool) {
	head := []string{"소단계", "분류", "예상", "범위", "근거"}
	if human {
		head = []string{"소단계", "분류", "예상", "범위", "사람눈금", "근거"}
	}
	var sum, lo, hi, hsum int64
	out := make([][]string, 0, len(rows)+1)
	for _, r := range rows {
		sum += r.P50Ms
		lo += r.P20Ms
		hi += r.P80Ms
		hsum += r.HumanMs
		cells := []string{
			r.Name, string(r.Class), render.Minutes(r.P50Ms),
			render.Minutes(r.P20Ms) + "~" + render.Minutes(r.P80Ms),
		}
		if human {
			cells = append(cells, render.Minutes(r.HumanMs))
		}
		out = append(out, append(cells, r.Source))
	}
	tail := []string{"**합**", "", "**" + render.Minutes(sum) + "**",
		"**" + render.Minutes(lo) + "~" + render.Minutes(hi) + "**"}
	if human {
		tail = append(tail, "**"+render.Minutes(hsum)+"**")
	}
	out = append(out, append(tail, ""))
	style := render.Pipe
	if wide {
		style = render.Wide
	}
	fmt.Print(render.Table(head, out, style))
	fmt.Println("\n합의 범위는 각 칸의 단순 합이다 (분산 합이 아니다 — 사람이 검산할 수 있게).")
}
