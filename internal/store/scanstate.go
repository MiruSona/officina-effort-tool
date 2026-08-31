package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SchemaVersion 은 캐시 모양 판이다. 바뀌면 전체 재스캔한다.
const SchemaVersion = "1"

// FileState 는 파일 하나를 어디까지 읽었는지다.
type FileState struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"mtime"`
	Offset  int64 `json:"offset"`
}

// ScanState 는 증분 스캔 기록이다.
type ScanState struct {
	Schema string               `json:"schema"`
	Files  map[string]FileState `json:"files"`
}

func NewScanState() *ScanState {
	return &ScanState{Schema: SchemaVersion, Files: map[string]FileState{}}
}

func (s *Store) LoadScanState() *ScanState {
	raw, err := os.ReadFile(filepath.Join(s.cacheDir(), "scanstate.json"))
	if err != nil {
		return NewScanState()
	}
	var st ScanState
	if json.Unmarshal(raw, &st) != nil || st.Schema != SchemaVersion || st.Files == nil {
		return NewScanState()
	}
	return &st
}

func (s *Store) SaveScanState(st *ScanState) error {
	st.Schema = SchemaVersion
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.cacheDir(), "scanstate.json"), raw)
}

// NeedsFullRead 는 그 파일을 처음부터 다시 읽어야 하는지 본다.
func (st *ScanState) NeedsFullRead(key string, size, mtime int64) bool {
	old, ok := st.Files[key]
	if !ok {
		return true
	}
	if size < old.Size || mtime < old.ModTime {
		return true
	}
	return size != old.Size
}

// Unchanged 는 지난번 뒤로 파일이 그대로인지 본다.
func (st *ScanState) Unchanged(key string, size, mtime int64) bool {
	old, ok := st.Files[key]
	return ok && old.Size == size && old.ModTime == mtime
}
