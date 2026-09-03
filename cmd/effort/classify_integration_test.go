package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

const testdataNotify = "../../testdata/notify"
const testdataOldRules = "../../testdata/oldrules"

// scanNotify 는 알림 세션 하나를 훑는다.
func scanNotify(t *testing.T, extra ...string) (string, []model.Task) {
	t.Helper()
	home := t.TempDir()
	args := append([]string{"scan", "--home", home, "--projects", testdataNotify, "--all"}, extra...)
	if code, out := capture(t, args...); code != exitOK {
		t.Fatalf("scan 종료 코드 %d\n%s", code, out)
	}
	tasks, err := store.New(home).ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	return home, tasks
}

func TestScanInheritsClassForNotifications(t *testing.T) {
	_, tasks := scanNotify(t)
	for _, id := range []string{"p-notify-0002", "p-notify-0003"} {
		task := findTask(tasks, id)
		if task == nil {
			t.Fatalf("%s 를 못 찾았다", id)
		}
		if task.Class != model.ClassDoc || task.ClassBy != classify.ByInherit {
			t.Fatalf("%s 분류 = %s (%s)", id, task.Class, task.ClassBy)
		}
	}
	chat := findTask(tasks, "p-notify-0004")
	if chat == nil || chat.Class != model.ClassChat {
		t.Fatalf("대화 작업 분류 = %+v", chat)
	}
	last := findTask(tasks, "p-notify-0005")
	if last == nil || last.Class != model.ClassDoc {
		t.Fatalf("마지막 알림 분류 = %+v", last)
	}
}

func TestScanToolsExcludeSubagent(t *testing.T) {
	_, tasks := scanNotify(t)
	task := findTask(tasks, "p-notify-0001")
	if task == nil {
		t.Fatal("작업을 못 찾았다")
	}
	if task.Tools["Write"] != 1 || task.Tools["Bash"] != 1 {
		t.Fatalf("메인 도구를 잘못 셌다 : %v", task.Tools)
	}
	if task.Tools["NotebookEdit"] != 0 {
		t.Fatalf("서브에이전트 도구가 섞였다 : %v", task.Tools)
	}
}

func TestScanTitlesFalseStillClassifies(t *testing.T) {
	_, tasks := scanNotify(t, "--titles=false")
	task := findTask(tasks, "p-notify-0004")
	if task == nil {
		t.Fatal("작업을 못 찾았다")
	}
	if task.Title != "" {
		t.Fatalf("제목을 안 지웠다 : %q", task.Title)
	}
	if task.Class != model.ClassChat {
		t.Fatalf("제목을 지웠다고 분류가 죽었다 : %s", task.Class)
	}
}

func TestSchemaBumpForcesFullRebuild(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataNotify, "--all")
	// 옛 판 캐시를 흉내낸다 : Tools 칸이 없던 시절.
	p := filepath.Join(home, "cache", "scanstate.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(raw), `"schema":"`+store.SchemaVersion+`"`, `"schema":"1"`, 1)
	if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(home, "cache", "tasks.jsonl")
	rawTasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.ReplaceAll(string(rawTasks), `"tools":`, `"tools_old":`)
	if err := os.WriteFile(tasksPath, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := capture(t, "scan", "--home", home, "--projects", testdataNotify, "--all")
	if code != exitOK {
		t.Fatalf("scan %d\n%s", code, out)
	}
	if !strings.Contains(out, "캐시 판이 1 → "+store.SchemaVersion) {
		t.Fatalf("판이 바뀐 것을 안 알렸다 :\n%s", out)
	}
	tasks, _ := store.New(home).ReadTasks()
	task := findTask(tasks, "p-notify-0001")
	if task == nil || len(task.Tools) == 0 {
		t.Fatalf("옛 판 작업이 그대로 남았다 : %+v", task)
	}
}

func TestScanIncrementalKeepsInherit(t *testing.T) {
	root := t.TempDir()
	copyTree(t, testdataNotify, root)
	home := t.TempDir()
	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("첫 scan %d\n%s", code, out)
	}
	path := filepath.Join(root, "proj", "sess-notify0001.jsonl")
	tail := `{"parentUuid":"a-notify-0005","isSidechain":false,"promptId":"p-notify-0006","type":"user","message":{"role":"user","content":"<task-notification>넷째가 끝났습니다</task-notification>"},"uuid":"u-notify-0006","timestamp":"2026-08-30T11:05:00.000Z","origin":{"kind":"task-notification"},"cwd":"C:\\proj","sessionId":"sess-notify0001","version":"2.1.238"}` + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(tail)
	f.Close()

	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("두번째 scan %d\n%s", code, out)
	}
	tasks, _ := store.New(home).ReadTasks()
	task := findTask(tasks, "p-notify-0006")
	if task == nil || task.Class != model.ClassDoc || task.ClassBy != classify.ByInherit {
		t.Fatalf("덧붙인 알림 분류 = %+v", task)
	}
}

