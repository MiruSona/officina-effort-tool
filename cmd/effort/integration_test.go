package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/collect"
	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

const testdataSessions = "../../testdata/sessions"

// capture 는 명령을 돌리고 표준출력을 잡는다.
func capture(t *testing.T, args ...string) (int, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := run(args)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return code, string(out)
}

// copyTree 는 시험용 사본을 만든다. 원본은 절대 안 건드린다.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scanTestdata(t *testing.T) (*store.Store, []model.Task) {
	t.Helper()
	home := t.TempDir()
	code, out := capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	if code != exitOK {
		t.Fatalf("scan 종료 코드 %d\n%s", code, out)
	}
	st := store.New(home)
	tasks, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	return st, tasks
}

func findTask(tasks []model.Task, promptID string) *model.Task {
	for i := range tasks {
		if tasks[i].PromptID == promptID {
			return &tasks[i]
		}
	}
	return nil
}

func TestScanSmallSessionMatchesCostState(t *testing.T) {
	st, tasks := scanTestdata(t)
	if len(tasks) != 4 {
		t.Fatalf("작업 수 = %d, 바란 값 4", len(tasks))
	}
	sessions, err := st.ReadSessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.SessionID != "sess-small0001" {
			continue
		}
		if s.SumUsage.Sum().Total() != s.StateUsage.Sum().Total() {
			t.Fatalf("작업 합 %d != cost-state %d",
				s.SumUsage.Sum().Total(), s.StateUsage.Sum().Total())
		}
		return
	}
	t.Fatal("작은 세션을 못 찾았다")
}

func TestScanCoverPctReported(t *testing.T) {
	st, _ := scanTestdata(t)
	sessions, _ := st.ReadSessions()
	for _, s := range sessions {
		if s.SessionID == "sess-small0001" && (s.CoverPct < 99.9 || s.CoverPct > 100.1) {
			t.Fatalf("CoverPct = %.2f", s.CoverPct)
		}
		if s.SessionID == "sess-run0001" && !s.InProgress {
			t.Fatal("도는 세션을 못 알아봤다")
		}
	}
}

func TestScanNoSubagentsFolder(t *testing.T) {
	_, tasks := scanTestdata(t)
	task := findTask(tasks, "p-nosub-0001")
	if task == nil {
		t.Fatal("작업을 못 찾았다")
	}
	if !task.HasWarn(collect.WarnNoSubagent) {
		t.Fatalf("경고가 없다 : %v", task.Warn)
	}
}

func TestScanInProgressSessionExcluded(t *testing.T) {
	_, tasks := scanTestdata(t)
	task := findTask(tasks, "p-run-0001")
	if task == nil || !task.HasWarn(collect.WarnInProgress) {
		t.Fatalf("진행중 표시가 없다 : %+v", task)
	}
}

func TestScanIncrementalAddsTail(t *testing.T) {
	root := t.TempDir()
	copyTree(t, testdataSessions, root)
	home := t.TempDir()
	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("첫 scan %d\n%s", code, out)
	}
	st := store.New(home)
	before, _ := st.ReadTasks()

	path := filepath.Join(root, "small", "sess-small0001.jsonl")
	tail := `{"parentUuid":null,"isSidechain":false,"promptId":"p-small-0003","type":"user","message":{"role":"user","content":"[삭제]"},"uuid":"u-small-0003","timestamp":"2026-08-30T10:20:00.000Z","cwd":"C:\\proj","sessionId":"sess-small0001","version":"2.1.238"}` + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(tail)
	f.Close()

	if code, out := capture(t, "scan", "--home", home, "--projects", root, "--all"); code != exitOK {
		t.Fatalf("두번째 scan %d\n%s", code, out)
	}
	after, _ := st.ReadTasks()
	if len(after) != len(before)+1 {
		t.Fatalf("작업 수 %d → %d, 하나만 늘어야 한다", len(before), len(after))
	}
}

func TestScanRebuildEqualsFullScan(t *testing.T) {
	root := t.TempDir()
	copyTree(t, testdataSessions, root)
	homeA := t.TempDir()
	homeB := t.TempDir()
	capture(t, "scan", "--home", homeA, "--projects", root, "--all")
	capture(t, "scan", "--home", homeA, "--projects", root, "--all")
	capture(t, "scan", "--home", homeB, "--projects", root, "--all", "--rebuild")
	a, _ := os.ReadFile(filepath.Join(homeA, "cache", "tasks.jsonl"))
	b, _ := os.ReadFile(filepath.Join(homeB, "cache", "tasks.jsonl"))
	if string(a) != string(b) {
		t.Fatal("증분 결과와 --rebuild 결과가 다르다")
	}
}

