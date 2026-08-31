package classify

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mirusona/efforttool/internal/model"
)

// Seed 는 표본이 모자랄 때 쓰는 사람 눈금이다 (분).
type Seed struct {
	P50 float64
	P20 float64
	P80 float64
}

// Rules 는 rules.txt 한 벌이다.
type Rules struct {
	Words     map[string]model.Class // 낱말 → 분류
	WordOrder []string               // 긴 낱말부터 보려고 차례를 기억한다
	Agents    map[string]model.Class // agentType → 분류
	Sizes     map[string]float64
	Seeds     map[model.Class]Seed
	Human     map[model.Class]float64
	MinSample int
	BlendMax  int
}

func newRules() *Rules {
	return &Rules{
		Words:     map[string]model.Class{},
		Agents:    map[string]model.Class{},
		Sizes:     map[string]float64{},
		Seeds:     map[model.Class]Seed{},
		Human:     map[model.Class]float64{},
		MinSample: 5,
		BlendMax:  12,
	}
}

// DefaultRulesText 는 rules.txt 가 없을 때 처음 한 번 만들어 주는 내용이다.
const DefaultRulesText = `# effort 분류 규칙. 탭으로 나눈다. # 은 주석.
# word   <분류>  <낱말>        설명 끝말·포함 낱말
# agent  <분류>  <agentType>
# size   <크기>  <배율>
# seed   <분류>  <p50분> <p20분> <p80분>
# human  <분류>  <배율>        사람 눈금 × 배율
# min    sample  <수>          이 아래면 시드만 쓴다
# blend  sample  <수>          이 위면 실측만 쓴다

word	조사	조사
word	조사	확인
word	조사	알아보기
word	조사	찾기
word	조사	탐색
word	조사	훑기
word	조사	research
word	설계	설계
word	설계	계획
word	설계	기획
word	설계	방안
word	설계	design
word	설계	plan
word	구현	구현
word	구현	만들기
word	구현	고침
word	구현	고치기
word	구현	수정
word	구현	추가
word	구현	붙이기
word	구현	코드
word	구현	리팩터
word	실측	실측
word	실측	측정
word	실측	재기
word	실측	벤치
word	실측	성능
word	실측	돌려보기
word	문서	문서
word	문서	정리
word	문서	기록
word	문서	보고
word	문서	README
word	문서	가이드
word	문서	안내
word	검토	검토
word	검토	리뷰
word	검토	점검
word	검토	감사
word	검토	review

agent	조사	Explore
agent	설계	Plan
agent	검토	superpowers:code-reviewer

size	S	0.6
size	M	1.0
size	L	1.8
size	XL	3.2

seed	조사	10	5	20
seed	설계	20	6	50
seed	구현	20	8	60
seed	실측	45	20	180
seed	문서	8	3	20
seed	검토	10	5	25
seed	미분류	15	5	40

human	조사	0.7
human	설계	0.7
human	구현	0.25
human	실측	1.0
human	문서	0.5
human	검토	0.5

min	sample	5
blend	sample	12
`

// ParseRules 는 rules.txt 를 읽는다. 모르는 종류가 나오면 줄 번호와 함께 실패한다.
func ParseRules(r io.Reader) (*Rules, error) {
	out := newRules()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := splitFields(line)
		if err := applyRuleLine(out, f, n); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for w := range out.Words {
		out.WordOrder = append(out.WordOrder, w)
	}
	sortByLenDesc(out.WordOrder)
	return out, nil
}

func applyRuleLine(out *Rules, f []string, n int) error {
	if len(f) < 3 {
		return fmt.Errorf("rules.txt %d번째 줄: 칸이 모자랍니다 (탭으로 세 칸 이상)", n)
	}
	switch f[0] {
	case "word":
		return addClassKey(out.Words, f[1], f[2], n)
	case "agent":
		return addClassKey(out.Agents, f[1], f[2], n)
	case "size":
		v, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return fmt.Errorf("rules.txt %d번째 줄: 배율이 숫자가 아닙니다 (%s)", n, f[2])
		}
		out.Sizes[strings.ToUpper(f[1])] = v
		return nil
	case "seed":
		return addSeed(out, f, n)
	case "human":
		v, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return fmt.Errorf("rules.txt %d번째 줄: 배율이 숫자가 아닙니다 (%s)", n, f[2])
		}
		if !model.IsClass(f[1]) {
			return fmt.Errorf("rules.txt %d번째 줄: 모르는 분류 %q", n, f[1])
		}
		out.Human[model.Class(f[1])] = v
		return nil
	case "min":
		return setSampleBound(&out.MinSample, f, n)
	case "blend":
		return setSampleBound(&out.BlendMax, f, n)
	}
	return fmt.Errorf("rules.txt %d번째 줄: 모르는 종류 %q", n, f[0])
}

func addClassKey(m map[string]model.Class, class, key string, n int) error {
	if !model.IsClass(class) {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 분류 %q", n, class)
	}
	m[key] = model.Class(class)
	return nil
}

func addSeed(out *Rules, f []string, n int) error {
	if len(f) < 5 {
		return fmt.Errorf("rules.txt %d번째 줄: seed 는 분류·p50·p20·p80 네 칸이 필요합니다", n)
	}
	if !model.IsClass(f[1]) {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 분류 %q", n, f[1])
	}
	vals := make([]float64, 3)
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(f[2+i], 64)
		if err != nil {
			return fmt.Errorf("rules.txt %d번째 줄: 분 값이 숫자가 아닙니다 (%s)", n, f[2+i])
		}
		vals[i] = v
	}
	out.Seeds[model.Class(f[1])] = Seed{P50: vals[0], P20: vals[1], P80: vals[2]}
	return nil
}

func setSampleBound(dst *int, f []string, n int) error {
	if f[1] != "sample" {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 이름 %q (sample 만 됩니다)", n, f[1])
	}
	v, err := strconv.Atoi(f[2])
	if err != nil {
		return fmt.Errorf("rules.txt %d번째 줄: 수가 아닙니다 (%s)", n, f[2])
	}
	*dst = v
	return nil
}

func splitFields(line string) []string {
	parts := strings.Split(line, "\t")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "#") {
			break
		}
		out = append(out, p)
	}
	return out
}

func sortByLenDesc(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && len([]rune(v[j])) > len([]rune(v[j-1])); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// Size 는 크기 배율이다. 모르면 1.0.
func (r *Rules) Size(name string) float64 {
	if v, ok := r.Sizes[strings.ToUpper(name)]; ok {
		return v
	}
	return 1.0
}
