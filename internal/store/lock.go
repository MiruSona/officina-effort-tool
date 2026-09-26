package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Lock 은 쓰기 명령이 겹쳐 도는 것을 막는다. scan 은 scan.lock, mark 는 marks.lock 을 잡는다.
type Lock struct {
	path string
	body string
}

const lockGrace = 2 * time.Second

// AcquireLock 은 scan 락을 잡는다. 산 락이 있으면 실패한다.
func AcquireLock(dir string) (*Lock, error) {
	return acquireNamed(dir, "scan.lock", "다른 effort scan 이 돌고 있습니다", staleAfter)
}

// acquireNamed 는 이름 있는 락 하나를 잡는다. 산 락이 있으면 busy 글로 실패한다.
// stale 은 이 시간보다 오래된 락을 죽은 것으로 보는 기한이다. scan 은 30분, ms 만 잡는 mark 는 몇 초다.
func acquireNamed(dir, name, busy string, stale time.Duration) (*Lock, error) {
	path := filepath.Join(dir, name)
	body := fmt.Sprintf("%d %d", os.Getpid(), processStartMs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for try := 0; try < 2; try++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := f.WriteString(body)
			f.Close()
			if werr != nil {
				return nil, werr
			}
			return &Lock{path: path, body: body}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if !isStale(path, stale) {
			break
		}
		if err := steal(path); err != nil {
			break
		}
	}
	return nil, fmt.Errorf("%s (%s)", busy, path)
}

// steal 은 죽은 락을 가로챈다. 고유 이름으로 옮겨 본 쪽만 지운다.
// 그냥 지우면 두 스캔이 같은 락을 함께 죽은 것으로 보고 둘 다 들어간다.
func steal(path string) error {
	tmp := fmt.Sprintf("%s.dead.%d.%d", path, os.Getpid(), time.Now().UnixNano())
	if err := os.Rename(path, tmp); err != nil {
		return err
	}
	return os.Remove(tmp)
}

// isStale 은 락이 죽은 것인지 본다. 만든 지 2초 안인 빈 락은 산 것으로 본다.
func isStale(path string, stale time.Duration) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return time.Since(st.ModTime()) > lockGrace
	}
	pid, started, ok := parseLock(string(raw))
	if !ok {
		return true
	}
	if pid == os.Getpid() && started != processStartMs {
		// PID 는 같은데 시작시각이 다르면 재사용된 번호다 — 죽은 락이다.
		return true
	}
	// 프로세스가 살았는지는 OS 마다 보는 법이 달라, 적어 둔 시작시각으로만 본다.
	// 적힌 시작시각과 파일 mtime 중 늦은 쪽으로 본다. 짧은 기한에서 프로세스 시작이 오래전인 경우를 살린다.
	since := time.Since(time.UnixMilli(started))
	if m := time.Since(st.ModTime()); m < since {
		since = m
	}
	return since > stale
}

// 이 프로세스가 뜬 시각. 락 주인이 나인지 가리는 데 쓴다.
var processStartMs = time.Now().UnixMilli()

const staleAfter = 30 * time.Minute

func parseLock(s string) (int, int64, bool) {
	f := strings.Fields(strings.TrimSpace(s))
	if len(f) != 2 {
		return 0, 0, false
	}
	pid, err1 := strconv.Atoi(f[0])
	ms, err2 := strconv.ParseInt(f[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return pid, ms, true
}

// Release 는 파일 내용이 아직 내 락일 때만 지운다.
func (l *Lock) Release() error {
	raw, err := os.ReadFile(l.path)
	if err != nil {
		return nil
	}
	if strings.TrimSpace(string(raw)) != l.body {
		return nil
	}
	var last error
	for i := 0; i < 3; i++ {
		last = os.Remove(l.path)
		if last == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return last
}
