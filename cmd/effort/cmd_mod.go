package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/modinstall"
	"github.com/mirusona/officina-effort-tool/internal/paths"
	"github.com/mirusona/officina-effort-tool/mod"
)

// effort mod — exe 에 담긴 effort-steps mod 를 ~/.claude/skills 로 풀고·견주고·걷어 낸다 (설계 2절).
// 대상은 effort-steps 하나뿐이라 이름 인자는 없다. 지우는 길은 없다 — 늘 mod-backups 로 옮긴다.

// executableFunc 는 이 exe 의 경로다. 시험만 바꾼다.
var executableFunc = os.Executable

const configHint = "설정 값 보기·바꾸기 : Claude Code 에서 /config → effort-steps (mode · effortPath · refreshSec · redRatio · autoPane)"

func cmdMod(args []string) error {
	if len(args) == 0 {
		return fail(exitUsage, "쓰는 법 : effort mod install [--force] [--effort-path <경로>] | uninstall | status   (셋 다 [--claude-dir <폴더>])")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "install":
		return modInstall(rest)
	case "uninstall":
		return modUninstall(rest)
	case "status":
		return modStatus(rest)
	case "-h", "--help", "help":
		printHelp("mod")
		return errHelpShown
	}
	return fail(exitUsage, "모르는 mod 명령 : %s (install · uninstall · status)", sub)
}

// modFlags 는 mod 의 옵션이다. --force · --effort-path 는 install 만 받는다.
type modFlags struct {
	claudeDir  string
	effortPath string
	force      bool
}

func parseModFlags(args []string, withInstall bool) (*modFlags, error) {
	fs := newFlags("mod")
	o := &modFlags{}
	fs.StringVar(&o.claudeDir, "claude-dir", "", "Claude Code 폴더 (기본 ~/.claude)")
	if withInstall {
		fs.BoolVar(&o.force, "force", false, "dev-mods 겹침이 있어도 깐다")
		fs.StringVar(&o.effortPath, "effort-path", "", "plugin.json 에 넣을 effort.exe 경로")
	}
	if err := parseFlags(fs, args); err != nil {
		return nil, err
	}
	if len(fs.Args()) > 0 {
		return nil, fail(exitUsage, "mod 는 위치 인자를 안 받습니다 : %s", strings.Join(fs.Args(), " "))
	}
	return o, nil
}

// newInstaller 는 claude-dir 을 정해 설치기를 만든다. 없는 폴더면 오타일 수 있어 멈춘다.
func newInstaller(flagDir string) (*modinstall.Installer, error) {
	dir := flagDir
	if dir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fail(exitRead, "홈 폴더를 못 찾았습니다 : %v", err)
		}
		dir = filepath.Join(h, ".claude")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fail(exitUsage, "Claude Code 폴더가 없습니다 : %s (--claude-dir 로 고릅니다)", dir)
	}
	in, err := modinstall.New(dir, mod.Files)
	if err != nil {
		return nil, fail(exitRead, "설치 자리를 못 정했습니다 : %v", err)
	}
	in.Now = nowFunc
	return in, nil
}

func modInstall(args []string) error {
	o, err := parseModFlags(args, true)
	if err != nil {
		return err
	}
	in, err := newInstaller(o.claudeDir)
	if err != nil {
		return err
	}
	// ① dev-mods 겹침
	overlaps, err := in.DevModOverlaps()
	if err != nil {
		return fail(exitRead, "dev-mods 를 못 읽었습니다 : %v", err)
	}
	printOverlaps(overlaps)
	if len(overlaps) > 0 && !o.force {
		return fail(exitUsage, "dev-mods 에 같은 이름이 있어 멈춥니다. 겹치면 다음 시작 때 설치본이 거절됩니다.\n"+
			"개발 세션이 끝난 뒤 그 폴더를 지우고 다시 칩니다. 그래도 깔려면 --force.")
	}
	if len(overlaps) > 0 {
		fmt.Println("주의 : --force 라 겹친 채로 깝니다. dev-mods 폴더를 지우기 전까지 설치본은 안 올라옵니다.")
	}
	// ② exe 경로
	exe, err := resolveEffortPath(o.effortPath)
	if err != nil {
		return err
	}
	// ③~⑤ 풀기 · 옛 판 옮기기 · 바꿔 끼우기
	res, err := in.Install(exe)
	if err != nil {
		return fail(exitWrite, "%v", err)
	}
	if res.Backup != "" {
		fmt.Printf("backup : %s\n", res.Backup)
	}
	fmt.Printf("installed : %s  (%s %s · exe %s)\n", res.Installed, modinstall.Name, res.Version, buildLabel())
	fmt.Printf("effortPath 기본값 : %s\n", exe)
	if !isFile(exe) {
		fmt.Println("주의 : 그 자리에 파일이 없습니다. mod 는 다른 후보(작업폴더 EffortTool/bin · PATH)를 이어 찾습니다.")
	}
	fmt.Println("설정(/config)에 effortPath 를 따로 적었다면 그 값이 먼저다.")
	fmt.Println("새 세션부터 돈다.")
	return nil
}

