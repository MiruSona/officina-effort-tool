package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 규칙 파일이 그대로면 안 바뀐 세션을 건너뛰고, 한 글자라도 바뀌면 전부 다시 읽는다.
// 캐시에 든 분류는 스캔 때 매겨지므로, 규칙만 고치고 부분 스캔을 하면 옛 분류가 그대로 남는다.
func TestScanRedoesWhenRulesChange(t *testing.T) {
	home := t.TempDir()
	scanArgs := []string{"scan", "--home", home, "--projects", testdataSessions, "--all"}

	code, out := capture(t, scanArgs...)
	if code != exitOK {
		t.Fatalf("첫 스캔 종료 코드 %d\n%s", code, out)
	}

	// 두 번째 : 규칙이 같으니 부분 스캔이다.
	code, out = capture(t, scanArgs...)
	if code != exitOK {
		t.Fatalf("두 번째 스캔 종료 코드 %d\n%s", code, out)
	}
	if strings.Contains(out, "규칙 파일이 바뀌어") {
		t.Fatalf("규칙이 그대로인데 전면 재스캔이 돌았다 :\n%s", out)
	}
	if !strings.Contains(out, "읽은 세션 0개") {
		t.Fatalf("안 바뀐 세션을 다시 읽었다 :\n%s", out)
	}

	// 규칙을 고치면 전면 재스캔이다.
	rules := filepath.Join(home, "rules.txt")
	raw, err := os.ReadFile(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, append(raw, []byte("contfirst\t어어\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = capture(t, scanArgs...)
	if code != exitOK {
		t.Fatalf("세 번째 스캔 종료 코드 %d\n%s", code, out)
	}
	if !strings.Contains(out, "규칙 파일이 바뀌어") {
		t.Fatalf("규칙을 고쳤는데 까닭이 안 찍혔다 :\n%s", out)
	}
	if strings.Contains(out, "읽은 세션 0개") {
		t.Fatalf("규칙을 고쳤는데 다시 안 읽었다 :\n%s", out)
	}

	// 그다음 판은 다시 부분 스캔이어야 한다 — 새 지문이 기록에 남았는가.
	_, out = capture(t, scanArgs...)
	if strings.Contains(out, "규칙 파일이 바뀌어") {
		t.Fatalf("새 지문을 안 남겼다 :\n%s", out)
	}
}

// 지문은 기본값 덧붙이기 「뒤」에 잰다. 앞에서 재면 매번 전면 재스캔이 돈다.
func TestScanHashTakenAfterAppendingDefaults(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	// 종류가 거의 빠진 rules.txt — 스캔이 기본값을 파일 끝에 더한다.
	if err := os.WriteFile(filepath.Join(home, "rules.txt"), []byte("word\t조사\t조사\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanArgs := []string{"scan", "--home", home, "--projects", testdataSessions, "--all"}
	code, out := capture(t, scanArgs...)
	if code != exitOK {
		t.Fatalf("첫 스캔 종료 코드 %d\n%s", code, out)
	}
	if !strings.Contains(out, "기본값을 더했습니다") {
		t.Fatalf("기본값을 안 더했다 :\n%s", out)
	}
	code, out = capture(t, scanArgs...)
	if code != exitOK {
		t.Fatalf("두 번째 스캔 종료 코드 %d\n%s", code, out)
	}
	if strings.Contains(out, "규칙 파일이 바뀌어") {
		t.Fatalf("덧붙이기 앞에서 지문을 쟀다 :\n%s", out)
	}
}
