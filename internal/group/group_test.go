package group

import (
	"testing"
	"time"

	"github.com/mirusona/efforttool/internal/model"
	"github.com/mirusona/efforttool/internal/store"
)

var base = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

func at(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

// task 는 시작 min 분에 lenMin 분짜리 작업 하나다.
func task(id, session string, min, lenMin int, c model.Class) model.Task {
	return model.Task{
		PromptID: id, SessionID: session, Title: id + " 제목",
		Start: at(min), End: at(min + lenMin), WallMs: int64(lenMin) * 60000,
		PureMs: int64(lenMin) * 30000, Class: c,
	}
}

func markKey() Key { return Key{Mode: ModeMark, Gap: 30 * time.Minute} }

func TestParseKey(t *testing.T) {
	def := 30 * time.Minute
	if k, err := ParseKey("", def); err != nil || k.Mode != ModeMark || k.Gap != def {
		t.Fatalf("빈 값 = %+v · %v", k, err)
	}
	if k, err := ParseKey("gap:45m", def); err != nil || k.Mode != ModeGap || k.Gap != 45*time.Minute {
		t.Fatalf("gap:45m = %+v · %v", k, err)
	}
	if _, err := ParseKey("gap:없음", def); err == nil {
		t.Fatal("이상한 간격을 안 막았다")
	}
	if _, err := ParseKey("모르는열쇠", def); err == nil {
		t.Fatal("모르는 열쇠를 안 막았다")
	}
}

func TestBuildGapCut(t *testing.T) {
	tasks := []model.Task{
		task("a", "s1", 0, 5, model.ClassBuild),
		task("b", "s1", 10, 5, model.ClassBuild),
		// 앞과 40분 떨어졌다 — 컷 30분을 넘으니 새 묶음이다.
		task("c", "s1", 50, 5, model.ClassBuild),
	}
	gs := Build(tasks, nil, Key{Mode: ModeGap, Gap: 30 * time.Minute})
	if len(gs) != 2 {
		t.Fatalf("묶음 수 = %d, 바란 값 2", len(gs))
	}
	if len(gs[0].Tasks) != 2 || len(gs[1].Tasks) != 1 {
		t.Fatalf("묶음 크기 = %d · %d", len(gs[0].Tasks), len(gs[1].Tasks))
	}
}

func TestBuildSessionBreaksGroup(t *testing.T) {
	tasks := []model.Task{
		task("a", "s1", 0, 5, model.ClassBuild),
		task("b", "s2", 6, 5, model.ClassBuild),
	}
	gs := Build(tasks, nil, Key{Mode: ModeGap, Gap: 30 * time.Minute})
	if len(gs) != 2 {
		t.Fatalf("세션이 다른데 한 묶음이 됐다 : %d", len(gs))
	}
}

func TestBuildMarkBeatsAuto(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 5, model.ClassBuild),
		task("bbbb2222", "s1", 6, 5, model.ClassBuild),
		task("cccc3333", "s1", 12, 5, model.ClassDoc),
	}
	marks := []store.Mark{{ID: "g1-01", Name: "사람이 정한 소단계", Tasks: []string{"aaaa1111", "cccc3333"}}}
	gs := Build(tasks, marks, markKey())
	if len(gs) != 2 {
		t.Fatalf("묶음 수 = %d, 바란 값 2", len(gs))
	}
	var marked, auto *Group
	for i := range gs {
		if gs[i].Marked {
			marked = &gs[i]
			continue
		}
		auto = &gs[i]
	}
	if marked == nil || len(marked.Tasks) != 2 {
		t.Fatalf("정본 묶음 = %+v", marked)
	}
	if auto == nil || len(auto.Tasks) != 1 || auto.Tasks[0] != "bbbb2222" {
		t.Fatalf("자동 묶음 = %+v", auto)
	}
}

// 정본에 든 작업은 자동 묶기 사슬을 끊지 않고 건너뛴다.
func TestMarkedTaskDoesNotBreakAutoChain(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 2, model.ClassBuild),
		task("bbbb2222", "s1", 5, 2, model.ClassBuild),
		task("cccc3333", "s1", 10, 2, model.ClassBuild),
	}
	marks := []store.Mark{{ID: "g1-01", Name: "가운데만", Tasks: []string{"bbbb2222"}}}
	gs := Build(tasks, marks, markKey())
	if len(gs) != 2 {
		t.Fatalf("묶음 수 = %d, 바란 값 2", len(gs))
	}
	for _, g := range gs {
		if !g.Marked && len(g.Tasks) != 2 {
			t.Fatalf("자동 묶음이 끊겼다 : %+v", g.Tasks)
		}
	}
}