// resolveEffortPath 는 plugin.json 에 넣을 exe 경로를 정한다 : --effort-path → 이 exe.
// 이 exe 가 TEMP 아래(go run · go test)면 다음 판에 사라지므로 멈춘다. --effort-path 는 그대로 믿는다.
func resolveEffortPath(flagPath string) (string, error) {
	if flagPath != "" {
		abs, err := filepath.Abs(flagPath)
		if err != nil {
			return "", fail(exitUsage, "--effort-path 를 못 읽었습니다 : %v", err)
		}
		return abs, nil
	}
	exe, err := executableFunc()
	if err != nil {
		return "", fail(exitRead, "이 exe 경로를 못 찾았습니다 : %v (--effort-path 로 줍니다)", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if underTemp(exe) {
		return "", fail(exitUsage, "이 exe 가 임시 폴더 아래에 있습니다 : %s\n"+
			"그 경로를 넣으면 다음 판에 사라집니다. EffortTool 폴더에서 `.\\build.ps1` 로 만든 bin\\effort.exe 로 다시 치거나 --effort-path 를 줍니다.", exe)
	}
	return exe, nil
}

// underTemp 는 p 가 os.TempDir() 아래인지다. 판정은 paths.Jail 의 뿌리 안 검사를 그대로 쓴다.
func underTemp(p string) bool {
	j, err := paths.NewJail(os.TempDir())
	if err != nil {
		return false
	}
	_, err = j.Resolve(p)
	return err == nil
}

func modUninstall(args []string) error {
	o, err := parseModFlags(args, false)
	if err != nil {
		return err
	}
	in, err := newInstaller(o.claudeDir)
	if err != nil {
		return err
	}
	if overlaps, err := in.DevModOverlaps(); err == nil {
		printOverlaps(overlaps)
	}
	bak, err := in.Uninstall()
	if err != nil {
		return fail(exitWrite, "%v", err)
	}
	if bak == "" {
		fmt.Printf("깔려 있지 않다 : %s\n", in.InstallDir())
		return nil
	}
	fmt.Printf("backup : %s\n", bak)
	fmt.Printf("uninstalled : %s\n", in.InstallDir())
	fmt.Println("새 세션부터 빠진다.")
	return nil
}

func modStatus(args []string) error {
	o, err := parseModFlags(args, false)
	if err != nil {
		return err
	}
	in, err := newInstaller(o.claudeDir)
	if err != nil {
		return err
	}
	embedded, err := modinstall.EmbeddedVersion(mod.Files)
	if err != nil {
		return fail(exitRead, "%v", err)
	}
	st, err := in.Compare()
	if err != nil {
		return fail(exitRead, "%v", err)
	}
	overlaps, err := in.DevModOverlaps()
	if err != nil {
		return fail(exitRead, "dev-mods 를 못 읽었습니다 : %v", err)
	}

	if st.State == modinstall.StateMissing {
		fmt.Printf("설치본 : 없음 (%s)\n", in.InstallDir())
	} else {
		fmt.Printf("설치본 : %s  (%s)\n", in.InstallDir(), orUnknown(st.InstalledVersion))
	}
	fmt.Printf("이 exe 에 담긴 판 : %s · exe %s\n", embedded, buildLabel())
	switch st.State {
	case modinstall.StateSame:
		fmt.Printf("견줌 : 같음 (파일 %d개)\n", st.Files)
	case modinstall.StateDiff:
		fmt.Printf("견줌 : 다름 (파일 %d개 중 %d개 : %s)\n", st.Files, len(st.Diffs), strings.Join(st.Diffs, ", "))
	default:
		fmt.Println("견줌 : 없음")
	}
	if st.State != modinstall.StateMissing {
		fmt.Printf("effortPath 기본값 : %s\n", effortPathLabel(st.EffortPathDefault))
	}
	if len(overlaps) == 0 {
		fmt.Println("dev-mods 겹침 : 없음")
	} else {
		printOverlaps(overlaps)
	}
	fmt.Println(configHint)

	switch st.State {
	case modinstall.StateMissing:
		return fail(exitNoData, "설치본이 없습니다. `effort mod install` 로 깝니다.")
	case modinstall.StateDiff:
		return fail(exitNoData, "설치본이 이 exe 에 담긴 판과 다릅니다. `effort mod install` 로 다시 깝니다.")
	}
	return nil
}

func printOverlaps(overlaps []string) {
	for _, p := range overlaps {
		fmt.Printf("dev-mods 겹침 : %s\n", p)
	}
}

func effortPathLabel(p string) string {
	if p == "" {
		return "(비어 있음 — mod 가 작업폴더 EffortTool/bin · PATH 를 찾는다)"
	}
	if isFile(p) {
		return p + "  (있음)"
	}
	return p + "  (없음)"
}

func orUnknown(s string) string {
	if s == "" {
		return "판 모름"
	}
	return s
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
