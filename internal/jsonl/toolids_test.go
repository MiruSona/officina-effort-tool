package jsonl

import (
	"encoding/json"
	"testing"
)

// tool_use 의 id 와 tool_result 의 tool_use_id 만 꺼낸다. 글 content 는 빈 값이다.
func TestToolIDs(t *testing.T) {
	var a Line
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{}},{"type":"tool_use","id":"toolu_2","name":"Read","input":{}}]}}`
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	uses, results := a.ToolIDs()
	if len(uses) != 2 || uses[0] != "toolu_1" || uses[1] != "toolu_2" || len(results) != 0 {
		t.Fatalf("uses %v · results %v", uses, results)
	}
	var u Line
	raw = `{"type":"user","message":{"content":[{"tool_use_id":"toolu_1","type":"tool_result","content":"ok"}]}}`
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatal(err)
	}
	uses, results = u.ToolIDs()
	if len(uses) != 0 || len(results) != 1 || results[0] != "toolu_1" {
		t.Fatalf("uses %v · results %v", uses, results)
	}
	var s Line
	if err := json.Unmarshal([]byte(`{"type":"user","message":{"content":"그냥 글"}}`), &s); err != nil {
		t.Fatal(err)
	}
	if uses, results = s.ToolIDs(); uses != nil || results != nil {
		t.Fatalf("글 content 에서 id 가 나왔다 : %v %v", uses, results)
	}
}
