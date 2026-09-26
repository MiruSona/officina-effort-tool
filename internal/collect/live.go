package collect

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/jsonl"
)

// LiveTailBytes 는 자기 파일을 찾을 때 파일 끝에서 읽는 크기다 (설계 2절 · U2).
// mark 를 부른 tool_use 줄은 방금 써진 것이라 끝쪽에 있다. 통째로 읽지 않아 도는 판 수십 개를 봐도 싸다.
const LiveTailBytes = 256 * 1024

// FindToolUse 는 파일 끝쪽의 assistant 줄에서 tool_use 입력 글이 match 에 맞는 마지막 줄의 시각을 준다.
// 본문은 메모리에서만 보고 버린다 — 부른 쪽에는 시각 하나만 간다 (개인정보 원칙).
// 끝쪽 앞머리의 잘린 줄은 버린다. 한 줄이 LiveTailBytes 보다 길면 못 찾는다.
func FindToolUse(r io.ReaderAt, size int64, match func(text string) bool) (time.Time, bool, error) {
	start := size - LiveTailBytes
	if start < 0 {
		start = 0
	}
	buf := make([]byte, size-start)
	n, err := r.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return time.Time{}, false, err
	}
	buf = buf[:n]
	if start > 0 {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			return time.Time{}, false, nil
		}
		buf = buf[i+1:]
	}
	var last time.Time
	found := false
	for _, raw := range bytes.Split(buf, []byte("\n")) {
		// 싼 거름부터. tool_use 낱말이 없는 줄은 JSON 을 풀지 않는다.
		if !bytes.Contains(raw, []byte(`"tool_use"`)) {
			continue
		}
		ts, texts, ok := toolUseTexts(raw)
		if !ok {
			continue
		}
		for _, t := range texts {
			if match(t) {
				last = ts
				found = true
				break
			}
		}
	}
	return last, found, nil
}

// shellTools 는 명령줄을 실제로 돌리는 도구다. mark 는 이 도구의 command 칸에서만 찾는다.
// 입력 글 전부를 보면 부모가 Agent 프롬프트에 적은 `effort mark start "…"` 까지 맞아 부모 세션 파일이 같이 잡힌다.
var shellTools = map[string]bool{"Bash": true, "PowerShell": true}

// toolUseTexts 는 assistant 줄 하나에서 셸 도구 tool_use 의 command 글만 꺼낸다.
func toolUseTexts(raw []byte) (time.Time, []string, bool) {
	var l struct {
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(bytes.TrimRight(raw, "\r"), &l) != nil || l.Type != "assistant" || l.Timestamp.IsZero() {
		return time.Time{}, nil, false
	}
	var blocks []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			Command string `json:"command"`
		} `json:"input"`
	}
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return time.Time{}, nil, false
	}
	var out []string
	for _, b := range blocks {
		if b.Type != "tool_use" || !shellTools[b.Name] || b.Input.Command == "" {
			continue
		}
		out = append(out, b.Input.Command)
	}
	return l.Timestamp, out, len(out) > 0
}

// MatchMarkCommand 는 글에 `mark <verb>` 가 있고 그 바로 뒤 인자가 name 인지 본다.
// name 이 비면 `mark <verb>` 만 본다. 따옴표는 벗겨 보고, 이름 뒤가 낱말 끝이어야 맞다 —
// 그래야 「2-타일」이 「2-타일그림」을 부르는 옆 갈래 파일에 잘못 묶이지 않는다.
func MatchMarkCommand(text, verb, name string) bool {
	key := "mark " + verb
	rest := text
	for {
		i := strings.Index(rest, key)
		if i < 0 {
			return false
		}
		rest = rest[i+len(key):]
		if name == "" {
			if rest == "" || isArgEnd(rest[0]) {
				return true
			}
			continue
		}
		arg := strings.TrimLeft(rest, " \t")
		if len(arg) == len(rest) {
			continue // `mark starts` 처럼 낱말이 이어진 것
		}
		if arg != "" && (arg[0] == '"' || arg[0] == '\'' || arg[0] == '`') {
			arg = arg[1:]
		}
		if strings.HasPrefix(arg, name) {
			after := arg[len(name):]
			if after == "" || isArgEnd(after[0]) {
				return true
			}
		}
	}
}

func isArgEnd(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '"', '\'', '`', ';', '&', '|', ')':
		return true
	}
	return false
}

// LiveSpan 은 mark 한 판의 기록 구간이다.
type LiveSpan struct {
	First    time.Time // from~to 안 첫 본줄
	Last     time.Time // from~to 안 끝 본줄
	LastMain time.Time // 파일 전체의 마지막 본줄
}

// Ms 는 기록 구간 길이다. 본줄이 없으면 -1.
func (s LiveSpan) Ms() int64 {
	if s.First.IsZero() {
		return -1
	}
	return s.Last.Sub(s.First).Milliseconds()
}

// MainSpan 은 두 시각 사이 본줄의 첫~끝 시각과 파일 전체의 마지막 본줄 시각을 준다.
// 곁줄(대기열·자리 비움 등)은 isMainLine 이 빼므로 끝을 늘리지 않는다. to 가 영 값이면 끝을 안 막는다.
func MainSpan(r io.Reader, from, to time.Time) (LiveSpan, error) {
	var out LiveSpan
	rd := jsonl.NewReader(r)
	var line jsonl.Line
	for rd.Next(&line) {
		if line.Timestamp.IsZero() || !isMainLine(&line) {
			continue
		}
		ts := line.Timestamp
		if ts.After(out.LastMain) {
			out.LastMain = ts
		}
		if ts.Before(from) || (!to.IsZero() && ts.After(to)) {
			continue
		}
		if out.First.IsZero() || ts.Before(out.First) {
			out.First = ts
		}
		if ts.After(out.Last) {
			out.Last = ts
		}
	}
	if err := rd.Err(); err != nil && err != io.EOF {
		return out, err
	}
	return out, nil
}
