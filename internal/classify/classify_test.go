package classify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirusona/efforttool/internal/model"
)

func defaultRules(t *testing.T) *Rules {
	t.Helper()
	r, err := ParseRules(strings.NewReader(DefaultRulesText))
	if err != nil {
		t.Fatalf("기본 규칙을 못 읽었다 : %v", err)
	}
	return r
}

func TestRulesParseOK(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("../../testdata/rules/ok.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := ParseRules(f)
	if err != nil {
		t.Fatalf("ParseRules : %v", err)
	}
	if r.Words["조사"] != model.ClassResearch {
		t.Fatalf("낱말을 못 읽었다 : %+v", r.Words)
	}
	if r.Size("L") != 1.8 || r.Size("없는크기") != 1.0 {
		t.Fatalf("배율을 못 읽었다")
	}
	if r.Seeds[model.ClassResearch].P80 != 20 {
		t.Fatalf("시드를 못 읽었다 : %+v", r.Seeds)
	}
}

func TestRulesParseUnknownKeyFails(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("../../testdata/rules/bad.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = ParseRules(f)
	if err == nil {
		t.Fatal("모르는 종류인데 안 막았다")
	}
	if !strings.Contains(err.Error(), "2번째 줄") {
		t.Fatalf("줄 번호가 없다 : %v", err)
	}
}

func TestDefaultRulesParse(t *testing.T) {
	r := defaultRules(t)
	// 시드가 필요한 것은 일 칸 여섯과 미분류다. 일 아닌 칸은 예상 표본에 안 들어가 시드도 없다.
	if len(r.Seeds) != len(model.WorkClasses)+1 {
		t.Fatalf("시드 수 = %d", len(r.Seeds))
	}
	if r.MinSample != 5 || r.BlendMax != 12 {
		t.Fatalf("표본 경계 = %d / %d", r.MinSample, r.BlendMax)
	}
}

// 실측으로 다시 잡은 크기 배율이다. 어림값으로 되돌아가면 여기서 걸린다.
func TestDefaultSizeMultipliers(t *testing.T) {
	r := defaultRules(t)
	want := map[string]float64{"S": 0.3, "M": 1.0, "L": 2.4, "XL": 5.0}
	for name, v := range want {
		if r.Size(name) != v {
			t.Fatalf("%s 배율 = %v, 바란 값 %v", name, r.Size(name), v)
		}
	}
}

func TestClassifyBySuffix(t *testing.T) {
	r := defaultRules(t)
	a := model.Agent{Description: "타일 자료 구조 조사", AgentType: "general-purpose"}
	c, by := r.Agent(&a)
	if c != model.ClassResearch || by != BySuffix {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyByAgentType(t *testing.T) {
	r := defaultRules(t)
	a := model.Agent{Description: "아무 낱말도 안 맞는 문장", AgentType: "Explore"}
	c, by := r.Agent(&a)
	if c != model.ClassResearch || by != ByAgentType {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyTaskTakesBiggestAgent(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Agents: []model.Agent{
		{Description: "짧은 조사", Usage: model.ModelUsage{"m": {Output: 1000}}},
		{Description: "긴 코드 구현", Usage: model.ModelUsage{"m": {Output: 50000}}},
	}}
	c, _ := r.Task(&task)
	if c != model.ClassBuild {
		t.Fatalf("분류 = %s, 바란 값 구현", c)
	}
}

func TestClassifyByTitleWhenNoAgent(t *testing.T) {
	r := defaultRules(t)
	task := model.Task{Title: "공수 측정 도구 설계"}
	c, by := r.Task(&task)
	if c != model.ClassDesign || by != ByTitle {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}

func TestClassifyFallsBackToUnknown(t *testing.T) {
	r := defaultRules(t)
	// 도구를 여러 번 썼는데 낱말이 하나도 안 맞으면 사람이 볼 몫이라 미분류다.
	task := model.Task{Title: "zzz qqq", Tools: map[string]int{"Bash": 5}}
	c, by := r.Task(&task)
	if c != model.ClassUnknown || by != ByDefault {
		t.Fatalf("분류 = %s (%s)", c, by)
	}
}