func TestScanSchemaBumpForcesRebuild(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	p := filepath.Join(home, "cache", "scanstate.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	bumped := strings.Replace(string(raw), `"schema":"`+store.SchemaVersion+`"`, `"schema":"옛판"`, 1)
	os.WriteFile(p, []byte(bumped), 0o644)
	code, out := capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	if code != exitOK {
		t.Fatalf("scan %d\n%s", code, out)
	}
	if strings.Contains(out, "안 바뀜 3개") {
		t.Fatalf("스키마 판이 다른데 다시 안 읽었다 :\n%s", out)
	}
}

func TestScanNeverWritesToProjectsRoot(t *testing.T) {
	root := t.TempDir()
	copyTree(t, testdataSessions, root)
	before := snapshot(t, root)
	capture(t, "scan", "--home", t.TempDir(), "--projects", root, "--all")
	after := snapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("원본 폴더의 파일 수가 바뀌었다 : %d → %d", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("원본 파일이 바뀌었다 : %s", k)
		}
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		out[p] = fmt.Sprintf("%s|%d", info.ModTime(), info.Size())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEndToEndEstimateTable(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	code, out := capture(t, "estimate", "--home", home, "--days", "100000", "조사:M", "설계:M", "구현:L")
	if code != exitOK {
		t.Fatalf("estimate 종료 코드 %d\n%s", code, out)
	}
	golden := filepath.FromSlash("../../testdata/golden/estimate.md")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("골든 파일이 없다 : %v", err)
	}
	if normalize(out) != normalize(string(want)) {
		t.Fatalf("표가 골든과 다르다.\n--- 나온 것 ---\n%s\n--- 골든 ---\n%s", out, want)
	}
}

func normalize(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n")
}

func TestExitCodes(t *testing.T) {
	home := t.TempDir()
	if code, _ := capture(t, "rules", "--home", home, "--check"); code != exitOK {
		t.Fatalf("0 을 바랐다 : %d", code)
	}
	if code, _ := capture(t, "stats", "--home", home, "--없는옵션"); code != exitUsage {
		t.Fatalf("1 을 바랐다 : %d", code)
	}
	if code, _ := capture(t, "list", "--home", t.TempDir()); code != exitNoData {
		t.Fatalf("2 를 바랐다 : %d", code)
	}
	missing := filepath.Join(t.TempDir(), "없는폴더")
	if code, _ := capture(t, "scan", "--home", t.TempDir(), "--projects", missing, "--all"); code != exitRead {
		t.Fatalf("3 을 바랐다 : %d", code)
	}
	if code, _ := capture(t, "scan", "--home", t.TempDir(), "--projects", corruptRoot(t), "--all"); code != exitCorrupt {
		t.Fatalf("5 를 바랐다 : %d", code)
	}
}

// corruptRoot 는 깨진 줄이 절반인 프로젝트 폴더를 만든다.
func corruptRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "깨진프로젝트")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(`{"type":"user","promptId":"p1","timestamp":"2026-08-30T10:00:00.000Z","sessionId":"s"}` + "\n")
	for i := 0; i < 9; i++ {
		b.WriteString("{망가진 줄}\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "sess-bad0001.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEstimateArgsAndFromConflict(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	code, _ := capture(t, "estimate", "--home", home, "--from", "-", "조사:M")
	if code != exitUsage {
		t.Fatalf("인자와 --from 을 같이 줬는데 종료 코드 %d", code)
	}
}

func TestStatsByClassRuns(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	code, out := capture(t, "stats", "--home", home, "--by", "class")
	if code != exitOK {
		t.Fatalf("stats 종료 코드 %d\n%s", code, out)
	}
	if !strings.Contains(out, "미분류") {
		t.Fatalf("표가 이상하다 :\n%s", out)
	}
	if code, _ := capture(t, "stats", "--home", home, "--by", "없는것"); code != exitUsage {
		t.Fatal("모르는 --by 를 안 막았다")
	}
}

func TestShowMatchesPrefix(t *testing.T) {
	home := t.TempDir()
	capture(t, "scan", "--home", home, "--projects", testdataSessions, "--all")
	code, out := capture(t, "show", "--home", home, "p-small-0001")
	if code != exitOK {
		t.Fatalf("show 종료 코드 %d\n%s", code, out)
	}
	if !strings.Contains(out, "검산") {
		t.Fatalf("검산 줄이 없다 :\n%s", out)
	}
}
