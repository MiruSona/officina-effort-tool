package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/paths"
	"github.com/mirusona/officina-effort-tool/internal/secret"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// 손상 줄이 이 비율을 넘으면 종료 5.
const badRatioLimit = 0.02

type scanTotals struct {
	files    int
	skipped  int
	tasks    int
	sessions int
	rollback int
	bad      int
	tooLong  int
	lines    int
	// 본줄도 아는 곁줄도 아닌 줄 종류 → 건수. 벽시계에서 뺀 것을 사람이 볼 수 있어야 한다.
	unknown map[string]int
}

func cmdScan(args []string) error {
	fs := newFlags("scan")
	home := fs.String("home", "", "파생 저장소 자리")
	projects := fs.String("projects", "", "읽을 뿌리 (~/.claude/projects)")
	project := fs.String("project", "", "프로젝트 폴더 이름 하나")
	all := fs.Bool("all", false, "모든 프로젝트")
	rebuild := fs.Bool("rebuild", false, "통째로 다시 읽기")
	titles := fs.Bool("titles", true, "제목을 캐시에 남긴다")
	quiet := fs.Bool("quiet", false, "끝 줄만 찍는다")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fail(exitUsage, "scan 은 인자를 안 받습니다 : %s", fs.Arg(0))
	}

	st, root, err := openStore(*home, *projects)
	if err != nil {
		return err
	}
	if err := st.EnsureDirs(); err != nil {
		return fail(exitWrite, "캐시 폴더를 못 만들었습니다 : %v", err)
	}
	created, err := st.EnsureRules()
	if err != nil {
		return fail(exitWrite, "rules.txt 를 못 만들었습니다 : %v", err)
	}
	if created && !*quiet {
		fmt.Printf("rules.txt 를 처음 만들었습니다 : %s\n", st.RulesPath())
	}
	rules, err := st.LoadRules()
	if err != nil {
		return fail(exitUsage, "%v", err)
	}
	var addedKinds []string
	if missing := rules.MissingKinds(); len(missing) > 0 {
		addedKinds, err = st.AppendMissingKinds(missing)
		if err != nil {
			return fail(exitWrite, "rules.txt 에 기본값을 못 더했습니다 : %v", err)
		}
		if len(addedKinds) > 0 {
			rules, err = st.LoadRules()
			if err != nil {
				return fail(exitUsage, "%v", err)
			}
			if !*quiet {
				fmt.Printf("rules.txt 에 %s 기본값을 더했습니다 : %s\n",
					strings.Join(addedKinds, " · "), st.RulesPath())
			}
		}
	}
	printMissingKinds(rules, st.RulesPath())

	dirs, err := pickProjectDirs(root, *project, *all)
	if err != nil {
		return err
	}

	lock, err := store.AcquireLock(st.Home())
	if err != nil {
		return fail(exitWrite, "%v", err)
	}
	defer lock.Release()

	state := st.LoadScanState()
	// 판이 다르면 옛 작업에 새 칸이 없다. 섞이면 분류가 조용히 틀리므로 rebuild 와 똑같이 다룬다.
	// 규칙을 더했을 때도 캐시에 남은 옛 분류를 새 규칙으로 다시 매긴다.
	full := *rebuild || state.Stale() || len(addedKinds) > 0
	if state.Stale() {
		fmt.Printf("캐시 판이 %s → %s 로 바뀌어 전부 다시 읽습니다.\n", oldSchemaName(state.Schema), store.SchemaVersion)
	}
	if full {
		state = store.NewScanState()
	}
	oldTasks, oldSessions := loadOld(st, full)

	tasksBySession := groupTasks(oldTasks)
	sessionByID := map[string]model.Session{}
	for _, s := range oldSessions {
		sessionByID[s.SessionID] = s
	}

	tot := scanTotals{unknown: map[string]int{}}
	for _, dir := range dirs {
		files, err := sessionFiles(root, dir)
		if err != nil {
			return err
		}
		for _, f := range files {
			changed, err := scanOne(f, state, rules, *titles, tasksBySession, sessionByID, &tot)
			if err != nil {
				return err
			}
			if !changed {
				tot.skipped++
			}
		}
	}

	tasks, sessions := flatten(tasksBySession, sessionByID)
	if err := st.WriteTasks(tasks); err != nil {
		return fail(exitWrite, "작업 캐시를 못 썼습니다 : %v", err)
	}
	if err := st.WriteSessions(sessions); err != nil {
		return fail(exitWrite, "세션 캐시를 못 썼습니다 : %v", err)
	}
	if err := st.SaveScanState(state); err != nil {
		return fail(exitWrite, "스캔 기록을 못 썼습니다 : %v", err)
	}
	tot.tasks = len(tasks)
	tot.sessions = len(sessions)
	printScanSummary(tot, st.Home())
	printClassCounts(tasks)
	if tot.lines > 0 && float64(tot.bad)/float64(tot.lines) > badRatioLimit {
		return fail(exitCorrupt, "손상 줄이 %.1f%% 로 상한 2%% 를 넘었습니다", float64(tot.bad)/float64(tot.lines)*100)
	}
	return nil
}

