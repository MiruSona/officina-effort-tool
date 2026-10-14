package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeExe 는 TEMP 밖처럼 보일 effort.exe 를 만든다. 시험 폴더도 TEMP 아래라 --effort-path 로 준다.
func fakeExe(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "effort.exe")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// install → status(같음, 0) → uninstall → status(없음, 2) → uninstall(할 것 없음, 0).
func TestModInstallStatusUninstall(t *testing.T) {
	dir := t.TempDir()
	exe := fakeExe(t)

	code, out := capture(t, "mod", "install", "--effort-path", exe, "--claude-dir", dir)
	if code != exitOK {
		t.Fatalf("install 종료 %d\n%s", code, out)
	}
	if strings.Contains(out, "backup :") {
		t.Fatalf("처음 까는데 backup 줄이 있다\n%s", out)
	}
	for _, want := range []string{"installed : ", "effortPath 기본값 : " + exe, "새 세션부터 돈다."} {
		if !strings.Contains(out, want) {
			t.Fatalf("install 출력에 %q 가 없다\n%s", want, out)
		}
	}

	code, out = capture(t, "mod", "status", "--claude-dir", dir)
	if code != exitOK {
		t.Fatalf("status 종료 %d (바란 값 0)\n%s", code, out)
	}
	for _, want := range []string{"견줌 : 같음 (파일 ", exe + "  (있음)", "dev-mods 겹침 : 없음", "/config"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status 출력에 %q 가 없다\n%s", want, out)
		}
	}

	// 다시 깔면 옛 판이 백업으로 간다.
	code, out = capture(t, "mod", "install", "--effort-path", exe, "--claude-dir", dir)
	if code != exitOK || !strings.Contains(out, "backup : ") || !strings.Contains(out, ".bak-") {
		t.Fatalf("재설치 종료 %d\n%s", code, out)
	}

	code, out = capture(t, "mod", "uninstall", "--claude-dir", dir)
	if code != exitOK || !strings.Contains(out, "uninstalled : ") {
		t.Fatalf("uninstall 종료 %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "effort-steps")); !os.IsNotExist(err) {
		t.Fatal("uninstall 뒤에 설치본이 남았다")
	}

	code, out = capture(t, "mod", "status", "--claude-dir", dir)
	if code != exitNoData || !strings.Contains(out, "설치본 : 없음") {
		t.Fatalf("없을 때 status 종료 %d (바란 값 2)\n%s", code, out)
	}

	code, out = capture(t, "mod", "uninstall", "--claude-dir", dir)
	if code != exitOK || !strings.Contains(out, "깔려 있지 않다") {
		t.Fatalf("없을 때 uninstall 종료 %d\n%s", code, out)
	}
}

func TestModStatusDiffExit2(t *testing.T) {
	dir := t.TempDir()
	if code, out := capture(t, "mod", "install", "--effort-path", fakeExe(t), "--claude-dir", dir); code != exitOK {
		t.Fatalf("install 종료 %d\n%s", code, out)
	}
	p := filepath.Join(dir, "skills", "effort-steps", "hooks", "main.tsx")
	if err := os.WriteFile(p, []byte("고침"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, "mod", "status", "--claude-dir", dir)
	if code != exitNoData || !strings.Contains(out, "견줌 : 다름") || !strings.Contains(out, "hooks/main.tsx") {
		t.Fatalf("다를 때 status 종료 %d (바란 값 2)\n%s", code, out)
	}
}

// 이 exe 가 TEMP 아래(go run · go test)면 --effort-path 없이 멈춘다.
func TestModInstallRejectsTempExe(t *testing.T) {
	dir := t.TempDir()
	old := executableFunc
	executableFunc = func() (string, error) { return filepath.Join(os.TempDir(), "go-build1", "effort.exe"), nil }
	t.Cleanup(func() { executableFunc = old })

	code, _ := capture(t, "mod", "install", "--claude-dir", dir)
	if code != exitUsage {
		t.Fatalf("TEMP exe 인데 종료 %d (바란 값 1)", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "effort-steps")); !os.IsNotExist(err) {
		t.Fatal("멈췄는데 설치본이 생겼다")
	}
}

// dev-mods 겹침은 --force 없이 멈추고, --force 면 깐다.
func TestModInstallDevModOverlap(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dev-mods", "sess-1", "effort-steps"), 0o755); err != nil {
		t.Fatal(err)
	}
	exe := fakeExe(t)
	code, out := capture(t, "mod", "install", "--effort-path", exe, "--claude-dir", dir)
	if code != exitUsage || !strings.Contains(out, "dev-mods 겹침 : ") {
		t.Fatalf("겹침인데 종료 %d (바란 값 1)\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "effort-steps")); !os.IsNotExist(err) {
		t.Fatal("멈췄는데 설치본이 생겼다")
	}
	code, out = capture(t, "mod", "install", "--force", "--effort-path", exe, "--claude-dir", dir)
	if code != exitOK || !strings.Contains(out, "--force") {
		t.Fatalf("--force 인데 종료 %d\n%s", code, out)
	}
}

func TestModUsageErrors(t *testing.T) {
	dir := t.TempDir()
	cases := [][]string{
		{"mod"},
		{"mod", "zap"},
		{"mod", "status", "--force", "--claude-dir", dir},
		{"mod", "status", "--claude-dir", dir, "effort-steps"},
		{"mod", "status", "--claude-dir", filepath.Join(dir, "없음")},
	}
	for _, args := range cases {
		if code, _ := capture(t, args...); code != exitUsage {
			t.Fatalf("%v : 종료 %d (바란 값 1)", args, code)
		}
	}
}
