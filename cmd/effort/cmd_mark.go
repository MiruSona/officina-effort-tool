package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/paths"
	"github.com/mirusona/officina-effort-tool/internal/render"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// effort mark — 서브에이전트가 자기 판을 잰다 (설계 2절).
// mark 는 시각만 exe 시계로 찍는다. 「실제」 값은 그 판의 transcript 에서 두 시각 사이 본줄 구간으로 잰다.
// 시간·길이를 받는 인자는 없다 — 「시간을 손으로 적어 넣는 길은 없다」를 지킨다.

const (
	// bindWindow 안에 바뀐 기록 파일만 묶기 후보로 본다. mark 를 부른 tool_use 줄이 방금 써졌기 때문이다.
	bindWindow = 2 * time.Minute
	// autoEndIdle 동안 안 바뀐 파일은 끝난 판으로 본다. stop 을 잊은 갈래를 살리는 데 쓴다.
	// 긴 도구 한 번(그림 생성 8분 등)을 끝으로 오해하지 않게 넉넉히 잡는다.
	autoEndIdle = 30 * time.Minute
	// 기록 구간과 찍은 구간이 이 비율보다 더 다르면 「어긋남」을 단다.
	driftRatio  = 0.2
	maxMarkName = 60
)

// 값 옆에 다는 표시.
const (
	flagOpen    = "진행중"
	flagAutoEnd = "끝자동"
	flagDrift   = "어긋남"
	flagOverCap = "상한넘음"
)

// nowFunc 는 exe 시계다. 시험만 바꾼다 — 사람이 시각을 넣는 길은 아니다.
var nowFunc = time.Now

// 시간·길이로 읽힐 수 있는 이름은 막는다 (15 · 1.5 · 10:30 · 15m · 20분 · 1h30m · 1시간30분).
var timeLike = regexp.MustCompile(`^([0-9]+([.:][0-9]+)*\s*(ms|s|m|h|min|분|초|시간)?\s*)+$`)

func cmdMark(args []string) error {
	if len(args) == 0 {
		return fail(exitUsage, "쓰는 법 : effort mark start \"<이름>\" | stop [<id|이름>] | show [<id|이름>] | list [--since D]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "start":
		return markStart(rest)
	case "stop":
		return markStopOrShow(rest, true)
	case "show":
		return markStopOrShow(rest, false)
	case "list":
		return markList(rest)
	case "-h", "--help", "help":
		printHelp("mark")
		return errHelpShown
	}
	return fail(exitUsage, "모르는 mark 명령 : %s (start · stop · show · list)", sub)
}

// markFlags 는 mark 의 옵션이다. 저장소·원본 자리만 받는다. 시각·길이 옵션은 일부러 없다.
type markFlags struct {
	home, projects, project string
	since                   string
}

func parseMarkFlags(args []string, withSince bool) (*markFlags, []string, error) {
	fs := newFlags("mark")
	o := &markFlags{}
	fs.StringVar(&o.home, "home", "", "파생 저장소 자리")
	fs.StringVar(&o.projects, "projects", "", "읽을 뿌리 (~/.claude/projects)")
	fs.StringVar(&o.project, "project", "", "프로젝트 폴더 이름 (기본은 지금 폴더)")
	if withSince {
		fs.StringVar(&o.since, "since", "", "이 날부터")
	}
	if err := parseFlags(fs, args); err != nil {
		return nil, nil, err
	}
	return o, fs.Args(), nil
}

func markStart(args []string) error {
	o, rest, err := parseMarkFlags(args, false)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fail(exitUsage, "쓰는 법 : effort mark start \"<이름>\" — 이름 하나만 받습니다. 시각·길이는 안 받습니다")
	}
	name := strings.TrimSpace(rest[0])
	if err := checkMarkName(name); err != nil {
		return err
	}
	st, root, err := openStore(o.home, o.projects)
	if err != nil {
		return err
	}
	// 경고만 먼저 보여 준다. id 는 겹치지 않게 store 가 락 안에서 뽑는다.
	if _, err := loadMarks(st); err != nil {
		return err
	}
	now := nowFunc()
	b := bindOwnFile(root, o.project, "start", name, now)
	m := store.TimeMark{Start: now, Slug: b.slug, File: b.file, Name: name, Bind: b.status}
	id, err := st.AppendMarkStart(m)
	if err != nil {
		return fail(exitWrite, "marks.txt 에 못 적었습니다 : %v", err)
	}
	fmt.Printf("mark %s 시작 : %s (%s)\n", id, name, now.Local().Format("2006-01-02 15:04:05"))
	switch b.status {
	case "":
		fmt.Printf("묶은 기록 : %s/%s\n", b.slug, b.file)
	case store.BindAmbiguous:
		fmt.Printf("주의 : %s — 같은 이름으로 mark start 를 부른 기록 파일이 %d개라 못 골랐습니다. 찍은 구간만 잽니다.\n", b.status, b.hits)
		fmt.Println("      이름은 판마다 다르게 짓습니다 (소단계 번호를 넣으면 겹치지 않습니다. 예 2-타일그림).")
	default:
		fmt.Printf("주의 : %s — 최근 %d분 안에 바뀐 기록 파일에서 이 mark start 를 못 찾았습니다. 찍은 구간만 잽니다.\n",
			b.status, int(bindWindow.Minutes()))
	}
	fmt.Printf("끝낼 때 : effort mark stop %s\n", id)
	return nil
}