// 여러 세션에 걸친 소단계는 정본에서만 된다.
func TestBuildCrossSessionMarkOnly(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 5, model.ClassBuild),
		task("bbbb2222", "s2", 100, 5, model.ClassBuild),
	}
	marks := []store.Mark{{ID: "g1-01", Name: "두 세션", Tasks: []string{"aaaa1111", "bbbb2222"}}}
	gs := Build(tasks, marks, markKey())
	if len(gs) != 1 || len(gs[0].Sessions) != 2 {
		t.Fatalf("묶음 = %+v", gs)
	}
}

// 묶음 시간은 작업 구간의 합집합이다. 작업 사이의 사람 대기는 안 든다.
func TestGroupWallIsUnion(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 5, model.ClassBuild),
		task("bbbb2222", "s1", 20, 5, model.ClassBuild),
	}
	gs := Build(tasks, nil, Key{Mode: ModeGap, Gap: 30 * time.Minute})
	if len(gs) != 1 {
		t.Fatalf("묶음 수 = %d", len(gs))
	}
	if gs[0].WallMs != 10*60000 {
		t.Fatalf("묶음 벽시계 = %d ms, 바란 값 10분 (사이 15분은 안 든다)", gs[0].WallMs)
	}
}

// 묶음 벽시계는 서브에이전트 구간도 같이 합집합한다.
func TestGroupWallIncludesAgentSpans(t *testing.T) {
	main := task("aaaa1111", "s1", 0, 10, model.ClassBuild)
	main.Agents = []model.Agent{{AgentID: "sub1", Start: at(10), End: at(30)}}
	gs := Build([]model.Task{main}, nil, Key{Mode: ModeGap, Gap: 30 * time.Minute})
	if len(gs) != 1 {
		t.Fatalf("묶음 수 = %d", len(gs))
	}
	if gs[0].WallMs != 30*60000 {
		t.Fatalf("묶음 벽시계 = %d ms, 바란 값 30분 (본줄 10분 + 갈래 20분)", gs[0].WallMs)
	}
}

// 시작·끝이 빈 갈래 구간은 벽시계에 안 든다.
func TestGroupWallSkipsEmptyAgentSpans(t *testing.T) {
	main := task("aaaa1111", "s1", 0, 10, model.ClassBuild)
	main.Agents = []model.Agent{{AgentID: "sub1"}, {AgentID: "sub2", Start: at(20)}}
	gs := Build([]model.Task{main}, nil, Key{Mode: ModeSession})
	if gs[0].WallMs != 10*60000 {
		t.Fatalf("묶음 벽시계 = %d ms, 바란 값 10분", gs[0].WallMs)
	}
}

// 묶음 분류는 일 칸 중 벽시계 합이 가장 큰 것이다.
func TestGroupClassIsBiggestWork(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 3, model.ClassDoc),
		task("bbbb2222", "s1", 4, 9, model.ClassBuild),
		task("cccc3333", "s1", 14, 20, model.ClassChat),
	}
	gs := Build(tasks, nil, Key{Mode: ModeSession})
	if gs[0].Class != model.ClassBuild {
		t.Fatalf("묶음 분류 = %s, 바란 값 구현", gs[0].Class)
	}
}

func TestBuildNoneIsOneTaskEach(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 2, model.ClassBuild),
		task("bbbb2222", "s1", 3, 2, model.ClassBuild),
	}
	gs := Build(tasks, nil, Key{Mode: ModeNone})
	if len(gs) != 2 {
		t.Fatalf("묶음 수 = %d, 바란 값 2", len(gs))
	}
}

func TestBuildClassChain(t *testing.T) {
	tasks := []model.Task{
		task("aaaa1111", "s1", 0, 2, model.ClassBuild),
		task("bbbb2222", "s1", 3, 2, model.ClassBuild),
		task("cccc3333", "s1", 6, 2, model.ClassDoc),
	}
	gs := Build(tasks, nil, Key{Mode: ModeClass})
	if len(gs) != 2 {
		t.Fatalf("묶음 수 = %d, 바란 값 2", len(gs))
	}
}
