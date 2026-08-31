package main

import (
	"fmt"
	"sort"

	"github.com/mirusona/efforttool/internal/classify"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/render"
)

func cmdRules(args []string) error {
	fs := newFlags("rules")
	home := fs.String("home", "", "파생 저장소 자리")
	check := fs.Bool("check", false, "문법만 본다")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	st, _, err := openStore(*home, "")
	if err != nil {
		return err
	}
	if _, err := st.EnsureRules(); err != nil {
		return fail(exitWrite, "rules.txt 를 못 만들었습니다 : %v", err)
	}
	rules, err := st.LoadRules()
	if err != nil {
		return fail(exitUsage, "%v", err)
	}
	if *check {
		fmt.Printf("rules.txt 문법 이상 없음 : %s\n", st.RulesPath())
		return nil
	}
	fmt.Printf("규칙 파일 : %s\n\n", st.RulesPath())
	printWordRules(rules)
	fmt.Println()
	printSizeRules(rules)
	fmt.Println()
	printSeedRules(rules)
	fmt.Printf("\n표본 %d건 미만이면 시드만, %d건 이상이면 실측만 씁니다.\n", rules.MinSample, rules.BlendMax)
	return nil
}

func printWordRules(r *classify.Rules) {
	byClass := map[model.Class][]string{}
	for w, c := range r.Words {
		byClass[c] = append(byClass[c], w)
	}
	agents := map[model.Class][]string{}
	for a, c := range r.Agents {
		agents[c] = append(agents[c], a)
	}
	head := []string{"분류", "낱말", "agentType"}
	rows := make([][]string, 0, len(model.AllClasses))
	for _, c := range model.AllClasses {
		w := byClass[c]
		a := agents[c]
		sort.Strings(w)
		sort.Strings(a)
		if len(w) == 0 && len(a) == 0 {
			continue
		}
		rows = append(rows, []string{string(c), joinOr(w), joinOr(a)})
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
}

func printSizeRules(r *classify.Rules) {
	names := make([]string, 0, len(r.Sizes))
	for n := range r.Sizes {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return r.Sizes[names[i]] < r.Sizes[names[j]] })
	head := []string{"크기", "배율"}
	rows := make([][]string, 0, len(names))
	for _, n := range names {
		rows = append(rows, []string{n, fmt.Sprintf("%.2f", r.Sizes[n])})
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
}

func printSeedRules(r *classify.Rules) {
	head := []string{"분류", "시드 p50", "p20", "p80", "사람눈금 배율"}
	rows := make([][]string, 0, len(model.AllClasses))
	for _, c := range model.AllClasses {
		s, ok := r.Seeds[c]
		if !ok {
			continue
		}
		rows = append(rows, []string{
			string(c),
			fmt.Sprintf("%.0f분", s.P50), fmt.Sprintf("%.0f분", s.P20), fmt.Sprintf("%.0f분", s.P80),
			fmt.Sprintf("%.2f", r.Human[c]),
		})
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
}

func joinOr(v []string) string {
	if len(v) == 0 {
		return "—"
	}
	out := v[0]
	for _, s := range v[1:] {
		out += " · " + s
	}
	return out
}
