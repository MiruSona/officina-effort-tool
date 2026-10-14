package modinstall

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mirusona/officina-effort-tool/mod"
)

const testPlugin = `{
  "name": "effort-steps",
  "version": "0.2.0",
  "description": "시험 & 판",
  "userConfig": {
    "effortPath": { "type": "string", "default": "" },
    "refreshSec": { "type": "number", "default": 30 },
    "redRatio": { "type": "number", "default": 1.5 }
  }
}
`

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"effort-steps/.claude-plugin/plugin.json": {Data: []byte(testPlugin)},
		"effort-steps/hooks/main.tsx":             {Data: []byte("export default 1\n")},
		"effort-steps/hooks/hooks.json":           {Data: []byte("{}\n")},
		"effort-steps/types/index.d.ts":           {Data: []byte("export {}\n")},
	}
}

// newTest 는 임시 claude-dir 과 고정 시계로 설치기를 만든다. 시계는 부를 때마다 1초씩 간다.
func newTest(t *testing.T, src fs.FS) (*Installer, string) {
	t.Helper()
	dir := t.TempDir()
	in, err := New(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 14, 21, 30, 0, 0, time.Local)
	n := 0
	in.Now = func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Second)
	}
	return in, dir
}

func readPlugin(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("설치본 plugin.json 이 JSON 이 아니다 : %v", err)
	}
	return m
}

func TestInstallFresh(t *testing.T) {
	in, dir := newTest(t, testFS())
	exe := `C:\툴\EffortTool\bin\effort.exe`
	res, err := in.Install(exe)
	if err != nil {
		t.Fatal(err)
	}
	if res.Backup != "" {
		t.Fatalf("옛 설치본이 없는데 백업이 생겼다 : %s", res.Backup)
	}
	if res.Version != "0.2.0" {
		t.Fatalf("판 = %q", res.Version)
	}
	want := filepath.Join(dir, "skills", Name)
	if !strings.EqualFold(res.Installed, want) {
		t.Fatalf("설치 자리 = %s, 바란 값 %s", res.Installed, want)
	}
	if raw, _ := os.ReadFile(filepath.Join(want, "hooks", "main.tsx")); string(raw) != "export default 1\n" {
		t.Fatalf("hooks/main.tsx 내용이 다르다 : %q", raw)
	}
	m := readPlugin(t, want)
	if got := effortPathDefault(m); got != exe {
		t.Fatalf("effortPath.default = %q", got)
	}
	// 나머지 칸은 그대로다.
	if m["description"] != "시험 & 판" {
		t.Fatalf("description 이 바뀌었다 : %v", m["description"])
	}
	uc := m["userConfig"].(map[string]any)
	if uc["redRatio"].(map[string]any)["default"] != 1.5 || uc["refreshSec"].(map[string]any)["default"] != 30.0 {
		t.Fatalf("다른 칸 값이 바뀌었다 : %v", uc)
	}
	raw, _ := os.ReadFile(filepath.Join(want, ".claude-plugin", "plugin.json"))
	if strings.Contains(string(raw), `\u0026`) {
		t.Fatal("& 가 \\u0026 으로 바뀌었다")
	}
	// .new- 폴더는 옮겨져 남지 않는다.
	ents, _ := os.ReadDir(filepath.Join(dir, "mod-backups"))
	for _, e := range ents {
		if strings.Contains(e.Name(), ".new-") {
			t.Fatalf(".new 폴더가 남았다 : %s", e.Name())
		}
	}
}