// printMissingKinds 는 rules.txt 에 새 규칙 종류가 없을 때 알린다. 파일은 사람 정본이라 안 덮는다.
func printMissingKinds(rules *classify.Rules, path string) {
	missing := rules.MissingKinds()
	if len(missing) == 0 {
		return
	}
	fmt.Printf("주의 : rules.txt 에 %s 줄이 없어 그 규칙이 꺼져 있습니다 (%s).\n",
		strings.Join(missing, " · "), path)
	fmt.Println("      scan 을 돌리면 빠진 종류의 기본값을 파일 끝에 더합니다.")
}

func oldSchemaName(s string) string {
	if s == "" {
		return "없음"
	}
	return s
}

func loadOld(st *store.Store, rebuild bool) ([]model.Task, []model.Session) {
	if rebuild {
		return nil, nil
	}
	tasks, err := st.ReadTasks()
	if err != nil {
		tasks = nil
	}
	sessions, err := st.ReadSessions()
	if err != nil {
		sessions = nil
	}
	return tasks, sessions
}

func groupTasks(tasks []model.Task) map[string][]model.Task {
	out := map[string][]model.Task{}
	for _, t := range tasks {
		out[t.SessionID] = append(out[t.SessionID], t)
	}
	return out
}

func flatten(byS map[string][]model.Task, sessions map[string]model.Session) ([]model.Task, []model.Session) {
	ids := make([]string, 0, len(sessions))
	for id := range sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var ts []model.Task
	var ss []model.Session
	for _, id := range ids {
		ts = append(ts, byS[id]...)
		ss = append(ss, sessions[id])
	}
	return ts, ss
}

// scanOne 은 세션 파일 하나를 본다. 안 바뀌었으면 건너뛴다.
func scanOne(path string, state *store.ScanState, rules *classify.Rules, keepTitles bool,
	tasks map[string][]model.Task, sessions map[string]model.Session, tot *scanTotals) (bool, error) {

	info, err := os.Stat(path)
	if err != nil {
		return false, fail(exitRead, "파일을 못 봤습니다 : %v", err)
	}
	key := path
	if state.Unchanged(key, info.Size(), info.ModTime().UnixMilli()) {
		return false, nil
	}
	res, err := collect.ReadSession(path)
	if err != nil {
		return false, fail(exitRead, "세션을 못 읽었습니다 (%s) : %v", filepath.Base(path), err)
	}
	// 제목은 분류에 쓰이므로 분류를 먼저 하고 그다음에 지운다.
	rules.ClassifySession(res.Tasks)
	if !keepTitles {
		for i := range res.Tasks {
			res.Tasks[i].Title = ""
		}
	}
	tasks[res.Session.SessionID] = res.Tasks
	sessions[res.Session.SessionID] = res.Session
	state.Files[key] = store.FileState{Size: info.Size(), ModTime: info.ModTime().UnixMilli(), Offset: info.Size()}
	tot.files++
	tot.rollback += res.Rollback
	tot.bad += res.Bad
	tot.tooLong += res.TooLong
	tot.lines += res.Total
	for name, n := range res.Unknown {
		tot.unknown[name] += n
	}
	return true, nil
}

