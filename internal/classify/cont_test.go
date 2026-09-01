package classify

import (
	"strings"
	"testing"
	"time"

	"github.com/mirusona/efforttool/internal/model"
)

// contTasks 는 앞 일 칸 하나와 이어질지 볼 작업 하나를 만든다.
func contTasks(title string, source string, gap time.Duration) []model.Task {
	prev := model.Task{Start: at(0), End: at(1), Title: "타일 코드 구현", Origin: "human"}
	next := model.Task{
		Start:        at(1).Add(gap),
		End:          at(1).Add(gap).Add(time.Minute),
		Title:        title,
		Origin:       "human",
		PromptSource: source,
		// 도구를 몇 개 써서 대화·도구실행으로 안 떨어지고 기본(미분류)까지 오게 한다.
		Tools: map[string]int{"Read": 3, "Bash": 4},
	}
	return []model.Task{prev, next}
}

func TestContInheritFourConditions(t *testing.T) {
	r := defaultRules(t)
	cases := []struct {
		name   string
		title  string
		source string
		gap    time.Duration
		want   model.Class
		wantBy string
	}{
		{"네 조건 다 맞음", "좋아 그대로 다 해줘", model.SourceTyped, 3 * time.Minute, model.ClassBuild, ByContinue},
		{"사람이 친 것이 아님", "좋아 그대로 다 해줘", model.SourceSystem, 3 * time.Minute, model.ClassUnknown, ByDefault},
		{"글머리 낱말 없음", "타일맵을 zzz 해줘", model.SourceTyped, 3 * time.Minute, model.ClassUnknown, ByDefault},
		{"끊는 낱말 있음", "좋아 그런데 다른 에이전트가 작업중이야", model.SourceTyped, 3 * time.Minute, model.ClassUnknown, ByDefault},
		{"10분을 넘김", "좋아 그대로 다 해줘", model.SourceTyped, 30 * time.Minute, model.ClassUnknown, ByDefault},
	}
	for _, c := range cases {
		tasks := contTasks(c.title, c.source, c.gap)
		r.ClassifySession(tasks)
		if tasks[1].Class != c.want || tasks[1].ClassBy != c.wantBy {
			t.Fatalf("%s : 분류 = %s (%s), 바란 값 %s (%s)",
				c.name, tasks[1].Class, tasks[1].ClassBy, c.want, c.wantBy)
		}
	}
}

