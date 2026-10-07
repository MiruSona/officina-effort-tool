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

// MainSpan 은 파일 첫 본줄(FileFirst)과 압축 경계 시각을 같이 모은다. 구간 밖 줄도 센다.
func TestMainSpanFileFirstAndCompacts(t *testing.T) {
	lines := `{"type":"queue-operation","timestamp":"2026-10-05T08:54:00Z"}
{"type":"user","timestamp":"2026-10-05T08:55:04Z","message":{"content":"시작"}}
{"type":"assistant","timestamp":"2026-10-05T08:56:00Z","message":{"content":"…"}}
{"type":"system","subtype":"compact_boundary","timestamp":"2026-10-05T08:56:30Z"}
{"type":"assistant","timestamp":"2026-10-05T08:57:00Z","message":{"content":"…"}}
{"type":"assistant","timestamp":"2026-10-05T08:59:04Z","message":{"content":"…"}}
`
	from := time.Date(2026, 10, 5, 8, 55, 30, 0, time.UTC)
	to := time.Date(2026, 10, 5, 8, 58, 0, 0, time.UTC)
	s, err := MainSpan(strings.NewReader(lines), from, to, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 5, 8, 55, 4, 0, time.UTC); !s.FileFirst.Equal(want) {
		t.Fatalf("FileFirst = %v (곁줄은 빼고 첫 본줄이어야 한다)", s.FileFirst)
	}
	if want := time.Date(2026, 10, 5, 8, 59, 4, 0, time.UTC); !s.LastMain.Equal(want) {
		t.Fatalf("LastMain = %v", s.LastMain)
	}
	if len(s.Compacts) != 1 || !s.Compacts[0].Equal(time.Date(2026, 10, 5, 8, 56, 30, 0, time.UTC)) {
		t.Fatalf("Compacts = %v", s.Compacts)
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

// ToolUseAll 은 파일 전체를 보고, stopAfter 를 넘는 tool_use 줄에서 멈추고, 너무 긴 줄은 버린다.
// FirstTimestamp 는 시각 없는 앞줄을 건너뛴다.
func TestToolUseAllAndFirstTimestamp(t *testing.T) {
	long := `{"type":"assistant","timestamp":"2026-10-07T08:25:10Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"effort mark start \"2-가\" ` + strings.Repeat("x", MaxLineBytes) + `"}}]}}`
	lines := `{"type":"summary","summary":"…"}
{"type":"user","timestamp":"2026-10-07T08:25:00Z","message":{"content":"시작"}}
` + long + `
{"type":"assistant","timestamp":"2026-10-07T08:25:44.680Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"effort mark start \"2-가\""}}]}}
{"type":"assistant","timestamp":"2026-10-07T08:30:00Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"effort mark start \"2-가\""}}]}}`
	var seen []time.Time
	visit := func(ts time.Time, s string) {
		if MatchMarkCommand(s, "start", "2-가") {
			seen = append(seen, ts)
		}
	}
	stop := time.Date(2026, 10, 7, 8, 26, 0, 0, time.UTC)
	complete, err := ToolUseAll(strings.NewReader(lines), stop, nil, visit)
	want := time.Date(2026, 10, 7, 8, 25, 44, 680_000_000, time.UTC)
	if err != nil || !complete || len(seen) != 1 || !seen[0].Equal(want) {
		t.Fatalf("긴 줄은 버리고 08:30 줄 앞에서 멈춰야 한다 : %v %v %v", seen, complete, err)
	}
	budget := int64(1000)
	seen = nil
	if complete, _ := ToolUseAll(strings.NewReader(lines), time.Time{}, &budget, visit); complete || len(seen) != 0 {
		t.Fatalf("상한을 넘으면 complete=false 로 멈춰야 한다 : %v %v", complete, seen)
	}
	first, ok := FirstTimestamp(strings.NewReader(lines))
	if !ok || !first.Equal(time.Date(2026, 10, 7, 8, 25, 0, 0, time.UTC)) {
		t.Fatalf("FirstTimestamp = %v %v", first, ok)
	}
	if _, ok := FirstTimestamp(strings.NewReader(strings.Repeat("x", FirstStampBytes) + "\n" + lines)); ok {
		t.Fatal("앞 64KB 밖의 시각을 읽었다")
	}
}
