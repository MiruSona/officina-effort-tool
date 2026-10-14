package mod

import (
	"encoding/json"
	"io/fs"
	"sort"
	"testing"
)

// 담긴 파일 목록이 정한 셋(plugin.json · hooks · types)과 같은지 본다.
// hooks 에 파일이 늘면(예: gate.ts) 이 목록도 같이 고친다.
func TestFilesList(t *testing.T) {
	want := []string{
		"effort-steps/.claude-plugin/plugin.json",
		"effort-steps/hooks/effort.ts",
		"effort-steps/hooks/gate.ts",
		"effort-steps/hooks/hooks.json",
		"effort-steps/hooks/main.tsx",
		"effort-steps/hooks/pair.ts",
		"effort-steps/hooks/parse.ts",
		"effort-steps/hooks/state.ts",
		"effort-steps/hooks/table.ts",
		"effort-steps/hooks/view.tsx",
		"effort-steps/types/index.d.ts",
		"effort-steps/types/mcp-self.d.ts",
	}

	var got []string
	err := fs.WalkDir(Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("파일 수 %d, 기대 %d\n got: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

// plugin.json 이 JSON 으로 읽히고 이름이 effort-steps 인지 본다.
// 이름이 바뀌면 mcp__effort-steps__plan 같은 도구 이름이 다 깨진다.
func TestPluginJSON(t *testing.T) {
	b, err := Files.ReadFile("effort-steps/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("plugin.json 이 JSON 이 아니다: %v", err)
	}
	if m["name"] != "effort-steps" {
		t.Errorf("name = %v, want effort-steps", m["name"])
	}
}
