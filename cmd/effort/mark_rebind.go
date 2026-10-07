package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/paths"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 미뤄 묶기 — start 때 「묶기없음」이 된 mark 를 stop·show·list 때 다시 찾아 묶는다.
//
// 메인 세션은 mark start 를 부른 tool_use 줄을 도구가 도는 동안(약 3초 뒤)에야 기록 파일에 쓴다.
// 그래서 start 순간엔 줄이 없어 못 묶는다. 뒤에 다시 보면 줄이 써져 있다.
// 찾으면 marks.txt 에 bind 줄을 덧붙여 다음부터는 다시 안 찾는다 — 파일 통째 읽기 값을 한 번만 치른다.
// 근거 : Docs/Research/2026-10-07-묶기없음원인조사.md · 설계 2026-09-23 「미뤄 묶기」 절
//
// 줄의 임자 : 같은 이름의 `mark start` 줄은 **줄 시각 − 5초 뒤에 처음 시작한 같은 이름 mark** 의 몫이다.
// tool_use 줄은 exe 가 시각을 찍기 전에 만들어지므로 줄은 제 mark 바로 앞에 온다. 아래 한계를 두지 않는다 —
// 권한 확인을 기다리면 줄과 mark 사이가 몇 분이 될 수 있다. 같은 이름의 앞 mark 가 그 앞 줄을 가져간다.

const (
	// rebindAfter 는 줄 시각이 mark 시각보다 늦어도 받아 주는 틈이다 (시계 어긋남 몫).
	rebindAfter = 5 * time.Second
	// rebindSettle 만큼 지나도 못 찾으면 (닫힌 mark 에 한해) 포기를 적고 다시 안 찾는다.
	rebindSettle = bindWindow
	// rebindTwin 안에 다른 파일에도 같은 임자의 줄이 있으면 어느 것인지 모른다 — 묶기모호.
	rebindTwin = 60 * time.Second
	// rebindStopAfter 는 통째 읽기를 멈추는 때다. 찾는 줄 시각(hi) 뒤로 이만큼 지난 줄이 나오면 그만 읽는다.
	rebindStopAfter = 10 * time.Minute
	// rebindBudget 은 한 번 부를 때 통째 읽기로 읽는 바이트 상한이다. 넘으면 남은 것은 다음 판으로 미룬다.
	rebindBudget = 512 << 20
)

// rebindOpts 는 다시 찾기의 허용 범위다.
type rebindOpts struct {
	settle bool // 포기 줄을 써도 되나 (stop·show 만. list 는 읽기라 안 쓴다)
	force  bool // 포기가 적힌 mark 도 다시 찾나 (show --rebind)
}

// lateLine 은 같은 이름 mark start 를 부른 줄 하나다.
type lateLine struct {
	ref fileRef
	at  time.Time
}

// needsRebind 는 다시 찾을 mark 인지다. 묶기모호(start 때 이미 여럿이 맞음)는 force 때만 본다.
func needsRebind(m *store.TimeMark, force bool) bool {
	if m.File != "" {
		return false
	}
	if force {
		return true
	}
	return m.Bind == store.BindNone && !m.Settled
}

// rebindMarks 는 want 에 맞는 안 묶인 mark 를 다시 찾아 묶고, marks 의 값도 바꿔 둔다.
// 같은 이름의 안 묶인 mark 는 한꺼번에 푼다 — 하나만 풀면 남의 줄을 제 것으로 볼 수 있다.
func rebindMarks(st *store.Store, marks []store.TimeMark, root, project string, now time.Time, want func(*store.TimeMark) bool, opt rebindOpts) {
	names := map[string]bool{}
	for i := range marks {
		if needsRebind(&marks[i], opt.force) && want(&marks[i]) {
			names[marks[i].Name] = true
		}
	}
	if len(names) == 0 {
		return
	}
	jail, err := paths.NewJail(root)
	if err != nil {
		return
	}
	budget := int64(rebindBudget)
	for name := range names {
		var group []*store.TimeMark
		for i := range marks {
			if marks[i].Name == name {
				group = append(group, &marks[i])
			}
		}
		sort.SliceStable(group, func(i, j int) bool { return group[i].Start.Before(group[j].Start) })
		var solve []*store.TimeMark
		for _, m := range group {
			if needsRebind(m, opt.force && want(m)) {
				solve = append(solve, m)
			}
		}
		lines, complete := findLateLines(jail, project, name, group, solve, &budget)
		for _, m := range solve {
			ref, status := pickLateLine(m, group, lines)
			switch {
			case status == "":
				commitBind(st, marks, m, now, ref.slug, ref.file, "")
			case opt.settle && complete && !m.Stop.IsZero() && now.Sub(m.Start) > rebindSettle:
				commitBind(st, marks, m, now, "", "", status)
			}
		}
	}
}

