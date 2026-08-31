package jsonl

import (
	"strings"
	"testing"
)

func TestScannerHandlesLongLine(t *testing.T) {
	big := strings.Repeat("가", 400*1024) // UTF-8 로 1MB 을 넘는다
	src := `{"type":"user","message":{"role":"user","content":"` + big + `"}}` + "\n"
	rd := NewReader(strings.NewReader(src))
	var line Line
	if !rd.Next(&line) {
		t.Fatal("1MB 줄을 못 읽었다 (64KB 기본값 회귀)")
	}
	if line.Type != "user" {
		t.Fatalf("Type = %q", line.Type)
	}
	if rd.Bad != 0 {
		t.Fatalf("Bad = %d", rd.Bad)
	}
}

func TestScannerSkipsOverlongLineAndCounts(t *testing.T) {
	huge := strings.Repeat("a", 9*1024*1024)
	src := `{"type":"user","message":{"role":"user","content":"` + huge + `"}}` + "\n" +
		`{"type":"assistant"}` + "\n"
	rd := NewReader(strings.NewReader(src))
	var line Line
	if !rd.Next(&line) {
		t.Fatal("긴 줄 뒤 줄을 못 읽었다")
	}
	if line.Type != "assistant" {
		t.Fatalf("Type = %q", line.Type)
	}
	if rd.TooLong != 1 {
		t.Fatalf("TooLong = %d, 바란 값 1", rd.TooLong)
	}
	if rd.Bad != 1 {
		t.Fatalf("Bad = %d, 바란 값 1", rd.Bad)
	}
}

func TestReaderCountsBrokenJSON(t *testing.T) {
	src := "{not json}\n{\"type\":\"user\"}\n"
	rd := NewReader(strings.NewReader(src))
	var line Line
	if !rd.Next(&line) || line.Type != "user" {
		t.Fatal("깨진 줄 다음을 못 읽었다")
	}
	if rd.Bad != 1 || rd.Total != 2 {
		t.Fatalf("Bad=%d Total=%d", rd.Bad, rd.Total)
	}
}

func TestUserTextFromBlocks(t *testing.T) {
	src := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"안녕"}]}}` + "\n"
	rd := NewReader(strings.NewReader(src))
	var line Line
	if !rd.Next(&line) {
		t.Fatal("못 읽었다")
	}
	if got := line.UserText(); got != "안녕" {
		t.Fatalf("UserText = %q", got)
	}
}
