package classify

import (
	"strings"
	"testing"
	"time"

	"github.com/mirusona/efforttool/internal/model"
)

func at(min int) time.Time {
	return time.Date(2026, 8, 31, 10, min, 0, 0, time.UTC)
}

func notifyTask(min int) model.Task {
	return model.Task{Start: at(min), Origin: OriginNotify, Title: "‹task-notification›"}
}

func TestClassifyInheritFromPrev(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "타일 코드 구현", Origin: "human"},
		notifyTask(2),
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassBuild || tasks[1].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestClassifyInheritNoPrevIsUnknown(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{notifyTask(1)}
	r.ClassifySession(tasks)
	if tasks[0].Class != model.ClassUnknown || tasks[0].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s)", tasks[0].Class, tasks[0].ClassBy)
	}
}

func TestClassifyInheritDoesNotChain(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "문서 정리", Origin: "human"},
		notifyTask(2),
		notifyTask(3),
		notifyTask(4),
	}
	r.ClassifySession(tasks)
	for i := 1; i < len(tasks); i++ {
		if tasks[i].Class != model.ClassDoc {
			t.Fatalf("%d번 분류 = %s, 바란 값 문서", i, tasks[i].Class)
		}
	}
}

func TestClassifyInheritSkipsNonWorkPrev(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "zzz 아무 낱말도 없음", Origin: "human"},
		notifyTask(2),
	}
	r.ClassifySession(tasks)
	if tasks[0].Class != model.ClassChat {
		t.Fatalf("앞 작업 분류 = %s, 바란 값 대화", tasks[0].Class)
	}
	if tasks[1].Class != model.ClassUnknown {
		t.Fatalf("알림 분류 = %s, 바란 값 미분류", tasks[1].Class)
	}
}

func TestClassifyPeerInherits(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "성능 측정", Origin: "human"},
		{Start: at(2), Origin: OriginPeer, Title: "Another Claude session sent a message:"},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassMeasure || tasks[1].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestClassifyNotificationByTitleWhenOriginEmpty(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "규칙 검토", Origin: "human"},
		{Start: at(2), Title: "‹task-notification›"},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassReview || tasks[1].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestClassifyToolRunByPrefix(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "‹bash-input› go build ./...‹/bash-input›"}
	c, by := r.Task(&task)
	if c != model.ClassTool || by != ByOrigin {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyToolRunBySlash(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "/compact"}
	if c, _ := r.Task(&task); c != model.ClassTool {
		t.Fatalf("분류 = %s", c)
	}
}

func TestClassifyToolRunByContains(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "[3 prior /loop wakeups found nothing actionable]"}
	if c, _ := r.Task(&task); c != model.ClassTool {
		t.Fatalf("분류 = %s", c)
	}
}