func TestReinstallMakesBackup(t *testing.T) {
	in, dir := newTest(t, testFS())
	if _, err := in.Install("a.exe"); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "skills", Name, "옛판표시.txt")
	if err := os.WriteFile(marker, []byte("옛"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := in.Install("b.exe")
	if err != nil {
		t.Fatal(err)
	}
	if res.Backup == "" {
		t.Fatal("재설치인데 백업이 없다")
	}
	if !strings.Contains(filepath.Base(res.Backup), Name+".bak-20261014-21300") {
		t.Fatalf("백업 이름 꼴이 다르다 : %s", res.Backup)
	}
	if _, err := os.Stat(filepath.Join(res.Backup, "옛판표시.txt")); err != nil {
		t.Fatalf("옛 설치본이 백업으로 안 옮겨졌다 : %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("새 설치본에 옛 파일이 남았다")
	}
	if got := effortPathDefault(readPlugin(t, filepath.Join(dir, "skills", Name))); got != "b.exe" {
		t.Fatalf("effortPath.default = %q", got)
	}
}

func TestSameSecondBackupNamesDoNotCollide(t *testing.T) {
	in, dir := newTest(t, testFS())
	fixed := time.Date(2026, 10, 14, 21, 30, 0, 0, time.Local)
	in.Now = func() time.Time { return fixed }
	for i := 0; i < 3; i++ {
		if _, err := in.Install("x.exe"); err != nil {
			t.Fatalf("%d번째 : %v", i, err)
		}
	}
	ents, _ := os.ReadDir(filepath.Join(dir, "mod-backups"))
	if len(ents) != 2 {
		t.Fatalf("백업 수 = %d, 바란 값 2", len(ents))
	}
}

func TestUninstall(t *testing.T) {
	in, dir := newTest(t, testFS())
	bak, err := in.Uninstall()
	if err != nil || bak != "" {
		t.Fatalf("없는데 걷어 냈다 : %q %v", bak, err)
	}
	if _, err := in.Install("x.exe"); err != nil {
		t.Fatal(err)
	}
	bak, err = in.Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.Base(bak), ".bak-") {
		t.Fatalf("백업 이름 = %s", bak)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", Name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("설치본이 남았다")
	}
	if _, err := os.Stat(filepath.Join(bak, "hooks", "main.tsx")); err != nil {
		t.Fatalf("백업에 파일이 없다 : %v", err)
	}
}

func TestCompare(t *testing.T) {
	in, dir := newTest(t, testFS())
	st, err := in.Compare()
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateMissing || st.Files != 4 {
		t.Fatalf("깔기 전 = %+v", st)
	}
	if _, err := in.Install(`D:\다른\effort.exe`); err != nil {
		t.Fatal(err)
	}
	st, err = in.Compare()
	if err != nil {
		t.Fatal(err)
	}
	// effortPath 기본값만 다르고 키 차례가 바뀌었어도 「같음」이다.
	if st.State != StateSame || st.Files != 4 || len(st.Diffs) != 0 {
		t.Fatalf("깐 뒤 = %+v", st)
	}
	if st.InstalledVersion != "0.2.0" || st.EffortPathDefault != `D:\다른\effort.exe` {
		t.Fatalf("판·기본값 = %+v", st)
	}

	// 설치본 밖에만 있는 파일은 안 본다.
	extra := filepath.Join(dir, "skills", Name, ".claude-plugin", "types")
	_ = os.MkdirAll(extra, 0o755)
	_ = os.WriteFile(filepath.Join(extra, "x.d.ts"), []byte("x"), 0o644)
	if st, _ = in.Compare(); st.State != StateSame {
		t.Fatalf("설치본에만 있는 파일로 다름이 됐다 : %+v", st)
	}

	// 파일 하나 고치기 · 하나 지우기 → 다름, 둘 다 찍힌다.
	_ = os.WriteFile(filepath.Join(dir, "skills", Name, "hooks", "main.tsx"), []byte("고침"), 0o644)
	_ = os.Remove(filepath.Join(dir, "skills", Name, "types", "index.d.ts"))
	st, err = in.Compare()
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDiff || strings.Join(st.Diffs, ",") != "hooks/main.tsx,types/index.d.ts" {
		t.Fatalf("고친 뒤 = %+v", st)
	}
}

func TestComparePluginMeaning(t *testing.T) {
	in, dir := newTest(t, testFS())
	if _, err := in.Install("x.exe"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "skills", Name, ".claude-plugin", "plugin.json")
	_ = os.WriteFile(p, []byte(strings.Replace(testPlugin, "0.2.0", "0.1.0", 1)), 0o644)
	st, _ := in.Compare()
	if st.State != StateDiff || st.InstalledVersion != "0.1.0" || st.Diffs[0] != ".claude-plugin/plugin.json" {
		t.Fatalf("판이 다른데 = %+v", st)
	}
	_ = os.WriteFile(p, []byte("{깨짐"), 0o644)
	st, err := in.Compare()
	if err != nil || st.State != StateDiff {
		t.Fatalf("깨진 plugin.json = %+v %v", st, err)
	}
}

func TestDevModOverlaps(t *testing.T) {
	in, dir := newTest(t, testFS())
	got, err := in.DevModOverlaps()
	if err != nil || len(got) != 0 {
		t.Fatalf("dev-mods 없을 때 = %v %v", got, err)
	}
	hit := filepath.Join(dir, "dev-mods", "sess-1", Name)
	_ = os.MkdirAll(hit, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "dev-mods", "sess-2", "agent-board"), 0o755)
	got, err = in.DevModOverlaps()
	if err != nil || len(got) != 1 || !strings.EqualFold(got[0], hit) {
		t.Fatalf("겹침 = %v %v", got, err)
	}
}

