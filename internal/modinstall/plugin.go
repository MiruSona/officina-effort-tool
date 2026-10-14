package modinstall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
)

// EmbeddedVersion 은 담긴 plugin.json 의 version 이다.
func EmbeddedVersion(src fs.FS) (string, error) {
	raw, err := fs.ReadFile(src, path.Join(Name, pluginRel))
	if err != nil {
		return "", fmt.Errorf("담긴 plugin.json 을 못 읽었습니다 : %w", err)
	}
	m, err := decodePlugin(raw)
	if err != nil {
		return "", fmt.Errorf("담긴 plugin.json 이 JSON 이 아닙니다 : %w", err)
	}
	return pluginVersion(m), nil
}

// withEffortPath 는 plugin.json 의 userConfig.effortPath.default 만 바꾼 글을 준다.
// map 으로 읽어 다시 쓰므로 키 차례는 바뀐다. 뜻은 그대로다 (설계 「plugin.json 다시 쓰기」).
func withEffortPath(raw []byte, effortPath string) ([]byte, error) {
	m, err := decodePlugin(raw)
	if err != nil {
		return nil, fmt.Errorf("담긴 plugin.json 이 JSON 이 아닙니다 : %w", err)
	}
	ep := effortPathEntry(m)
	if ep == nil {
		return nil, fmt.Errorf("담긴 plugin.json 에 userConfig.effortPath 칸이 없습니다")
	}
	ep["default"] = effortPath

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 경로의 & 같은 글자가 & 으로 바뀌지 않게 한다.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodePlugin 은 숫자를 글 그대로(json.Number) 두고 읽는다. 30 이 30.0 따위로 바뀌지 않게.
func decodePlugin(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// effortPathEntry 는 userConfig.effortPath 칸이다. 없으면 nil.
func effortPathEntry(m map[string]any) map[string]any {
	uc, _ := m["userConfig"].(map[string]any)
	if uc == nil {
		return nil
	}
	ep, _ := uc["effortPath"].(map[string]any)
	return ep
}

// effortPathDefault 는 userConfig.effortPath.default 글이다. 없으면 빈 값.
func effortPathDefault(m map[string]any) string {
	ep := effortPathEntry(m)
	if ep == nil {
		return ""
	}
	s, _ := ep["default"].(string)
	return s
}

func pluginVersion(m map[string]any) string {
	s, _ := m["version"].(string)
	return s
}

// withoutEffortPathDefault 는 견주기용으로 effortPath.default 칸을 뺀다 (m 을 고친다).
func withoutEffortPathDefault(m map[string]any) map[string]any {
	if ep := effortPathEntry(m); ep != nil {
		delete(ep, "default")
	}
	return m
}