// checkMarkName 은 이름을 본다. 시각·길이로 읽힐 이름은 손으로 시간을 적는 길이 되므로 막는다.
func checkMarkName(name string) error {
	if name == "" {
		return fail(exitUsage, "mark 이름이 비었습니다")
	}
	if utf8.RuneCountInString(name) > maxMarkName {
		return fail(exitUsage, "mark 이름은 %d자까지입니다", maxMarkName)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fail(exitUsage, "mark 이름에 탭·줄바꿈 같은 제어 글자는 못 씁니다")
		}
	}
	if strings.HasPrefix(name, "-") || timeLike.MatchString(name) {
		return fail(exitUsage, "mark 는 시각·길이를 안 받습니다 (%s). 이름만 줍니다 — 예 : effort mark start \"2-타일그림\"", name)
	}
	return nil
}

func loadMarks(st *store.Store) ([]store.TimeMark, error) {
	marks, warns, err := st.LoadMarks()
	if err != nil {
		return nil, fail(exitUsage, "%v\n%s", err, rebuildHint())
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "주의 : %s\n", w.String())
	}
	return marks, nil
}

// bindResult 는 자기 기록 파일 찾기 결과다.
type bindResult struct {
	slug, file string
	status     string // "" · 묶기없음 · 묶기모호
	hits       int
}

// bindOwnFile 은 mark 를 부른 tool_use 줄이 든 기록 파일을 찾는다 (설계 2절 「자기 파일 찾기」).
// 「가장 최근에 바뀐 파일」로 고르면 옆 갈래 파일을 잡는다 — 그래서 명령 글과 이름으로 찾는다.
func bindOwnFile(root, project, verb, name string, now time.Time) bindResult {
	hits := findCallers(root, project, verb, name, now)
	switch len(hits) {
	case 0:
		return bindResult{status: store.BindNone}
	case 1:
		return bindResult{slug: hits[0].slug, file: hits[0].file}
	}
	return bindResult{status: store.BindAmbiguous, hits: len(hits)}
}

type fileRef struct{ slug, file string }

// findCallers 는 최근에 바뀐 기록 파일 중 `mark <verb> <name>` tool_use 가 끝쪽에 있는 파일을 모은다.
// 지금 프로젝트 폴더에서 못 찾으면 전 프로젝트를 한 번 더 본다 (worktree 로 cwd 가 다를 때).
func findCallers(root, project, verb, name string, now time.Time) []fileRef {
	jail, err := paths.NewJail(root)
	if err != nil {
		return nil
	}
	own := project
	if own == "" {
		if cwd, err := os.Getwd(); err == nil {
			own = paths.Slug(cwd)
		}
	}
	match := func(t string) bool { return collect.MatchMarkCommand(t, verb, name) }
	hits := scanCallers(jail, []string{own}, now, match)
	if len(hits) > 0 {
		return hits
	}
	entries, err := os.ReadDir(jail.Root())
	if err != nil {
		return nil
	}
	var others []string
	for _, e := range entries {
		if e.IsDir() && !strings.EqualFold(e.Name(), own) {
			others = append(others, e.Name())
		}
	}
	return scanCallers(jail, others, now, match)
}