// 옛 rules.txt 를 쓰면 scan 이 빠진 종류의 기본값을 끝에 더하고 새 규칙으로 다시 분류한다.
func TestScanFillsOldRulesFile(t *testing.T) {
	home := t.TempDir()
	oldText, err := os.ReadFile(filepath.FromSlash("../../testdata/rules/old.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(home, "rules.txt")
	if err := os.WriteFile(rulesPath, oldText, 0o644); err != nil {
		t.Fatal(err)
	}
	// 옛 규칙만으로는 두 건 다 미분류다.
	if n := unknownWithRules(t, string(oldText)); n != 2 {
		t.Fatalf("옛 규칙 미분류 = %d건 (2건이어야 표본이 된다)", n)
	}

	code, out := capture(t, "scan", "--home", home, "--projects", testdataOldRules, "--all")
	if code != exitOK {
		t.Fatalf("scan %d\n%s", code, out)
	}
	if !strings.Contains(out, "기본값을 더했습니다") {
		t.Fatalf("더했다고 안 알렸다 :\n%s", out)
	}
	if strings.Contains(out, "규칙이 꺼져 있습니다") {
		t.Fatalf("더하고도 경고가 남았다 :\n%s", out)
	}
	tasks, err := store.New(home).ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.Class == model.ClassUnknown {
			t.Fatalf("새 규칙인데 아직 미분류다 : %s %q", task.PromptID, task.Title)
		}
	}
	raw, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), string(oldText)) {
		t.Fatal("사람이 쓴 줄을 건드렸다")
	}

	// 두 번째 scan 은 더할 것이 없다.
	code, out = capture(t, "scan", "--home", home, "--projects", testdataOldRules, "--all")
	if code != exitOK {
		t.Fatalf("두번째 scan %d\n%s", code, out)
	}
	if strings.Contains(out, "기본값을 더했습니다") {
		t.Fatalf("다 있는데 또 더했다 :\n%s", out)
	}
	again, _ := os.ReadFile(rulesPath)
	if string(raw) != string(again) {
		t.Fatal("두 번 돌렸더니 rules.txt 가 달라졌다")
	}
}

// rules --check 는 읽기 명령이라 파일을 고치지 않고 알리기만 한다.
func TestRulesCheckDoesNotAppend(t *testing.T) {
	home := t.TempDir()
	oldText, err := os.ReadFile(filepath.FromSlash("../../testdata/rules/old.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(home, "rules.txt")
	if err := os.WriteFile(rulesPath, oldText, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "rules", "--home", home, "--check")
	if code != exitOK {
		t.Fatalf("rules --check %d\n%s", code, out)
	}
	if !strings.Contains(out, "규칙이 꺼져 있습니다") {
		t.Fatalf("빠진 종류를 안 알렸다 :\n%s", out)
	}
	raw, _ := os.ReadFile(rulesPath)
	if string(raw) != string(oldText) {
		t.Fatal("읽기 명령이 rules.txt 를 고쳤다")
	}
}

// unknownWithRules 는 주어진 규칙으로 시험 세션을 분류해 미분류 건수를 센다.
func unknownWithRules(t *testing.T, text string) int {
	t.Helper()
	rules, err := classify.ParseRules(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	res, err := collect.ReadSession(filepath.FromSlash(testdataOldRules + "/proj/sess-oldrules0001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	rules.ClassifySession(res.Tasks)
	n := 0
	for _, task := range res.Tasks {
		if task.Class == model.ClassUnknown {
			n++
		}
	}
	return n
}

func TestStatsHidesNonWorkByDefault(t *testing.T) {
	home, _ := scanNotify(t)
	code, out := capture(t, "stats", "--home", home, "--by", "class")
	if code != exitOK {
		t.Fatalf("stats %d\n%s", code, out)
	}
	if strings.Contains(out, string(model.ClassChat)) {
		t.Fatalf("기본 표에 대화가 보인다 :\n%s", out)
	}
	if !strings.Contains(out, "일 아닌 것") {
		t.Fatalf("뺐다는 안내가 없다 :\n%s", out)
	}
	code, out = capture(t, "stats", "--home", home, "--by", "class", "--all")
	if code != exitOK || !strings.Contains(out, string(model.ClassChat)) {
		t.Fatalf("--all 인데 대화가 없다 :\n%s", out)
	}
}

func TestListShowsInheritArrow(t *testing.T) {
	home, _ := scanNotify(t)
	code, out := capture(t, "list", "--home", home, "--limit", "20")
	if code != exitOK {
		t.Fatalf("list %d\n%s", code, out)
	}
	if !strings.Contains(out, string(model.ClassDoc)+"←") {
		t.Fatalf("이어받기 화살표가 없다 :\n%s", out)
	}
}

func TestEstimateExcludesNonWorkSamples(t *testing.T) {
	home, tasks := scanNotify(t)
	st := store.New(home)
	// 대화 작업을 잔뜩 넣어도 구현 표본은 흔들리면 안 된다.
	base := findTask(tasks, "p-notify-0004")
	if base == nil {
		t.Fatal("대화 작업을 못 찾았다")
	}
	for i := 0; i < 100; i++ {
		chat := *base
		chat.PromptID = "p-chat-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		chat.WallMs = 9_000_000
		tasks = append(tasks, chat)
	}
	if err := st.WriteTasks(tasks); err != nil {
		t.Fatal(err)
	}
	// 대화 100건이 쌓여도 표본으로 안 들어가므로 근거는 n=0 시드여야 한다.
	code, out := capture(t, "estimate", "--home", home, "--days", "100000", "대화:M")
	if code != exitOK {
		t.Fatalf("estimate %d\n%s", code, out)
	}
	if !strings.Contains(out, "시드 (n=0)") {
		t.Fatalf("일 아닌 칸이 표본으로 들어갔다 :\n%s", out)
	}
	if code, out := capture(t, "estimate", "--home", home, "--days", "100000", "구현:M"); !strings.Contains(out, "시드") {
		t.Fatalf("구현 표본이 오염됐다 (%d) :\n%s", code, out)
	}
}
