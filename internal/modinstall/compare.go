package modinstall

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
)

// State 는 설치본과 담긴 판을 견준 결과다.
type State string

const (
	StateSame    State = "같음"
	StateDiff    State = "다름"
	StateMissing State = "없음"
)

// Status 는 status 한 판의 결과다.
type Status struct {
	State State
	// Files 는 담긴 파일 수다.
	Files int
	// Diffs 는 다르거나 설치본에 없는 파일(effort-steps 아래 "/" 꼴 경로)이다.
	Diffs []string
	// InstalledVersion 은 설치본 plugin.json 의 version 이다. 못 읽으면 빈 값.
	InstalledVersion string
	// EffortPathDefault 는 설치본 plugin.json 의 effortPath 기본값이다.
	EffortPathDefault string
}

// Compare 는 담긴 파일을 설치본과 하나씩 견준다.
// plugin.json 은 JSON 뜻으로 견주되 effortPath.default 는 뺀다. 나머지는 바이트로 견준다.
// 설치본에만 있는 파일(엔진이 까는 .claude-plugin/types/ 등)은 보지 않는다.
// 설치본을 못 읽으면(권한 등) 오류를 준다. 설치본 plugin.json 이 JSON 이 아니면 「다름」이다.
func (in *Installer) Compare() (*Status, error) {
	files, err := embeddedFiles(in.src)
	if err != nil {
		return nil, err
	}
	st := &Status{Files: len(files)}
	present, err := in.installedDir()
	if err != nil {
		return nil, err
	}
	if !present {
		st.State = StateMissing
		return st, nil
	}
	for _, rel := range files {
		same, err := in.sameFile(rel, st)
		if err != nil {
			return nil, err
		}
		if !same {
			st.Diffs = append(st.Diffs, rel)
		}
	}
	st.State = StateSame
	if len(st.Diffs) > 0 {
		st.State = StateDiff
	}
	return st, nil
}

// sameFile 은 파일 하나를 견준다. 설치본 plugin.json 의 판·기본값은 st 에 적는다.
func (in *Installer) sameFile(rel string, st *Status) (bool, error) {
	want, err := fs.ReadFile(in.src, path.Join(Name, rel))
	if err != nil {
		return false, fmt.Errorf("담긴 파일을 못 읽었습니다 (%s) : %w", rel, err)
	}
	got, err := in.readInstalled(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("설치본을 못 읽었습니다 (%s) : %w", rel, err)
	}
	if rel != pluginRel {
		return bytes.Equal(want, got), nil
	}
	gm, err := decodePlugin(got)
	if err != nil {
		return false, nil
	}
	st.InstalledVersion = pluginVersion(gm)
	st.EffortPathDefault = effortPathDefault(gm)
	wm, err := decodePlugin(want)
	if err != nil {
		return false, fmt.Errorf("담긴 plugin.json 이 JSON 이 아닙니다 : %w", err)
	}
	return reflect.DeepEqual(withoutEffortPathDefault(wm), withoutEffortPathDefault(gm)), nil
}

// readInstalled 는 설치본 안의 보통 파일 하나를 읽는다. 링크는 안 연다 (Jail.OpenRead).
func (in *Installer) readInstalled(rel string) ([]byte, error) {
	f, err := in.skills.OpenRead(filepath.FromSlash(rel))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
