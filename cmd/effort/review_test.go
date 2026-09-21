package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureBoth 는 stdout 과 stderr 를 따로 받는다. 어느 쪽으로 나갔는지를 보는 시험용이다.
func captureBoth(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wo, we
	code = run(args)
	wo.Close()
	we.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	so, _ := io.ReadAll(ro)
	se, _ := io.ReadAll(re)
	return code, string(so), string(se)
}

// -h 는 사용법 오류가 아니다. 여덟 하위 명령 모두 종료 0 · 오류 줄 없음 · 제 도움말이 stdout 이다.
func TestSubcommandHelpExitsZero(t *testing.T) {
	cmds := []string{"scan", "stats", "estimate", "show", "list", "group", "actual", "rules"}
	for _, c := range cmds {
		for _, f := range []string{"-h", "--help"} {
			code, out, errOut := captureBoth(t, c, f)
			if code != exitOK {
				t.Fatalf("%s %s 종료 코드 %d\n%s%s", c, f, code, out, errOut)
			}
			if errOut != "" {
				t.Fatalf("%s %s 가 stderr 에 찍었다 : %q", c, f, errOut)
			}
			if !strings.HasPrefix(out, c+" :") {
				t.Fatalf("%s %s 가 제 도움말이 아니다 :\n%s", c, f, out)
			}
		}
	}
}

// 모르는 옵션은 그대로 사용법 오류여야 한다.
func TestUnknownFlagStillUsageError(t *testing.T) {
	code, _, _ := captureBoth(t, "list", "--없는옵션")
	if code != exitUsage {
		t.Fatalf("모르는 옵션인데 종료 코드 %d", code)
	}
}

// 캐시에 없는 묶음 경고는 stderr 로 가야 한다. stdout 에 섞이면 --json 이 깨진다.
func TestGroupStatsJSONStaysClean(t *testing.T) {
	home := scanForGroups(t)
	code, out := capture(t, "group", "--home", home, "--add", "없는 작업 묶음",
		"--class", "문서", "p-notify-0001", "p-nosuch-9999")
	if code != exitOK {
		t.Fatalf("group --add %d\n%s", code, out)
	}
	code, stdout, stderr := captureBoth(t, "stats", "--home", home, "--by", "group", "--json")
	if code != exitOK {
		t.Fatalf("stats %d\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "groups.txt") {
		t.Fatalf("경고가 stderr 에 없다 :\n%s", stderr)
	}
	var v any
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Fatalf("stdout 이 JSON 이 아니다 : %v\n%s", err, stdout)
	}
}
