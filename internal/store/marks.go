package store

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// marks.txt 는 rules.txt · groups.txt 와 나란한 세 번째 정본이다 (설계 2절).
// 다시 만들 수 없는 값이라 scan --rebuild 로 안 지운다. exe 만 끝에 덧붙이고 고쳐 쓰지 않는다.
const marksHead = "# effort marks — effort mark 가 덧붙인다. 손으로 고치지 않는다.\n"

// 시각은 UTC RFC3339 밀리초로 적는다. 원본 JSONL 의 timestamp 와 같은 꼴이라 견주기 쉽다.
const markTimeLayout = "2006-01-02T15:04:05.000Z07:00"

// 묶기 결과. 묶였으면 빈 값이다.
const (
	BindNone      = "묶기없음"
	BindAmbiguous = "묶기모호"
)

// TimeMark 는 marks.txt 의 start 줄과 그 짝 stop 줄을 합친 것이다.
type TimeMark struct {
	ID    string
	Start time.Time
	Stop  time.Time // 안 닫혔으면 영 값
	Slug  string    // 묶인 파일이 든 프로젝트 폴더 이름. 묶기 실패면 빈 값
	File  string    // 프로젝트 폴더 안 기록 파일 : "<세션id>" 또는 "<세션id>/agent-<id>". 실패면 빈 값
	Name  string
	Bind  string // 묶기 결과 (빈 값 · 묶기없음 · 묶기모호)
}

// Open 은 아직 stop 이 없는지다.
func (m *TimeMark) Open() bool { return m.Stop.IsZero() }

// MarkWarning 은 marks.txt 에서 건너뛴 줄이다. rules.txt R1 과 같이 경고하고 그 줄만 건너뛴다.
// 한 줄이 틀렸다고 start·stop·show·list 가 다 막히면 mark 를 영영 못 쓰게 되기 때문이다.
type MarkWarning struct {
	Line int
	Head string // 모르는 머리 낱말. 아는 머리의 줄 오류면 빈 값
	Msg  string // 아는 머리의 줄 오류 설명
}

func (w MarkWarning) String() string {
	if w.Msg != "" {
		return fmt.Sprintf("marks.txt %d번째 줄 : %s — 건너뜁니다", w.Line, w.Msg)
	}
	return fmt.Sprintf("marks.txt %d번째 줄 : 이 exe 가 모르는 줄 머리 %q — 건너뜁니다. 더 새 exe 가 더한 줄일 수 있습니다", w.Line, w.Head)
}

// marksLockStale 은 marks.lock 을 죽은 것으로 보는 기한이다. 락은 줄 하나 덧붙이는 ms 동안만 잡는다.
const marksLockStale = 10 * time.Second

func (s *Store) MarksPath() string { return filepath.Join(s.home, "marks.txt") }

