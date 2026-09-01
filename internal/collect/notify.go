package collect

import "strings"

// 알림 본문에서 훑어보는 앞부분 길이. 뒤로 갈수록 서브에이전트가 쓴 보고 글이라 볼 것이 없다.
const maxNotifyScan = 4096

// 열쇠로 쓸 수 있는 값의 길이 상한.
const maxTagValueLen = 64

// 알림 본문의 꼬리표 이름.
const (
	tagTaskID    = "task-id"
	tagToolUseID = "tool-use-id"
)

// 알림인지 알아보는 본문 접두. 살균이 < 를 ‹ 로 접으므로 두 꼴을 다 본다.
var notifyPrefixes = []string{"<task-notification>", "‹task-notification›"}

// isNotifyText 는 알림 본문인지다.
func isNotifyText(text string) bool {
	s := strings.TrimSpace(text)
	for _, p := range notifyPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// parseNotifyTags 는 알림 본문 앞 4KB 에서 <task-id>·<tool-use-id> 값을 꺼낸다.
// 본문 자체는 저장하지 않는다. 꺼낸 값도 꼴이 안 맞으면 버린다.
func parseNotifyTags(text string) (taskID, toolUseID string) {
	head := text
	if len(head) > maxNotifyScan {
		head = head[:maxNotifyScan]
	}
	return tagValue(head, tagTaskID), tagValue(head, tagToolUseID)
}

// tagValue 는 <이름>값</이름> 한 쌍에서 값을 꺼낸다. 꼴이 안 맞으면 빈 글을 준다.
func tagValue(text, name string) string {
	head := "<" + name + ">"
	tail := "</" + name + ">"
	i := strings.Index(text, head)
	if i < 0 {
		return ""
	}
	rest := text[i+len(head):]
	j := strings.Index(rest, tail)
	if j < 0 {
		return ""
	}
	v := strings.TrimSpace(rest[:j])
	if !okTagValue(v) {
		return ""
	}
	return v
}

// okTagValue 는 열쇠로 쓸 수 있는 값인지다 : [0-9A-Za-z_-] 1~64자.
func okTagValue(s string) bool {
	if s == "" || len(s) > maxTagValueLen {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
