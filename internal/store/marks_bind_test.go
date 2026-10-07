package store

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// 여러 판이 같은 mark 를 동시에 묶어도 bind 줄은 하나만 남는다 (락 안에서 다시 읽는다).
func TestAppendMarkBindConcurrent(t *testing.T) {
	s := New(t.TempDir())
	at := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	id, err := s.AppendMarkStart(TimeMark{Start: at, Name: "1-동시", Bind: BindNone})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wrote := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.AppendMarkBind(id, at, "C--p", "sess", "")
			if err != nil {
				t.Error(err)
			}
			wrote <- ok
		}()
	}
	wg.Wait()
	close(wrote)
	n := 0
	for ok := range wrote {
		if ok {
			n++
		}
	}
	raw, _ := os.ReadFile(s.MarksPath())
	if lines := strings.Count(string(raw), "\nbind\t"); lines != 1 || n != 1 {
		t.Fatalf("bind 줄 %d개 · 썼다는 판 %d개 (둘 다 1 이어야 한다)\n%s", lines, n, raw)
	}
}

// 포기 뒤 성공 bind 는 덮는다. 포기를 또 적지는 않는다.
func TestAppendMarkBindSettleThenSuccess(t *testing.T) {
	s := New(t.TempDir())
	at := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	id, _ := s.AppendMarkStart(TimeMark{Start: at, Name: "2-포기", Bind: BindNone})
	if ok, _ := s.AppendMarkBind(id, at, "", "", BindNone); !ok {
		t.Fatal("포기를 못 적었다")
	}
	if ok, _ := s.AppendMarkBind(id, at, "", "", BindNone); ok {
		t.Fatal("포기를 두 번 적었다")
	}
	if ok, _ := s.AppendMarkBind(id, at, "C--p", "sess", ""); !ok {
		t.Fatal("포기 뒤 성공 bind 를 못 적었다")
	}
	ms, _, _ := s.LoadMarks()
	if m := ms[0]; m.File != "sess" || m.Settled || !m.Late {
		t.Fatalf("성공 bind 가 포기를 못 덮었다 : %+v", m)
	}
}