// 잡무 낱말이 있으면 이어짐을 막기만 하고 분류는 안 바꾼다.
func TestChoreWordBlocksContInherit(t *testing.T) {
	r := defaultRules(t)
	tasks := contTasks("좋아 그러면 다 커밋하고 푸시해줘", model.SourceTyped, 2*time.Minute)
	r.ClassifySession(tasks)
	if tasks[1].Class == model.ClassBuild {
		t.Fatalf("잡무 낱말이 있는데 일 칸을 물려받았다 : %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
}

// 물려받은 값은 다시 물려주지 않는다. 짝지은 갈래의 분류가 세션을 물들이면 안 된다.
func TestInheritedClassNotChained(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), End: at(2), Title: "타일 코드 구현", Origin: "human"},
		{
			Start: at(3), End: at(4), Origin: OriginNotify, Title: "‹task-notification›",
			NotifyKind: model.NotifyAgent, NotifyTaskID: "ab12cd34",
			Agents: []model.Agent{{AgentID: "ab12cd34", Description: "성능 측정 실측"}},
		},
		// 짝짓기로 받은 실측이 아니라 원래 앞 일 칸(구현)을 물려받아야 한다.
		{Start: at(5), End: at(6), Origin: OriginNotify, Title: "‹task-notification›"},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassMeasure || tasks[1].ClassBy != ByMatch {
		t.Fatalf("짝짓기 = %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
	if tasks[2].Class != model.ClassBuild || tasks[2].ClassBy != ByInherit {
		t.Fatalf("셋째 = %s (%s), 바란 값 구현 (이어받기)", tasks[2].Class, tasks[2].ClassBy)
	}
}

// 이어짐 창은 「바로 앞 작업의 끝」에서 잰다. 일 칸이 아닌 작업이 끼어도 창이 밀린다.
func TestContWindowUsesPreviousTaskEnd(t *testing.T) {
	r := defaultRules(t)
	tasks := contTasks("좋아 그대로 다 해줘", model.SourceTyped, 3*time.Minute)
	r.ClassifySession(tasks)
	if tasks[1].ClassBy != ByContinue {
		t.Fatalf("3분 뒤인데 안 이어졌다 : %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
	far := contTasks("좋아 그대로 다 해줘", model.SourceTyped, 11*time.Minute)
	r.ClassifySession(far)
	if far[1].ClassBy == ByContinue {
		t.Fatal("11분 뒤인데 이어졌다")
	}
}

func TestNotifyMatchByTaskID(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), End: at(2), Title: "문서 정리", Origin: "human"},
		{
			Start: at(3), End: at(4), Title: "타일 코드 구현", Origin: "human",
			Agents: []model.Agent{{AgentID: "ab12cd34", Description: "성능 측정 실측"}},
		},
		{
			Start: at(5), End: at(6), Origin: OriginNotify, Title: "‹task-notification›",
			NotifyKind: model.NotifyAgent, NotifyTaskID: "ab12cd34",
		},
	}
	r.ClassifySession(tasks)
	// 앞 일 칸(구현)이 아니라 짝지은 갈래의 분류(실측)를 써야 한다.
	if tasks[2].Class != model.ClassMeasure || tasks[2].ClassBy != ByMatch {
		t.Fatalf("분류 = %s (%s), 바란 값 실측 (짝짓기)", tasks[2].Class, tasks[2].ClassBy)
	}
}

func TestNotifyFallsBackToInheritWhenNoMatch(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), End: at(2), Title: "타일 코드 구현", Origin: "human"},
		{
			Start: at(3), End: at(4), Origin: OriginNotify, Title: "‹task-notification›",
			NotifyKind: model.NotifyAgent, NotifyTaskID: "없는아이디",
		},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassBuild || tasks[1].ClassBy != ByInherit {
		t.Fatalf("분류 = %s (%s)", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestMonitorNotifyIsToolRun(t *testing.T) {
	r := defaultRules(t)
	tasks := []model.Task{
		{Start: at(1), End: at(2), Title: "타일 코드 구현", Origin: "human"},
		{
			Start: at(3), End: at(4), Origin: OriginNotify, Title: "‹task-notification›",
			NotifyKind: model.NotifyMonitor, NotifyTaskID: "mon00001",
		},
	}
	r.ClassifySession(tasks)
	if tasks[1].Class != model.ClassTool || tasks[1].ClassBy != ByOrigin {
		t.Fatalf("감시 알림 분류 = %s (%s), 바란 값 도구실행 (원인)", tasks[1].Class, tasks[1].ClassBy)
	}
}

func TestRulesParseContKinds(t *testing.T) {
	text := "contfirst\t좋아\ncontstop\t궁금\ncont\tmax\t7\ngap\tmax\t45\n"
	r, err := ParseRules(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ContFirst) != 1 || len(r.ContStop) != 1 {
		t.Fatalf("글머리 = %v · 끊는말 = %v", r.ContFirst, r.ContStop)
	}
	if r.ContMax != 7 || r.GapMax != 45 {
		t.Fatalf("이어짐 상한 = %d · 묶기 간격 = %d", r.ContMax, r.GapMax)
	}
}

func TestContDefaultsWhenRuleMissing(t *testing.T) {
	r, err := ParseRules(strings.NewReader("chore\t커밋\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r.ContMax != 10 || r.GapMax != 30 {
		t.Fatalf("기본값 = %d · %d, 바란 값 10 · 30", r.ContMax, r.GapMax)
	}
	miss := r.MissingKinds()
	found := false
	for _, k := range miss {
		if k == KindCont {
			found = true
		}
	}
	if !found {
		t.Fatalf("빠진 종류에 이어짐이 없다 : %v", miss)
	}
}

func TestSizeNamesAndHasSize(t *testing.T) {
	r := defaultRules(t)
	if !r.HasSize("l") || r.HasSize("30") {
		t.Fatal("크기 이름 검사가 이상하다")
	}
	names := r.SizeNames()
	if len(names) != 4 || names[0] != "S" || names[3] != "XL" {
		t.Fatalf("크기 차례 = %v", names)
	}
}