// 새 판을 설치 자리로 옮기다 실패하면 옛 설치본이 제자리로 돌아온다.
func TestInstallRollsBackOnFinalMoveFailure(t *testing.T) {
	in, dir := newTest(t, testFS())
	if _, err := in.Install("old.exe"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "skills", Name)
	real := in.rename
	in.rename = func(o, n string) error {
		if strings.EqualFold(n, in.target) && strings.Contains(o, ".new-") {
			return errors.New("꾸민 실패")
		}
		return real(o, n)
	}
	_, err := in.Install("new.exe")
	if err == nil || !strings.Contains(err.Error(), "제자리로 되돌렸습니다") {
		t.Fatalf("오류 = %v", err)
	}
	if got := effortPathDefault(readPlugin(t, target)); got != "old.exe" {
		t.Fatalf("옛 설치본이 안 돌아왔다 : effortPath = %q", got)
	}
}

// 설치 자리가 링크면 옮기지도 읽지도 않는다.
func TestInstallRefusesSymlinkTarget(t *testing.T) {
	in, dir := newTest(t, testFS())
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "skills"), 0o755)
	if err := os.Symlink(outside, filepath.Join(dir, "skills", Name)); err != nil {
		t.Skipf("링크를 못 만든다 (권한) : %v", err)
	}
	if _, err := in.Install("x.exe"); err == nil {
		t.Fatal("링크 자리에 깔았다")
	}
	if _, err := in.Compare(); err == nil {
		t.Fatal("링크 자리를 견줬다")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("링크 대상이 옮겨졌다")
	}
}

func TestEmbeddedModInstalls(t *testing.T) {
	in, _ := newTest(t, mod.Files)
	if _, err := in.Install("x.exe"); err != nil {
		t.Fatal(err)
	}
	st, err := in.Compare()
	if err != nil || st.State != StateSame || st.Files == 0 {
		t.Fatalf("담긴 mod 로 깐 뒤 = %+v %v", st, err)
	}
}

// 윈도우 정션은 권한 없이 만들 수 있고 Go 1.23+ 의 EvalSymlinks 가 안 푼다. 설치 자리가 정션이면 손대지 않는다.
func TestInstallRefusesJunctionTarget(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("윈도우 정션 시험")
	}
	in, dir := newTest(t, testFS())
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "skills"), 0o755)
	link := filepath.Join(dir, "skills", Name)
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("정션을 못 만든다 : %v %s", err, out)
	}
	if _, err := in.Install("x.exe"); err == nil {
		t.Fatal("정션 자리에 깔았다")
	}
	if _, err := in.Compare(); err == nil {
		t.Fatal("정션 자리를 견줬다")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("정션이 옮겨졌다")
	}
	if ents, _ := os.ReadDir(outside); len(ents) != 0 {
		t.Fatalf("정션 너머에 썼다 : %d개", len(ents))
	}
}