// commitBind 는 bind 줄을 적고 m 에 반영한다. 다른 판이 먼저 적었으면 marks.txt 를 다시 읽어 그 값을 쓴다.
func commitBind(st *store.Store, marks []store.TimeMark, m *store.TimeMark, now time.Time, slug, file, status string) {
	wrote, err := st.AppendMarkBind(m.ID, now, slug, file, status)
	if err != nil {
		fmt.Fprintf(os.Stderr, "주의 : mark %s 의 bind 줄을 못 적었습니다 : %v\n", m.ID, err)
		return
	}
	if !wrote {
		if fresh, _, err := st.LoadMarks(); err == nil {
			for _, x := range fresh {
				if x.ID == m.ID {
					m.Slug, m.File, m.Bind, m.Late, m.Settled = x.Slug, x.File, x.Bind, x.Late, x.Settled
				}
			}
		}
		return
	}
	if file != "" {
		m.Slug, m.File, m.Bind, m.Late, m.Settled = slug, file, "", true, false
		return
	}
	m.Bind, m.Settled = status, true
}

// lineOwner 는 줄 시각 at 의 임자 — at−5초 뒤에 처음 시작한 같은 이름 mark — 다. 없으면 nil.
// group 은 시작 차례로 놓여 있다.
func lineOwner(group []*store.TimeMark, at time.Time) *store.TimeMark {
	for _, m := range group {
		if !m.Start.Before(at.Add(-rebindAfter)) {
			return m
		}
	}
	return nil
}

// pickLateLine 은 m 몫의 줄 중 가장 늦은 것을 고른다. 빈 status 면 고른 것이다.
// 그 줄과 rebindTwin 안에 다른 파일의 m 몫 줄이 또 있으면 묶기모호다.
func pickLateLine(m *store.TimeMark, group []*store.TimeMark, lines []lateLine) (fileRef, string) {
	var own []lateLine
	for _, l := range lines {
		if lineOwner(group, l.at) == m {
			own = append(own, l)
		}
	}
	if len(own) == 0 {
		return fileRef{}, store.BindNone
	}
	best := own[0]
	for _, l := range own[1:] {
		if l.at.After(best.at) {
			best = l
		}
	}
	for _, l := range own {
		if l.ref != best.ref && best.at.Sub(l.at) <= rebindTwin {
			return fileRef{}, store.BindAmbiguous
		}
	}
	return best.ref, ""
}

// findLateLines 는 name 으로 mark start 를 부른 줄을 모은다. 지금 프로젝트 폴더에서 하나도 없으면 다른 폴더도 본다.
// complete 는 후보 파일을 다 열고 다 읽었는지다 — 거짓이면 포기 줄을 쓰지 않는다.
func findLateLines(jail *paths.Jail, project, name string, group, solve []*store.TimeMark, budget *int64) ([]lateLine, bool) {
	if len(solve) == 0 {
		return nil, true
	}
	lo, hi := solve[0].Start, solve[0].Start
	for _, m := range solve {
		if m.Start.Before(lo) {
			lo = m.Start
		}
		if m.Start.After(hi) {
			hi = m.Start
		}
	}
	hi = hi.Add(rebindAfter)
	// enough 는 끝 256KB 에서 모은 줄로 충분한지다 — 풀 mark 마다 제 몫 줄이 하나라도 있으면
	// 그 파일에서 그 mark 의 가장 늦은 줄은 끝쪽에 있다.
	enough := func(got []time.Time) bool {
		for _, m := range solve {
			has := false
			for _, at := range got {
				if lineOwner(group, at) == m {
					has = true
					break
				}
			}
			if !has {
				return false
			}
		}
		return true
	}
	own := ownSlug(project)
	lines, complete := scanLate(jail, []string{own}, name, lo, hi, enough, budget)
	if len(lines) > 0 {
		return lines, complete
	}
	more, c2 := scanLate(jail, otherSlugs(jail, own), name, lo, hi, enough, budget)
	return more, complete && c2
}

