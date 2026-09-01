package collect

import "github.com/mirusona/efforttool/internal/jsonl"

// subtypeAway 는 사람이 자리를 비웠다는 표시다. 앞줄과 3분 이상 떨어져 찍힌다.
const subtypeAway = "away_summary"

// isMainLine 은 「일한 흔적」이 있는 줄인지다. 곁줄은 다음 프롬프트가 큐에 들어오는 순간이나
// 사람이 자리를 비운 뒤에 찍혀, 앞 작업의 끝을 사람 대기만큼 늘린다.
// 빼는 목록이 아니라 넣는 목록인 까닭은 새 곁줄 종류가 생겨도 시간이 안 부풀게 하려는 것이다.
func isMainLine(l *jsonl.Line) bool {
	switch l.Type {
	case "user", "assistant", "attachment":
		return true
	case "system":
		return l.Subtype != subtypeAway
	}
	return false
}

// knownSideTypes 는 우리가 「알고 뺀」 곁줄이다. 여기에도 없는 종류는 scan 이 건수를 찍는다.
var knownSideTypes = map[string]bool{
	// system 은 subtype 으로 갈린다. away_summary 만 곁줄이고 나머지는 본줄이라 여기 오지 않는다.
	"system":                true,
	"queue-operation":       true,
	"file-history-snapshot": true,
	"file-history-delta":    true,
	"last-prompt":           true,
	"mode":                  true,
	"permission-mode":       true,
	"bridge-session":        true,
	"atis-latch":            true,
	"ai-title":              true,
	"cost-state":            true,
}

// unknownLineType 은 본줄도 아는 곁줄도 아닌 종류인지다.
func unknownLineType(l *jsonl.Line) bool {
	if l.Type == "" || isMainLine(l) {
		return false
	}
	return !knownSideTypes[l.Type]
}