func TestClassifyWordBeatsToolRun(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "문서 정리해줘"}
	c, by := r.Task(&task)
	if c != model.ClassDoc || by != ByTitle {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyChoreNeedsNoWriteTool(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "이것도 커밋하고 푸시해줘", Tools: map[string]int{"Bash": 4}}
	c, by := r.Task(&task)
	if c != model.ClassChore || by != ByChore {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
	task2 := model.Task{Title: "이것도 커밋하고 푸시해줘", Tools: map[string]int{"Bash": 4, "Edit": 1}}
	if c, _ := r.Task(&task2); c == model.ClassChore {
		t.Fatal("파일을 고쳤는데 잡무로 봤다")
	}
}

func TestClassifyWebResearch(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz 값이 얼마야", Tools: map[string]int{"WebSearch": 3, "Read": 1}}
	c, by := r.Task(&task)
	if c != model.ClassResearch || by != ByWeb {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyWebResearchNeedsWebTool(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz 값이 얼마야", Tools: map[string]int{"Read": 4, "Grep": 2}}
	if c, _ := r.Task(&task); c == model.ClassResearch {
		t.Fatal("웹 도구가 없는데 조사로 봤다")
	}
}

func TestClassifyShellOnceIsToolRun(t *testing.T) {
	r := defaultRules(t)
	one := model.Task{Title: "zzz 돌려 줘", Tools: map[string]int{"Bash": 2}}
	if c, by := r.Task(&one); c != model.ClassTool || by != ByOrigin {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
	many := model.Task{Title: "zzz 돌려 줘", Tools: map[string]int{"Bash": 3}}
	if c, _ := r.Task(&many); c != model.ClassUnknown {
		t.Fatalf("셸 3회 분류 = %s, 바란 값 미분류", c)
	}
}

func TestClassifyChatWhenNoToolNoAgent(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz qqq"}
	c, by := r.Task(&task)
	if c != model.ClassChat || by != ByOrigin {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyChatWord(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "고마워"}
	c, by := r.Task(&task)
	if c != model.ClassChat || by != ByTitle {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

// 곁도구만 쓴 작업은 도구를 안 쓴 것과 같이 봐야 대화로 떨어진다.
func TestClassifySideToolsOnlyIsChat(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz qqq", Tools: map[string]int{
		"SendMessage": 3, "TaskStop": 1, "Monitor": 2, "ScheduleWakeup": 1,
	}}
	c, by := r.Task(&task)
	if c != model.ClassChat || by != ByOrigin {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifySideToolsDoNotBlockShellRun(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz qqq", Tools: map[string]int{"Bash": 1, "ToolSearch": 4}}
	c, by := r.Task(&task)
	if c != model.ClassTool || by != ByOrigin {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

// 웹조사 중에 셸을 몇 번 써도 조사로 봐야 한다.
func TestClassifyWebResearchAllowsShell(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz 값이 얼마야", Tools: map[string]int{
		"WebSearch": 3, "WebFetch": 2, "Bash": 6,
	}}
	c, by := r.Task(&task)
	if c != model.ClassResearch || by != ByWeb {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyWebResearchStillNeedsNoWriteTool(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz 값이 얼마야", Tools: map[string]int{"WebSearch": 3, "Write": 1}}
	if c, _ := r.Task(&task); c == model.ClassResearch {
		t.Fatal("파일을 썼는데 웹조사로 봤다")
	}
}

func TestClassifyToolRunByAgentPrefix(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "Agent 도구로 서브에이전트를 띄웠다"}
	if c, _ := r.Task(&task); c != model.ClassTool {
		t.Fatalf("분류 = %s", c)
	}
}

func TestClassifyMemCommandIsToolRun(t *testing.T) {
	r := defaultRules(t)
	for _, title := range []string{"mem search 타일", "mem index 다시", "mem status 보여줘"} {
		task := model.Task{Title: title}
		if c, _ := r.Task(&task); c != model.ClassTool {
			t.Fatalf("%q 분류 = %s", title, c)
		}
	}
}

func TestClassifyAgentBeatsChat(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "zzz 아무 낱말 없음", Agents: []model.Agent{
		{Description: "zzz 아무 낱말 없음", AgentType: "general-purpose"},
	}}
	if c, _ := r.Task(&task); c == model.ClassChat {
		t.Fatal("서브를 띄웠는데 대화로 봤다")
	}
}

func TestClassifyOrderIsFixed(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), Title: "타일 코드 구현", Origin: "human"},
		{Start: at(2), Origin: OriginNotify, Title: "‹bash-input› ls‹/bash-input›"},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassBuild || tasks[1].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s), 이어받기가 이겨야 한다", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestRulesParseToolSet(t *testing.T) {
	r, err := ParseRules(strings.NewReader("tool\t쓰기\tWrite\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Set(SetWrite)["Write"] {
		t.Fatal("도구 집합에 안 들어갔다")
	}
}

func TestRulesParseUnknownToolSetFails(t *testing.T) {
	_, err := ParseRules(strings.NewReader("# 머리\ntool\t이상한\tWrite\n"))
	if err == nil {
		t.Fatal("모르는 집합인데 안 막았다")
	}
	if !strings.Contains(err.Error(), "2번째 줄") {
		t.Fatalf("줄 번호가 없다 : %v", err)
	}
}

func TestRulesParseTwoFieldKinds(t *testing.T) {
	r, err := ParseRules(strings.NewReader("runpre\t## Context Usage\nrunin\tloop wakeup\nchore\t커밋\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Prefixes) != 1 || r.Prefixes[0] != "## Context Usage" {
		t.Fatalf("접두 = %v", r.Prefixes)
	}
	if len(r.Contains) != 1 || len(r.ChoreWord) != 1 {
		t.Fatalf("포함낱말 = %v · 잡무낱말 = %v", r.Contains, r.ChoreWord)
	}
}