// scanLate 는 lo−bindWindow 뒤에 바뀌었고 첫 줄 시각이 hi 앞인 파일만 본다 — 그 사이에 살아 있던 기록만 후보다.
// 끝 256KB 를 먼저 보고, 파일이 그보다 크면 통째로 흘려 읽는다 (hi + 10분 뒤 줄에서 멈춤 · 바이트 상한).
func scanLate(jail *paths.Jail, slugs []string, name string, lo, hi time.Time, enough func([]time.Time) bool, budget *int64) ([]lateLine, bool) {
	var out []lateLine
	complete := true
	since := lo.Add(-bindWindow)
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		for _, c := range slugFiles(jail, slug, func(mod time.Time) bool { return !mod.Before(since) }) {
			got, ok := lateFileLines(jail, c, name, hi, enough, budget)
			if !ok {
				complete = false
			}
			for _, at := range got {
				out = append(out, lateLine{ref: fileRef{slug: slug, file: c.file}, at: at})
			}
		}
	}
	return out, complete
}

// lateFileLines 는 파일 하나에서 시각이 hi 이하인 name 의 mark start 줄 시각을 모은다. ok 는 다 읽었는지다.
func lateFileLines(jail *paths.Jail, c candidate, name string, hi time.Time, enough func([]time.Time) bool, budget *int64) ([]time.Time, bool) {
	f, err := jail.OpenRead(c.path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	if first, ok := collect.FirstTimestamp(f); ok && first.After(hi) {
		return nil, true
	}
	var got []time.Time
	visit := func(ts time.Time, text string) bool {
		if !ts.After(hi) && collect.MatchMarkCommand(text, "start", name) {
			got = append(got, ts)
		}
		return false
	}
	if _, _, err := collect.FindToolUseTail(f, c.size, visit); err != nil {
		return nil, false
	}
	// 끝 256KB 로 충분하면 거기서 끝낸다. 통째 읽기는 모자라고 파일이 더 클 때만.
	if c.size <= collect.LiveTailBytes || enough(got) {
		return got, true
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, false
	}
	got = nil
	complete, err := collect.ToolUseAll(f, hi.Add(rebindStopAfter), budget, func(ts time.Time, text string) { visit(ts, text) })
	return got, complete && err == nil
}

// openMainFileMarks 는 지금 프로젝트의 메인 세션 파일에 묶였고 그 파일이 bindWindow 안에 바뀐 안 닫힌 mark 들이다.
// 인자 없는 show 를 메인 세션이 불렀을 때 고르는 데 쓴다.
func openMainFileMarks(marks []store.TimeMark, root, project string, now time.Time) []*store.TimeMark {
	jail, err := paths.NewJail(root)
	if err != nil {
		return nil
	}
	own := ownSlug(project)
	var out []*store.TimeMark
	for i := range marks {
		m := &marks[i]
		if !m.Open() || m.File == "" || isAgentFile(m.File) || !strings.EqualFold(m.Slug, own) {
			continue
		}
		p, err := jail.Resolve(markFilePath(m.Slug, m.File))
		if err != nil {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if d := now.Sub(info.ModTime()); d <= bindWindow && d >= -bindWindow {
			out = append(out, m)
		}
	}
	return out
}
