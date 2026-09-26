package collect

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMatchMarkCommand(t *testing.T) {
	cases := []struct {
		text, verb, name string
		want             bool
	}{
		{`effort mark start "2-타일"`, "start", "2-타일", true},
		{`.\bin\effort.exe mark start 2-타일; echo`, "start", "2-타일", true},
		{`effort mark start '2-타일그림'`, "start", "2-타일", false},
		{`effort mark start 2-타일그림`, "start", "2-타일", false},
		{`effort mark starts 2-타일`, "start", "2-타일", false},
		{`effort mark stop`, "stop", "", true},
		{`effort mark stop m0926-ab12`, "stop", "", true},
		{`effort mark stopper`, "stop", "", false},
		{`ls`, "start", "2-타일", false},
	}
	for _, c := range cases {
		if got := MatchMarkCommand(c.text, c.verb, c.name); got != c.want {
			t.Fatalf("%q %s %q = %v", c.text, c.verb, c.name, got)
		}
	}
}

// 끝쪽만 읽고, 잘린 첫 줄은 버리고, 맞는 마지막 줄의 시각을 준다.
func TestFindToolUseTail(t *testing.T) {
	at := time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC)
	line := `{"type":"assistant","timestamp":"2026-09-26T05:00:00Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"effort mark start \"2-타일\""}}]}}`
	var buf bytes.Buffer
	buf.WriteString(strings.Repeat("x", LiveTailBytes)) // 앞머리 잘린 줄
	buf.WriteString("\n")
	buf.WriteString(`{"type":"user","timestamp":"2026-09-26T04:59:00Z","message":{"content":"mark start 2-타일"}}` + "\n")
	buf.WriteString(line + "\n")
	r := bytes.NewReader(buf.Bytes())
	got, ok, err := FindToolUse(r, int64(buf.Len()), func(s string) bool { return MatchMarkCommand(s, "start", "2-타일") })
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("got %v %v %v", got, ok, err)
	}
	// user 줄의 글은 tool_use 가 아니라 안 잡힌다.
	user := `{"type":"user","timestamp":"2026-09-26T04:59:00Z","message":{"content":[{"type":"tool_result","content":"mark start 2-타일"}]}}` + "\n"
	ur := strings.NewReader(user)
	_, ok, _ = FindToolUse(ur, int64(len(user)), func(s string) bool { return MatchMarkCommand(s, "start", "2-타일") })
	if ok {
		t.Fatal("tool_use 가 아닌 줄에서 찾았다")
	}
}

// 셸 도구의 command 칸만 본다. Agent 프롬프트·Write 내용에 같은 명령이 있어도 안 잡힌다.
func TestFindToolUseShellCommandOnly(t *testing.T) {
	lines := `{"type":"assistant","timestamp":"2026-09-26T05:00:00Z","message":{"content":[{"type":"tool_use","name":"Agent","input":{"prompt":"effort mark start \"2-타일\""}}]}}
{"type":"assistant","timestamp":"2026-09-26T05:00:01Z","message":{"content":[{"type":"tool_use","name":"Write","input":{"content":"effort mark start \"2-타일\""}}]}}
`
	match := func(s string) bool { return MatchMarkCommand(s, "start", "2-타일") }
	if _, ok, _ := FindToolUse(strings.NewReader(lines), int64(len(lines)), match); ok {
		t.Fatal("셸 도구가 아닌 입력에서 찾았다")
	}
	ps := `{"type":"assistant","timestamp":"2026-09-26T05:00:02Z","message":{"content":[{"type":"tool_use","name":"PowerShell","input":{"command":"effort.exe mark start 2-타일"}}]}}` + "\n"
	if _, ok, _ := FindToolUse(strings.NewReader(ps), int64(len(ps)), match); !ok {
		t.Fatal("PowerShell command 를 못 찾았다")
	}
}
