package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SchemaVersion 은 캐시 모양 판이다. 바뀌면 전체 재스캔한다.
// 1 → 2 : Task.Tools (도구 이름별 호출 수) 추가.
// 2 → 3 : 벽시계 뜻이 「본줄 ∪ 서브 구간 합집합」으로 바뀜 · promptSource · 알림 task-id · parentAgentId.
// 3 → 4 : 벽시계를 본줄만으로 되돌림. 서브 구간은 AgentWallMs 에만 남는다.
const SchemaVersion = "4"

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
	// RulesHash 는 마지막 스캔 때 쓴 rules.txt 의 지문이다.
	// 분류는 스캔 때 캐시에 박히므로 규칙이 바뀌면 옛 분류를 통째로 다시 매겨야 한다.
	// 옛 기록에는 이 칸이 없어 빈 값으로 읽힌다 — 그러면 한 번은 전면 재스캔이 돈다.
	RulesHash string `json:"rules_hash"`
}

func NewScanState() *ScanState {
	return &ScanState{Schema: SchemaVersion, Files: map[string]FileState{}}
}

// LoadScanState 는 증분 기록을 읽는다.
// 판이 다르면 Files 는 비우고 Schema 에는 파일에 적힌 옛 판을 담는다 — 호출자가 판을 비교할 수 있어야 한다.
func (s *Store) LoadScanState() *ScanState {
	raw, err := os.ReadFile(filepath.Join(s.cacheDir(), "scanstate.json"))
	if err != nil {
		return NewScanState()
	}
	var st ScanState
	if json.Unmarshal(raw, &st) != nil {
		return NewScanState()
	}
	if st.Schema != SchemaVersion || st.Files == nil {
		return &ScanState{Schema: st.Schema, Files: map[string]FileState{}}
	}
	return &st
}

// Stale 은 캐시가 옛 판이라 통째로 다시 읽어야 하는지다.
func (st *ScanState) Stale() bool { return st.Schema != SchemaVersion }

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
