package main

import (
	"fmt"
	"sort"

	"github.com/mirusona/officina-effort-tool/internal/estimate"
	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/render"
)

// measureMinSeed 는 시드를 새로 낼 최소 표본 수다. 이보다 적으면 옛 값을 그대로 둔다.
const measureMinSeed = 12

// runRulesMeasure 는 캐시 표본으로 크기 배율과 분류별 시드를 다시 재서
// rules.txt 에 붙여 넣을 줄을 찍는다. 파일은 안 건드린다 — rules.txt 는 사람 정본이다.
func runRulesMeasure(home string) error {
	st, tasks, err := readCache(home)
	if err != nil {
		return err
	}
	tasks, _, err = filterTasks(tasks, "", "", "", true)
	if err != nil {
		return err
	}
	groups, _, err := buildGroups(st, tasks, "")
	if err != nil {
		return err
	}
	all := []float64{}
	byClass := map[model.Class][]float64{}
	for i := range groups {
		g := &groups[i]
		// 잰 표본과 보는 표본이 갈리면 안 된다 — list --group·estimate 와 같은 잣대다.
		if !estimate.KeepSample(g.Class, g.Warn) || g.WallMs <= 0 {
			continue
		}
		m := float64(g.WallMs) / 60000
		all = append(all, m)
		byClass[g.Class] = append(byClass[g.Class], m)
	}
	if len(all) < measureMinSeed {
		return fail(exitNoData, "표본이 %d건뿐이라 다시 잴 수 없습니다.", len(all))
	}
	sort.Float64s(all)
	p20, p50, p80, p95 := pctOf(all, 20), pctOf(all, 50), pctOf(all, 80), pctOf(all, 95)

	fmt.Println(render.DataNotice)
	fmt.Printf("일 칸 묶음 %d건의 벽시계(본줄) 분위수 : p20 %.1f · p50 %.1f · p80 %.1f · p95 %.1f 분\n\n",
		len(all), p20, p50, p80, p95)
	fmt.Println("아래를 rules.txt 에 붙여 넣습니다 (파일은 안 건드렸습니다).")
	fmt.Println()
	fmt.Printf("size\tS\t%.2f\n", p20/p50)
	fmt.Printf("size\tM\t1.00\n")
	fmt.Printf("size\tL\t%.2f\n", p80/p50)
	fmt.Printf("size\tXL\t%.2f\n", p95/p50)
	fmt.Println()
	skipped := []string{}
	for _, c := range model.AllClasses {
		xs := byClass[c]
		if len(xs) < measureMinSeed {
			if len(xs) > 0 {
				skipped = append(skipped, fmt.Sprintf("%s(%d건)", c, len(xs)))
			}
			continue
		}
		sort.Float64s(xs)
		fmt.Printf("seed\t%s\t%.0f\t%.0f\t%.0f\n", c, pctOf(xs, 50), pctOf(xs, 20), pctOf(xs, 80))
	}
	if len(skipped) > 0 {
		fmt.Printf("\n표본 %d건 미만이라 시드를 안 낸 분류 : %s — 옛 값을 그대로 둡니다.\n",
			measureMinSeed, joinOr(skipped))
	}
	return nil
}

// pctOf 는 오름차순 목록의 백분위수다. 가장 가까운 순위를 고른다.
func pctOf(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(p)/100*float64(len(sorted)-1) + 0.5)
	return sorted[i]
}