func scanCallers(jail *paths.Jail, slugs []string, now time.Time, match func(string) bool) []fileRef {
	var out []fileRef
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		for _, c := range recentFiles(jail, slug, now) {
			f, err := jail.OpenRead(c.path)
			if err != nil {
				continue
			}
			_, ok, _ := collect.FindToolUse(f, c.size, match)
			f.Close()
			if ok {
				out = append(out, fileRef{slug: slug, file: c.file})
			}
		}
	}
	return out
}

type candidate struct {
	path string
	file string
	size int64
}

// recentFiles 는 프로젝트 폴더의 세션 파일과 서브에이전트 파일 중 bindWindow 안에 바뀐 것을 준다.
func recentFiles(jail *paths.Jail, slug string, now time.Time) []candidate {
	dir, err := jail.Resolve(slug)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	fresh := func(p string) (int64, bool) {
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			return 0, false
		}
		d := now.Sub(info.ModTime())
		return info.Size(), d <= bindWindow && d >= -bindWindow
	}
	var out []candidate
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if !e.IsDir() {
			if !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			if size, ok := fresh(p); ok {
				out = append(out, candidate{path: p, file: strings.TrimSuffix(e.Name(), ".jsonl"), size: size})
			}
			continue
		}
		subDir, err := jail.Resolve(filepath.Join(p, "subagents"))
		if err != nil {
			continue
		}
		subs, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, s := range subs {
			if s.IsDir() || !strings.HasPrefix(s.Name(), "agent-") || !strings.HasSuffix(s.Name(), ".jsonl") {
				continue
			}
			sp := filepath.Join(subDir, s.Name())
			if size, ok := fresh(sp); ok {
				out = append(out, candidate{path: sp, file: e.Name() + "/" + strings.TrimSuffix(s.Name(), ".jsonl"), size: size})
			}
		}
	}
	return out
}

// markFilePath 는 marks.txt 의 파일 칸을 원본 뿌리 안 경로로 바꾼다.
func markFilePath(slug, file string) string {
	if sid, agent, ok := strings.Cut(file, "/"); ok {
		return filepath.Join(slug, sid, "subagents", agent+".jsonl")
	}
	return filepath.Join(slug, file+".jsonl")
}

func markStopOrShow(args []string, stop bool) error {
	o, rest, err := parseMarkFlags(args, false)
	if err != nil {
		return err
	}
	verb := "show"
	if stop {
		verb = "stop"
	}
	if len(rest) > 1 {
		return fail(exitUsage, "쓰는 법 : effort mark %s [<mark id|이름>] — 하나만 받습니다", verb)
	}
	want := ""
	if len(rest) == 1 {
		want = strings.TrimSpace(rest[0])
		if strings.HasPrefix(want, "-") || timeLike.MatchString(want) {
			return fail(exitUsage, "mark 는 시각·길이를 안 받습니다 (%s). mark id 나 이름을 줍니다", want)
		}
	}
	st, root, err := openStore(o.home, o.projects)
	if err != nil {
		return err
	}
	marks, err := loadMarks(st)
	if err != nil {
		return err
	}
	now := nowFunc()
	m, err := pickMark(marks, want, stop, root, o.project, verb, now)
	if err != nil {
		return err
	}
	if stop {
		if !m.Open() {
			return fail(exitUsage, "이미 닫힌 mark 입니다 : %s (effort mark show %s 로 보세요)", m.ID, m.ID)
		}
		if err := st.AppendMarkStop(m.ID, now); err != nil {
			return fail(exitWrite, "marks.txt 에 못 적었습니다 : %v", err)
		}
		m.Stop = now
	}
	v := measureMark(m, root, markSubMax(st), now)
	printMarkValue(m, v)
	return nil
}

