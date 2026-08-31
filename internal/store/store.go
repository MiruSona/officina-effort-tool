package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/mirusona/efforttool/internal/classify"
	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/secret"
)

// ErrNoCache 는 아직 scan 을 안 돌렸다는 뜻이다.
var ErrNoCache = errors.New("캐시가 없습니다")

// Store 는 .effort 아래 파생 저장소다.
type Store struct {
	home string
}

func New(home string) *Store { return &Store{home: home} }

func (s *Store) Home() string     { return s.home }
func (s *Store) cacheDir() string { return filepath.Join(s.home, "cache") }
func (s *Store) RulesPath() string {
	return filepath.Join(s.home, "rules.txt")
}

func (s *Store) EnsureDirs() error {
	return os.MkdirAll(s.cacheDir(), 0o755)
}

// EnsureRules 는 rules.txt 가 없을 때만 기본값으로 만든다. 있으면 절대 안 덮는다.
func (s *Store) EnsureRules() (created bool, err error) {
	p := s.RulesPath()
	if _, err := os.Stat(p); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(s.home, 0o755); err != nil {
		return false, err
	}
	if err := writeAtomic(p, []byte(classify.DefaultRulesText)); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) LoadRules() (*classify.Rules, error) {
	f, err := os.Open(s.RulesPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return classify.ParseRules(f)
}

// WriteTasks 는 작업을 캐시에 넣는다. 살균·비밀검사를 지나는 쓰기 입구는 여기뿐이다.
func (s *Store) WriteTasks(tasks []model.Task) error {
	for i := range tasks {
		secret.MaskAll(tasks[i].SanitizableFields())
	}
	return s.writeJSONL(filepath.Join(s.cacheDir(), "tasks.jsonl"), len(tasks), func(i int) any {
		return tasks[i]
	})
}

func (s *Store) WriteSessions(sessions []model.Session) error {
	for i := range sessions {
		secret.MaskAll(sessions[i].SanitizableFields())
	}
	return s.writeJSONL(filepath.Join(s.cacheDir(), "sessions.jsonl"), len(sessions), func(i int) any {
		return sessions[i]
	})
}

func (s *Store) writeJSONL(path string, n int, get func(int) any) error {
	if err := s.EnsureDirs(); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for i := 0; i < n; i++ {
		if err := enc.Encode(get(i)); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) ReadTasks() ([]model.Task, error) {
	var out []model.Task
	err := s.readJSONL(filepath.Join(s.cacheDir(), "tasks.jsonl"), func(dec *json.Decoder) error {
		var t model.Task
		if err := dec.Decode(&t); err != nil {
			return err
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

func (s *Store) ReadSessions() ([]model.Session, error) {
	var out []model.Session
	err := s.readJSONL(filepath.Join(s.cacheDir(), "sessions.jsonl"), func(dec *json.Decoder) error {
		var v model.Session
		if err := dec.Decode(&v); err != nil {
			return err
		}
		out = append(out, v)
		return nil
	})
	return out, err
}

func (s *Store) readJSONL(path string, step func(*json.Decoder) error) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return ErrNoCache
	}
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReaderSize(f, 256*1024))
	for dec.More() {
		if err := step(dec); err != nil {
			return err
		}
	}
	return nil
}

// writeAtomic 은 tmp 에 다 쓰고 이름을 바꾼다. 반쯤 쓰인 파일이 안 남는다.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
