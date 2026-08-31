package jsonl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	startBufBytes = 256 * 1024
	maxLineBytes  = 8 * 1024 * 1024
)

// Reader 는 JSONL 을 한 줄씩 읽는다. 긴 줄과 깨진 줄은 건너뛰고 센다.
type Reader struct {
	br      *bufio.Reader
	offset  int64
	Total   int
	Bad     int // JSON 이 깨진 줄
	TooLong int // 상한을 넘어 건너뛴 줄
	err     error
}

func NewReader(r io.Reader) *Reader {
	return &Reader{br: bufio.NewReaderSize(r, startBufBytes)}
}

// Next 는 다음 줄을 읽는다. 못 읽으면 false 를 준다.
func (r *Reader) Next(out *Line) bool {
	for {
		raw, tooLong, ok := r.readLine()
		if !ok {
			return false
		}
		r.Total++
		if tooLong {
			r.TooLong++
			r.Bad++
			continue
		}
		raw = bytes.TrimRight(raw, "\r")
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		*out = Line{}
		if json.Unmarshal(raw, out) != nil {
			r.Bad++
			continue
		}
		return true
	}
}

// readLine 은 줄 하나를 읽는다. 상한을 넘으면 나머지를 버리고 tooLong 을 준다.
func (r *Reader) readLine() (line []byte, tooLong bool, ok bool) {
	var buf []byte
	for {
		chunk, err := r.br.ReadSlice('\n')
		r.offset += int64(len(chunk))
		if errors.Is(err, bufio.ErrBufferFull) {
			if tooLong {
				continue
			}
			if len(buf)+len(chunk) <= maxLineBytes {
				buf = append(buf, chunk...)
				continue
			}
			tooLong = true
			buf = nil
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			r.err = err
			return nil, false, false
		}
		if errors.Is(err, io.EOF) && len(chunk) == 0 && len(buf) == 0 {
			return nil, false, false
		}
		buf = append(buf, bytes.TrimSuffix(chunk, []byte("\n"))...)
		if tooLong {
			return nil, true, true
		}
		return buf, false, true
	}
}

func (r *Reader) Err() error { return r.err }

// Offset 은 지금까지 읽은 바이트 수다 (증분 스캔용).
func (r *Reader) Offset() int64 { return r.offset }

// BadRatio 는 못 읽은 줄 비율이다.
func (r *Reader) BadRatio() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Bad) / float64(r.Total)
}