// pickMark 는 인자(id·이름)나 부른 기록 파일로 mark 하나를 고른다.
// 인자를 빼면 이 transcript 에 묶인 안 닫힌 mark 하나를 고른다. 못 고르면 목록을 보여 주고 사용법 오류다.
func pickMark(marks []store.TimeMark, want string, openOnly bool, root, project, verb string, now time.Time) (*store.TimeMark, error) {
	if want != "" {
		for i := range marks {
			if marks[i].ID == want {
				return &marks[i], nil
			}
		}
		var best *store.TimeMark
		for i := range marks {
			m := &marks[i]
			if m.Name != want || (openOnly && !m.Open()) {
				continue
			}
			if best == nil || m.Start.After(best.Start) {
				best = m
			}
		}
		if best == nil {
			return nil, fail(exitNoData, "그런 mark 가 없습니다 : %s (effort mark list 로 보세요)", want)
		}
		return best, nil
	}
	callers := map[fileRef]bool{}
	for _, c := range findCallers(root, project, verb, "", now) {
		callers[c] = true
	}
	var hits []*store.TimeMark
	for i := range marks {
		m := &marks[i]
		if m.Open() && m.File != "" && callers[fileRef{slug: m.Slug, file: m.File}] {
			hits = append(hits, m)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	var open []string
	for i := range marks {
		if marks[i].Open() {
			open = append(open, marks[i].ID+" "+marks[i].Name)
		}
	}
	if len(open) == 0 {
		return nil, fail(exitNoData, "안 닫힌 mark 가 없습니다. id 나 이름을 주면 닫힌 것도 봅니다")
	}
	return nil, fail(exitUsage, "어느 mark 인지 못 골랐습니다. id 나 이름을 줍니다 : %s", strings.Join(open, " · "))
}

// markValue 는 mark 한 판을 잰 값이다.
type markValue struct {
	recordMs int64 // 기록 구간. 못 쟀으면 -1
	markedMs int64 // 찍은 구간
	end      time.Time
	flags    []string
}

// measureMark 는 묶인 파일의 본줄로 기록 구간을 재고, exe 시계로 찍은 구간을 잰다.
func measureMark(m *store.TimeMark, root string, subMax int, now time.Time) markValue {
	v := markValue{recordMs: -1}
	if m.File == "" {
		if m.Bind != "" {
			v.flags = append(v.flags, m.Bind)
		}
	} else {
		v = measureBound(m, root, now)
	}
	end := m.Stop
	if end.IsZero() {
		end = v.end
	}
	if end.IsZero() {
		end = now
		if !hasFlag(v.flags, flagOpen) {
			v.flags = append([]string{flagOpen}, v.flags...)
		}
	}
	v.end = end
	v.markedMs = end.Sub(m.Start).Milliseconds()
	if v.recordMs >= 0 && v.markedMs > 0 {
		diff := float64(v.markedMs - v.recordMs)
		if diff < 0 {
			diff = -diff
		}
		if diff/float64(v.markedMs) > driftRatio {
			v.flags = append(v.flags, flagDrift)
		}
	}
	// 상한을 넘어도 자르지 않는다 — 한 판의 실제 값이다. 표시만 단다 (설계 U4).
	if v.recordMs > int64(subMax)*60*1000 {
		v.flags = append(v.flags, flagOverCap)
	}
	return v
}

// measureBound 는 묶인 파일을 원본 뿌리 감옥을 거쳐 열고 본줄 구간을 잰다.
func measureBound(m *store.TimeMark, root string, now time.Time) markValue {
	v := markValue{recordMs: -1}
	jail, err := paths.NewJail(root)
	if err != nil {
		v.flags = append(v.flags, "파일못엶")
		return v
	}
	f, err := jail.OpenRead(markFilePath(m.Slug, m.File))
	if err != nil {
		v.flags = append(v.flags, "파일못엶")
		return v
	}
	defer f.Close()
	to := m.Stop
	autoEnd := false
	if m.Open() {
		to = now
		if info, err := f.Stat(); err == nil && now.Sub(info.ModTime()) > autoEndIdle {
			autoEnd = true
			to = time.Time{}
		}
	}
	span, err := collect.MainSpan(f, m.Start, to)
	if err != nil {
		v.flags = append(v.flags, "파일못읽음")
		return v
	}
	v.recordMs = span.Ms()
	if m.Open() {
		if autoEnd {
			v.flags = append(v.flags, flagAutoEnd)
			v.end = span.LastMain
			if v.end.Before(m.Start) {
				v.end = m.Start
			}
		} else {
			v.flags = append(v.flags, flagOpen)
		}
	}
	return v
}

func hasFlag(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

// markSubMax 는 rules.txt 의 sub max 다. 못 읽으면 기본 규칙 값을 쓴다.
func markSubMax(st *store.Store) int {
	if r, _, err := st.LoadRules(); err == nil {
		return r.SubMax
	}
	if r, err := classify.ParseRules(strings.NewReader(classify.DefaultRulesText)); err == nil {
		return r.SubMax
	}
	return 120
}

func spanText(ms int64) string {
	if ms < 0 {
		return "—"
	}
	if ms == 0 {
		return "1분 미만"
	}
	return render.Minutes(ms)
}

func printMarkValue(m *store.TimeMark, v markValue) {
	fmt.Printf("mark %s · %s\n", m.ID, m.Name)
	endText := v.end.Local().Format("2006-01-02 15:04:05")
	switch {
	case hasFlag(v.flags, flagOpen):
		endText += " (지금 — 진행중)"
	case hasFlag(v.flags, flagAutoEnd):
		endText += " (stop 없음 — 파일의 마지막 본줄)"
	}
	fmt.Printf("시작 : %s\n", m.Start.Local().Format("2006-01-02 15:04:05 -07:00"))
	fmt.Printf("끝   : %s\n", endText)
	if m.File != "" {
		fmt.Printf("묶은 기록 : %s/%s\n", m.Slug, m.File)
	}
	fmt.Printf("기록 구간 : %s   ← 공수 표 「실제」 칸에 쓰는 값\n", spanText(v.recordMs))
	fmt.Printf("찍은 구간 : %s   (exe 시계 · 참고)\n", spanText(v.markedMs))
	if len(v.flags) > 0 {
		fmt.Printf("표시 : %s\n", strings.Join(v.flags, " · "))
	}
	if v.recordMs < 0 {
		fmt.Println("기록 구간을 못 쟀습니다. 「실제」 칸에는 지어내지 말고 「못 쟀다」고 적습니다.")
	}
}

func markList(args []string) error {
	o, rest, err := parseMarkFlags(args, true)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fail(exitUsage, "mark list 는 인자를 안 받습니다 : %s", rest[0])
	}
	var cut time.Time
	if o.since != "" {
		t, err := time.ParseInLocation("2006-01-02", o.since, time.Local)
		if err != nil {
			return fail(exitUsage, "--since 는 YYYY-MM-DD 꼴입니다 : %s", o.since)
		}
		cut = t
	}
	st, root, err := openStore(o.home, o.projects)
	if err != nil {
		return err
	}
	marks, err := loadMarks(st)
	if err != nil {
		return err
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].Start.Before(marks[j].Start) })
	now := nowFunc()
	subMax := markSubMax(st)
	head := []string{"mark", "이름", "시작", "기록 구간", "찍은 구간", "표시"}
	var rows [][]string
	for i := range marks {
		m := &marks[i]
		if !cut.IsZero() && m.Start.Before(cut) {
			continue
		}
		v := measureMark(m, root, subMax, now)
		rows = append(rows, []string{m.ID, m.Name, m.Start.Local().Format("01-02 15:04"),
			spanText(v.recordMs), spanText(v.markedMs), strings.Join(v.flags, " · ")})
	}
	if len(rows) == 0 {
		return fail(exitNoData, "찍은 mark 가 없습니다 (%s)", st.MarksPath())
	}
	fmt.Print(render.Table(head, rows, render.Pipe))
	return nil
}