// NewMarkID 는 m + 월일 + 무작위 16진 4자다. 이미 있는 id 와 겹치면 다시 뽑는다.
func NewMarkID(now time.Time, taken map[string]bool) (string, error) {
	for try := 0; try < 32; try++ {
		var b [2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := "m" + now.Format("0102") + "-" + hex.EncodeToString(b[:])
		if !taken[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("mark id 를 못 뽑았습니다 (겹침이 너무 많습니다)")
}

// AppendMarkStart 는 새 mark id 를 뽑아 start 줄 하나를 덧붙이고 그 id 를 준다.
// id 는 락 안에서 파일을 다시 읽고 뽑는다 — 락 밖에서 뽑으면 동시 start 둘이 같은 id 를 낼 수 있다.
// 묶기 실패면 다섯째 칸이 `-` 이고 일곱째 칸에 까닭을 적는다.
// 일곱째 칸은 설계 꼴(여섯 칸) 뒤에 붙인 것이라 없어도 읽힌다 (R6 과 같은 방식).
func (s *Store) AppendMarkStart(m TimeMark) (string, error) {
	if err := checkMarkField(m.Name); err != nil {
		return "", err
	}
	var id string
	err := s.withMarksLock(func() error {
		marks, _, err := s.LoadMarks()
		if err != nil {
			return err
		}
		taken := map[string]bool{}
		for _, x := range marks {
			taken[x.ID] = true
		}
		id, err = NewMarkID(m.Start, taken)
		if err != nil {
			return err
		}
		m.ID = id
		return s.appendMarkLineLocked(startLine(m))
	})
	return id, err
}

func startLine(m TimeMark) string {
	slug, file := m.Slug, m.File
	if slug == "" {
		slug = "-"
	}
	if file == "" {
		file = "-"
	}
	line := strings.Join([]string{"start", m.ID, m.Start.UTC().Format(markTimeLayout), slug, file, m.Name}, "\t")
	if m.Bind != "" {
		line += "\t" + m.Bind
	}
	return line
}

// AppendMarkStop 은 stop 줄 하나를 덧붙인다.
func (s *Store) AppendMarkStop(id string, at time.Time) error {
	return s.withMarksLock(func() error {
		return s.appendMarkLineLocked(strings.Join([]string{"stop", id, at.UTC().Format(markTimeLayout)}, "\t"))
	})
}

// checkMarkField 는 탭·줄바꿈 같은 칸을 깨는 글자를 막는다.
func checkMarkField(v string) error {
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("이름에 탭·줄바꿈 같은 제어 글자는 못 씁니다")
		}
	}
	return nil
}

// withMarksLock 은 marks.lock 을 잡고 fn 을 부른다.
// 서브에이전트 여럿이 동시에 부를 수 있어 락이 잡혀 있으면 잠깐 기다렸다 다시 본다.
func (s *Store) withMarksLock(fn func() error) error {
	if err := os.MkdirAll(s.home, 0o755); err != nil {
		return err
	}
	var lock *Lock
	var err error
	for try := 0; try < 30; try++ {
		lock, err = acquireNamed(s.home, "marks.lock", "다른 effort mark 가 marks.txt 를 쓰고 있습니다", marksLockStale)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	defer lock.Release()
	return fn()
}

// appendMarkLineLocked 는 끝에 줄 하나를 더한다. 앞줄은 한 바이트도 안 건드린다. 락은 부른 쪽이 잡는다.
func (s *Store) appendMarkLineLocked(line string) error {
	p := s.MarksPath()
	body := ""
	st, statErr := os.Stat(p)
	if os.IsNotExist(statErr) || (statErr == nil && st.Size() == 0) {
		body = marksHead
	} else if statErr != nil {
		return statErr
	} else if !endsWithNewline(p, st.Size()) {
		body = "\n"
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(body + line + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func endsWithNewline(p string, size int64) bool {
	f, err := os.Open(p)
	if err != nil {
		return true
	}
	defer f.Close()
	b := make([]byte, 1)
	if _, err := f.ReadAt(b, size-1); err != nil {
		return true
	}
	return b[0] == '\n'
}

// LoadMarks 는 marks.txt 를 읽어 start 차례대로 준다. 파일이 없으면 빈 목록이다.
// 모르는 머리 낱말 줄, 칸이 모자라거나 시각이 틀린 줄, 짝 없는 stop, 겹친 id(뒤쪽) 는 경고하고 건너뛴다.
// 오류를 돌려주는 것은 파일을 못 읽을 때뿐이다.
func (s *Store) LoadMarks() ([]TimeMark, []MarkWarning, error) {
	f, err := os.Open(s.MarksPath())
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var out []TimeMark
	var warns []MarkWarning
	byID := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		f := strings.Split(line, "\t")
		bad := func(msg string, a ...any) {
			warns = append(warns, MarkWarning{Line: n, Msg: fmt.Sprintf(msg, a...)})
		}
		switch f[0] {
		case "start":
			m, msg := parseStart(f)
			if msg != "" {
				bad("%s", msg)
				continue
			}
			if _, dup := byID[m.ID]; dup {
				bad("mark id %s 가 앞에서 이미 나왔습니다", m.ID)
				continue
			}
			byID[m.ID] = len(out)
			out = append(out, m)
		case "stop":
			if len(f) < 3 {
				bad("stop 은 id·시각 두 칸이 필요합니다")
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, f[2])
			if err != nil {
				bad("시각이 틀렸습니다 (%s)", f[2])
				continue
			}
			i, ok := byID[f[1]]
			if !ok {
				bad("시작이 없는 mark id %s", f[1])
				continue
			}
			// 첫 stop 만 쓴다. 덧붙이기만 하므로 두 번째 stop 이 앞 값을 덮으면 안 된다.
			if out[i].Stop.IsZero() {
				out[i].Stop = at
			}
		default:
			warns = append(warns, MarkWarning{Line: n, Head: f[0]})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, warns, err
	}
	return out, warns, nil
}

// parseStart 는 start 줄을 푼다. 틀렸으면 까닭 글을 준다.
func parseStart(f []string) (TimeMark, string) {
	if len(f) < 6 {
		return TimeMark{}, "start 는 id·시각·프로젝트·파일·이름 다섯 칸이 필요합니다"
	}
	at, err := time.Parse(time.RFC3339Nano, f[2])
	if err != nil {
		return TimeMark{}, fmt.Sprintf("시각이 틀렸습니다 (%s)", f[2])
	}
	m := TimeMark{ID: f[1], Start: at, Slug: f[3], File: f[4], Name: f[5]}
	if m.Slug == "-" {
		m.Slug = ""
	}
	if m.File == "-" {
		m.File = ""
	}
	if len(f) >= 7 {
		m.Bind = f[6]
	}
	if m.File == "" && m.Bind == "" {
		m.Bind = BindNone
	}
	return m, ""
}
