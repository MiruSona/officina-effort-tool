package main

import (
	"fmt"
	"sort"

	"github.com/mirusona/efforttool/internal/collect"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/render"
	"github.com/mirusona/efforttool/internal/store"
)

type bucket struct {
	Key    string `json:"key"`
	Count  int    `json:"count"`
	WallMs int64  `json:"wall_ms"`
	PureMs int64  `json:"pure_ms"`
	Tokens int64  `json:"tokens"`
}

func cmdStats(args []string) error {
	fs := newFlags("stats")
	home := fs.String("home", "", "파생 저장소 자리")
	class := fs.String("class", "", "분류 하나만")
	since := fs.String("since", "", "이 날부터 (YYYY-MM-DD)")
	by := fs.String("by", "class", "class|agent|model|session")
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
	tasks, dropped, err := filterTasks(tasks, *class, *since, "", *all)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fail(exitNoData, "고른 조건에 맞는 작업이 0건입니다.")
	}
	buckets, err := groupBy(tasks, *by)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(buckets)
	}
	printStatsHead(tasks, dropped)
	printCoverWarn(st)
	head := []string{"칸", "건수", "벽시계", "순수시간", "토큰", "ms/1K tok"}
	rows := make([][]string, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, []string{
			b.Key, fmt.Sprintf("%d", b.Count),
			render.Minutes(b.WallMs), render.Minutes(b.PureMs),
			render.Tokens(b.Tokens), render.MsPerKTok(b.PureMs, b.Tokens),
		})
	}
	style := render.Pipe
	if *wide {
		style = render.Wide
	}
	fmt.Print(render.Table(head, rows, style))
	return nil
}

func printStatsHead(tasks []model.Task, dropped int) {
	noSub, unknown := 0, 0
	for _, t := range tasks {
		if t.HasWarn(collect.WarnNoSubagent) {
			noSub++
		}
		if t.Class == model.ClassUnknown {
			unknown++
		}
	}
	n := float64(len(tasks))
	fmt.Printf("작업 %d건 · 미분류 %.0f%% · 서브기록없음 %.0f%%",
		len(tasks), float64(unknown)/n*100, float64(noSub)/n*100)
	if dropped > 0 {
		fmt.Printf(" · 일 아닌 것 %d건 뺌(--all 로 봄)", dropped)
	}
	fmt.Print("\n\n")
}

// printCoverWarn 은 쪼갠 합이 세션 총계보다 작을 때 알린다. 배분은 하지 않는다.
func printCoverWarn(st *store.Store) {
	sessions, err := st.ReadSessions()
	if err != nil {
		return
	}
	var sum, total int64
	low, counted := 0, 0
	for _, s := range sessions {
		if s.InProgress {
			continue
		}
		counted++
		sum += s.SumUsage.Sum().Total()
		total += s.StateUsage.Sum().Total()
		if s.CoverPct > 0 && s.CoverPct < 60 {
			low++
		}
	}
	if total == 0 {
		return
	}
	pct := float64(sum) / float64(total) * 100
	fmt.Printf("cost-state 가 있는 세션 %d개 기준 : 작업 합 %s / 세션 총계 %s (%.0f%%)\n",
		counted, render.Tokens(sum), render.Tokens(total), pct)
	if low > 0 {
		fmt.Printf("⚠ 쪼갠 합이 세션 총계보다 작은 세션 %d개 (까닭 미확정)\n", low)
	}
	fmt.Println()
}

func groupBy(tasks []model.Task, by string) ([]bucket, error) {
	m := map[string]*bucket{}
	add := func(key string, wall, pure, tok int64) {
		b := m[key]
		if b == nil {
			b = &bucket{Key: key}
			m[key] = b
		}
		b.Count++
		b.WallMs += wall
		b.PureMs += pure
		b.Tokens += tok
	}
	switch by {
	case "class":
		for _, t := range tasks {
			add(string(t.Class), t.WallMs, t.PureMs, t.Usage.Sum().Total())
		}
	case "session":
		for _, t := range tasks {
			add(t.SessionID, t.WallMs, t.PureMs, t.Usage.Sum().Total())
		}
	case "agent":
		for _, t := range tasks {
			for _, a := range t.Agents {
				add(a.AgentType, a.End.Sub(a.Start).Milliseconds(), 0, a.Usage.Sum().Total())
			}
		}
	case "model":
		for _, t := range tasks {
			for name, u := range t.Usage {
				add(name, 0, 0, u.Total())
			}
		}
	default:
		return nil, fail(exitUsage, "--by 는 class|agent|model|session 중 하나입니다 : %s", by)
	}
	out := make([]bucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if len(out) == 0 {
		return nil, fail(exitNoData, "묶을 것이 없습니다.")
	}
	return out, nil
}
