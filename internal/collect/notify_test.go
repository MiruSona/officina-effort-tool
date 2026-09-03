package collect

import (
	"strings"
	"testing"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

func TestParseNotifyTaskID(t *testing.T) {
	text := "<task-notification>\n<task-id>ab12cd34</task-id>\n<tool-use-id>toolu_01Ab</tool-use-id>\n끝났습니다"
	id, toolUse := parseNotifyTags(text)
	if id != "ab12cd34" {
		t.Fatalf("task-id = %q", id)
	}
	if toolUse != "toolu_01Ab" {
		t.Fatalf("tool-use-id = %q", toolUse)
	}
}

func TestMonitorNotifyHasNoToolUseID(t *testing.T) {
	text := "<task-notification><task-id>mon12345</task-id>Monitor 조건이 맞았습니다</task-notification>"
	id, toolUse := parseNotifyTags(text)
	if id != "mon12345" || toolUse != "" {
		t.Fatalf("task-id = %q · tool-use-id = %q", id, toolUse)
	}
}

func TestRejectsBadTagValue(t *testing.T) {
	bad := []string{
		"<task-id>a b</task-id>",                             // 빈칸
		"<task-id>../../etc/passwd</task-id>",                // 경로
		"<task-id>" + strings.Repeat("x", 65) + "</task-id>", // 너무 김
		"<task-id></task-id>",                                // 빈 값
		"<task-id>없는닫는태그",                                    // 안 닫힘
	}
	for _, text := range bad {
		if v := tagValue(text, tagTaskID); v != "" {
			t.Fatalf("%q 에서 값을 받아들였다 : %q", text, v)
		}
	}
	if !okTagValue("a-b_C9") {
		t.Fatal("멀쩡한 값을 막았다")
	}
}

// 알림 본문은 캐시에 안 남고 열쇠와 종류만 남는다.
func TestNotifyKindFromSession(t *testing.T) {
	agent := `{"type":"user","promptId":"p1","uuid":"u1","promptSource":"system","origin":{"kind":"task-notification"},` +
		`"message":{"role":"user","content":"<task-notification><task-id>ab12cd34</task-id><tool-use-id>toolu_9</tool-use-id></task-notification>"},` +
		`"timestamp":"2026-08-30T10:00:00.000Z"}`
	monitor := `{"type":"user","promptId":"p2","uuid":"u2","promptSource":"system","origin":{"kind":"task-notification"},` +
		`"message":{"role":"user","content":"<task-notification><task-id>mon00001</task-id>감시</task-notification>"},` +
		`"timestamp":"2026-08-30T10:10:00.000Z"}`
	res := writeSession(t, agent, monitor)
	if len(res.Tasks) != 2 {
		t.Fatalf("작업 수 = %d", len(res.Tasks))
	}
	if res.Tasks[0].NotifyKind != model.NotifyAgent || res.Tasks[0].NotifyTaskID != "ab12cd34" {
		t.Fatalf("갈래 알림 = %+v", res.Tasks[0])
	}
	if res.Tasks[1].NotifyKind != model.NotifyMonitor {
		t.Fatalf("감시 알림 종류 = %q", res.Tasks[1].NotifyKind)
	}
	if res.Tasks[0].PromptSource != model.SourceSystem {
		t.Fatalf("promptSource = %q", res.Tasks[0].PromptSource)
	}
}

func TestNormalizeSourceFolds(t *testing.T) {
	if model.NormalizeSource("typed") != model.SourceTyped {
		t.Fatal("아는 값을 접었다")
	}
	if model.NormalizeSource("") != "" {
		t.Fatal("빈 값이 기타가 됐다")
	}
	if model.NormalizeSource("아무거나<script>") != model.SourceOther {
		t.Fatal("모르는 값을 안 접었다")
	}
}