// pickProjectDirs 는 훑을 프로젝트 폴더를 고른다.
func pickProjectDirs(root, project string, all bool) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fail(exitRead, "원본 폴더를 못 읽었습니다 (%s) : %v", root, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if all {
		return names, nil
	}
	want := project
	if want == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fail(exitRead, "지금 폴더를 못 봤습니다 : %v", err)
		}
		want = paths.Slug(cwd)
	}
	for _, n := range names {
		if strings.EqualFold(n, want) {
			return []string{n}, nil
		}
	}
	return nil, fail(exitNoData, "그런 프로젝트 폴더가 없습니다 : %s (--all 로 전부 훑을 수 있습니다)", want)
}

// sessionFiles 는 프로젝트 폴더 안의 세션 JSONL 을 모은다.
func sessionFiles(root, dir string) ([]string, error) {
	full := filepath.Join(root, dir)
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, fail(exitRead, "폴더를 못 읽었습니다 (%s) : %v", full, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, filepath.Join(full, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// printClassCounts 는 분류 결과를 한 줄로 찍는다. 규칙을 손봤을 때 어디서 갈렸는지 볼 유일한 자리다.
func printClassCounts(tasks []model.Task) {
	byClass := map[model.Class]int{}
	byRule := map[string]int{}
	for i := range tasks {
		byClass[tasks[i].Class]++
		byRule[tasks[i].ClassBy]++
	}
	var parts []string
	for _, c := range model.AllClasses {
		if byClass[c] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", c, byClass[c]))
	}
	fmt.Printf("분류 : %s\n", strings.Join(parts, " · "))

	var rules []string
	for _, by := range []string{classify.ByInherit, classify.ByMatch, classify.ByContinue} {
		rules = append(rules, fmt.Sprintf("%s %d", by, byRule[by]))
	}
	fmt.Printf("물려받음 : %s\n", strings.Join(rules, " · "))
}

func printScanSummary(t scanTotals, home string) {
	fmt.Printf("읽은 세션 %d개 (안 바뀜 %d개) · 작업 %d건 · 줄 %d개\n", t.files, t.skipped, t.tasks, t.lines)
	fmt.Printf("캐시 : %s\n", filepath.Join(home, "cache"))
	if t.rollback > 0 {
		fmt.Printf("주의 : output 토큰이 줄어든 자리 %d곳 (단조 증가 가정이 깨졌습니다)\n", t.rollback)
	}
	if t.bad > 0 {
		fmt.Printf("주의 : 못 읽은 줄 %d개 (그 중 너무 긴 줄 %d개)\n", t.bad, t.tooLong)
	}
	printUnknownLines(t.unknown)
}

// printUnknownLines 는 본줄도 아는 곁줄도 아닌 줄 종류를 알린다.
// 클로드 코드가 새 줄 종류를 더했을 때 벽시계가 조용히 어긋나는 것을 여기서 잡는다.
func printUnknownLines(unknown map[string]int) {
	if len(unknown) == 0 {
		return
	}
	names := make([]string, 0, len(unknown))
	for name := range unknown {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s %d개", secret.Sanitize(name), unknown[name]))
	}
	fmt.Printf("주의 : 모르는 줄 종류 %d가지 (%s) — 벽시계에서 뺐습니다.\n",
		len(names), strings.Join(parts, " · "))
}
