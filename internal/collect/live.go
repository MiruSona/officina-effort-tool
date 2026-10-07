package collect

import (
	"bufio"
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
	return FindToolUseTail(r, size, func(_ time.Time, text string) bool { return match(text) })
}

// FindToolUseTail 은 FindToolUse 와 같되 match 가 그 줄의 시각도 받는다. 미뤄 묶기가 시각 창을 거는 데 쓴다.
func FindToolUseTail(r io.ReaderAt, size int64, match func(ts time.Time, text string) bool) (time.Time, bool, error) {
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
		if ts, ok := matchToolUseLine(raw, match); ok {
			last = ts
			found = true
		}
	}
	return last, found, nil
}

// matchToolUseLine 은 줄 하나가 셸 도구 tool_use 이고 그 command 가 match 에 맞는지 본다.
func matchToolUseLine(raw []byte, match func(ts time.Time, text string) bool) (time.Time, bool) {
	// 싼 거름부터. tool_use 낱말이 없는 줄은 JSON 을 풀지 않는다.
	if !bytes.Contains(raw, []byte(`"tool_use"`)) {
		return time.Time{}, false
	}
	ts, texts, ok := toolUseTexts(raw)
	if !ok {
		return time.Time{}, false
	}
	for _, t := range texts {
		if match(ts, t) {
			return ts, true
		}
	}
	return time.Time{}, false
}

// MaxLineBytes 보다 긴 줄은 버리고 넘긴다. 그림이 든 도구 결과 줄 하나가 메모리를 다 먹지 않게.
// mark 를 부른 tool_use 줄은 명령 글뿐이라 이보다 훨씬 짧다.
const MaxLineBytes = 16 << 20

// ToolUseAll 은 파일을 앞에서부터 흘려 읽으며 셸 도구 tool_use 의 (시각, command 글) 마다 visit 을 부른다.
// 끝 256KB 밖으로 밀린 줄을 찾을 때만 쓴다 — 메인 세션 파일은 수십 MB 라 비싸서 끝을 둘 둔다.
//
//   - stopAfter : tool_use 줄 시각이 이것을 넘으면 거기서 멈춘다 (기록은 시각 차례로 쌓인다). 영 값이면 끝까지.
//   - budget : 읽은 바이트만큼 깎는다. 0 아래로 가면 멈추고 complete=false 다. nil 이면 상한 없음.
//
// complete 는 멈춤 조건(stopAfter·파일 끝)까지 다 봤는지다.
func ToolUseAll(r io.Reader, stopAfter time.Time, budget *int64, visit func(ts time.Time, text string)) (complete bool, err error) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		raw, n, rerr := readCappedLine(br, MaxLineBytes)
		if budget != nil {
			*budget -= int64(n)
			if *budget < 0 {
				return false, nil
			}
		}
		if raw != nil && bytes.Contains(raw, []byte(`"tool_use"`)) {
			ts, texts, _ := toolUseTexts(raw)
			if !stopAfter.IsZero() && ts.After(stopAfter) {
				return true, nil
			}
			for _, t := range texts {
				visit(ts, t)
			}
		}
		if rerr == io.EOF {
			return true, nil
		}
		if rerr != nil {
			return false, rerr
		}
	}
}

// readCappedLine 은 줄 하나를 읽는다. max 보다 길면 나머지를 버리고 nil 을 준다. n 은 읽은 바이트 수다.
func readCappedLine(br *bufio.Reader, max int) ([]byte, int, error) {
	var line []byte
	n := 0
	tooLong := false
	for {
		chunk, err := br.ReadSlice('\n')
		n += len(chunk)
		if !tooLong {
			if len(line)+len(chunk) > max {
				tooLong, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if tooLong {
			return nil, n, err
		}
		return bytes.TrimRight(line, "\r\n"), n, err
	}
}

// FirstStampBytes 는 FirstTimestamp 가 앞에서 보는 바이트다. 앞머리에 시각 없는 요약 줄이 몇 개 올 수 있다.
const FirstStampBytes = 64 * 1024

// FirstTimestamp 는 파일 앞 64KB 안에서 처음 나오는 timestamp 를 준다. 그 기록이 언제 시작했는지 싸게 알 때 쓴다.
func FirstTimestamp(r io.Reader) (time.Time, bool) {
	br := bufio.NewReaderSize(io.LimitReader(r, FirstStampBytes), 64*1024)
	for {
		raw, _, err := readCappedLine(br, FirstStampBytes)
		var l struct {
			Timestamp time.Time `json:"timestamp"`
		}
		if len(raw) > 0 && json.Unmarshal(raw, &l) == nil && !l.Timestamp.IsZero() {
			return l.Timestamp, true
		}
		if err != nil {
			return time.Time{}, false
		}
	}
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
	WaitMs   int64     // from~to 안에서 시작하고 끝난 긴 도구 틈의 합 (「대기」, 기록 구간에서 안 뺌)
	// FileFirst 는 파일 전체의 첫 본줄이다. LastMain 과 짝지어 「갈래 기록 전체」를 잰다.
	FileFirst time.Time
	// Compacts 는 압축 경계(system · compact_boundary) 줄의 시각이다. 압축 뒤에도 같은 파일에 이어 쓴다.
	Compacts []time.Time
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
// waitMin 은 「대기」 문턱이다. 구간 안 줄만 보므로 구간 밖에서 시작한 도구 틈은 안 든다.
func MainSpan(r io.Reader, from, to time.Time, waitMin time.Duration) (LiveSpan, error) {
	var out LiveSpan
	wait := newWaitTracker(waitMin)
	rd := jsonl.NewReader(r)
	var line jsonl.Line
	for rd.Next(&line) {
		if line.Timestamp.IsZero() || !isMainLine(&line) {
			continue
		}
		ts := line.Timestamp
		if line.Type == "system" && line.Subtype == subtypeCompact {
			out.Compacts = append(out.Compacts, ts)
		}
		if out.FileFirst.IsZero() || ts.Before(out.FileFirst) {
			out.FileFirst = ts
		}
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
		wait.see(&line)
	}
	if err := rd.Err(); err != nil && err != io.EOF {
		return out, err
	}
	out.WaitMs = wait.Ms()
	return out, nil
}
