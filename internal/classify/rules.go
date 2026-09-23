package classify

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// Seed 는 표본이 모자랄 때 쓰는 사람 눈금이다 (분).
type Seed struct {
	P50 float64
	P20 float64
	P80 float64
}

// 도구 집합 이름. 넷뿐이고 늘리려면 코드를 고쳐야 한다 (설계 3-3).
const (
	SetWrite = "쓰기"
	SetWeb   = "웹"
	SetRead  = "읽기"
	SetShell = "셸"
)

var toolSetNames = []string{SetWrite, SetWeb, SetRead, SetShell}

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

	ToolSets  map[string]map[string]bool // 집합 이름 → 도구 이름
	Prefixes  []string                   // 도구실행 접두
	Contains  []string                   // 도구실행 포함낱말
	ChoreWord []string                   // 잡무 낱말
	ShellMax  int                        // 한 줄 셸 대행으로 볼 호출 상한

	ContFirst []string // 앞말 이어짐 글머리 낱말
	ContStop  []string // 이어짐을 끊는 낱말
	ContMax   int      // 앞 작업 끝에서 이 분 안쪽일 때만 이어짐
	GapMax    int      // 자동 소단계 묶기의 간격 컷 (분)
	SubMax    int      // estimate 표본에서 서브 구간이 묶음·작업 하나에 더할 수 있는 상한 (분)
	subSeen   bool     // rules.txt 에 sub 줄이 있었나 (빠진 종류 알림용)
}

func newRules() *Rules {
	r := &Rules{
		Words:     map[string]model.Class{},
		Agents:    map[string]model.Class{},
		Sizes:     map[string]float64{},
		Seeds:     map[model.Class]Seed{},
		Human:     map[model.Class]float64{},
		MinSample: 5,
		BlendMax:  12,
		ToolSets:  map[string]map[string]bool{},
		ShellMax:  2,
		ContMax:   10,
		GapMax:    30,
		SubMax:    120,
	}
	for _, n := range toolSetNames {
		r.ToolSets[n] = map[string]bool{}
	}
	return r
}

// 자동으로 채워 줄 수 있는 규칙 종류 이름. 그대로 사람에게도 보여 준다.
const (
	KindTool  = "tool"
	KindRun   = "runpre·runin"
	KindChore = "chore"
	KindCont  = "contfirst·contstop·cont·gap"
	KindSub   = "sub"
)

// KindOrder 는 종류를 보여 주고 덧붙이는 차례다.
var KindOrder = []string{KindTool, KindRun, KindChore, KindCont, KindSub}

// MissingKinds 는 rules.txt 에 아예 없는 새 규칙 종류를 알려 준다.
// rules.txt 는 사람의 정본이라 판이 올라도 안 덮으므로, 옛 파일을 쓰면 규칙이 조용히 꺼진다.
func (r *Rules) MissingKinds() []string {
	var out []string
	empty := true
	for _, n := range toolSetNames {
		if len(r.ToolSets[n]) > 0 {
			empty = false
		}
	}
	if empty {
		out = append(out, KindTool)
	}
	if len(r.Prefixes) == 0 && len(r.Contains) == 0 {
		out = append(out, KindRun)
	}
	if len(r.ChoreWord) == 0 {
		out = append(out, KindChore)
	}
	if len(r.ContFirst) == 0 && len(r.ContStop) == 0 {
		out = append(out, KindCont)
	}
	if !r.subSeen {
		out = append(out, KindSub)
	}
	return out
}

// KindDefault 는 종류 하나의 기본 줄 덩어리다. 모르는 종류면 빈 글을 준다.
func KindDefault(kind string) string {
	return kindDefaults[kind]
}

// Set 은 도구 집합 하나다. 없는 이름이면 빈 집합을 준다.
func (r *Rules) Set(name string) map[string]bool {
	if s, ok := r.ToolSets[name]; ok {
		return s
	}
	return map[string]bool{}
}

// DefaultRulesText 는 rules.txt 가 없을 때 처음 한 번 만들어 주는 내용이다.
// 종류별 덩어리는 kindDefaults 에서 가져다 쓰므로 「빠진 종류 덧붙이기」와 어긋날 수 없다.
var DefaultRulesText = defaultHead + defaultToolLines + "\n" + defaultRunLines + "\n" +
	defaultChoreLines + "\n" + defaultContLines + "\n" + defaultSubLines + "\n" + defaultTail

// kindDefaults 는 그 종류가 rules.txt 에 한 줄도 없을 때 더해 줄 기본 줄이다.
var kindDefaults = map[string]string{
	KindTool:  defaultToolLines,
	KindRun:   defaultRunLines,
	KindChore: defaultChoreLines,
	KindCont:  defaultContLines,
	KindSub:   defaultSubLines,
}

const defaultToolLines = `tool	쓰기	Write
tool	쓰기	Edit
tool	쓰기	NotebookEdit
tool	쓰기	Artifact
tool	웹	WebSearch
tool	웹	WebFetch
tool	읽기	Read
tool	읽기	Grep
tool	읽기	Glob
tool	읽기	ToolSearch
tool	셸	Bash
tool	셸	PowerShell
`

const defaultRunLines = `runpre	‹bash-input›
runpre	<bash-input>
runpre	‹command-message›
runpre	<command-message>
runpre	‹local-command-caveat›
runpre	<local-command-caveat>
runpre	## Context Usage
runpre	/
runpre	Agent 도구로
runin	loop wakeup
`

// defaultSubLines 는 estimate 표본의 서브 구간 상한이다. 승인 대기로 몇 시간 산 갈래가 p80 을 끌어올리는 것을 막는다.
const defaultSubLines = `sub	max	120
`

const defaultChoreLines = `chore	커밋
chore	푸시
chore	commit
chore	push
chore	stash
chore	머지
chore	merge
chore	rebase
`

const defaultContLines = `contfirst	좋아
contfirst	그럼
contfirst	그러면
contfirst	아하
contfirst	일단
contfirst	오케
contfirst	그래
contfirst	넵
contfirst	이어서
contfirst	계속
contfirst	그리고
contfirst	1.
contfirst	2.
contfirst	3.
contfirst	지금
contfirst	이야
contfirst	어어
contfirst	으음
contfirst	그..
contfirst	그…
contfirst	엇
contfirst	앗
contfirst	엥
contfirst	엣
contfirst	아
contfirst	어
contfirst	오
contfirst	음
contstop	다른 에이전트
contstop	다른쪽
contstop	새 세션
contstop	새세션
contstop	궁금
contstop	혹시
contstop	하려고
contstop	문제가
cont	max	10
gap	max	30
`

const defaultHead = `# effort 분류 규칙. 탭으로 나눈다. # 은 주석.
# word   <분류>  <낱말>        설명 끝말·포함 낱말
# agent  <분류>  <agentType>
# size   <크기>  <배율>
# seed   <분류>  <p50분> <p20분> <p80분>
# human  <분류>  <배율>        사람 눈금 × 배율
# min    sample  <수>          이 아래면 시드만 쓴다
# blend  sample  <수>          이 위면 실측만 쓴다
# tool   <집합>  <도구이름>    집합은 쓰기·웹·읽기·셸 넷뿐
# runpre <접두>                제목이 이것으로 시작하면 도구실행
# runin  <낱말>                제목에 이것이 들어 있으면 도구실행
# chore  <낱말>                제목에 이것이 있고 쓰기 도구가 없으면 잡무
# shell  max     <수>          이 수 이하 셸 호출만 도구실행으로 본다
# contfirst <낱말>             제목이 이것으로 시작하면 앞 일을 이어가는 말로 본다
# contstop  <낱말>             제목에 이것이 있으면 이어가지 않는다 (새 주제·잡담)
# cont      max     <분>       앞 작업 끝에서 이 분 안쪽일 때만 이어간다
# gap       max     <분>       자동 소단계 묶기의 간격 컷
# sub       max     <분>       estimate 표본에서 서브에이전트 구간이 묶음·작업 하나에 더할 수 있는 상한

word	조사	조사
word	조사	확인
word	조사	알아보기
word	조사	찾기
word	조사	탐색
word	조사	훑기
word	조사	research
word	조사	남은일
word	조사	남은 작업
word	조사	남은 할일
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
word	실측	goldenset
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
word	도구실행	모니터링
word	도구실행	mem search
word	도구실행	mem index
word	도구실행	mem eval
word	도구실행	mem status
word	도구실행	compact
word	대화	고마워
word	대화	알겠어

agent	조사	Explore
agent	설계	Plan
agent	검토	superpowers:code-reviewer

`

const defaultTail = `shell	max	2

# 2026-09-04 실측 — 일 칸 묶음 252건의 벽시계(본줄만) 분위수
# p20 0.7 · p50 1.9 · p80 4.5 · p95 12.4 분. effort rules --measure 로 다시 낼 수 있다.
size	S	0.35
size	M	1.0
size	L	2.35
size	XL	6.49

# 2026-09-04 실측 (분) — 표본 12건 이상인 분류만 새 값. 검토·미분류는 옛 짐작값 그대로.
seed	조사	2	1	4
seed	설계	3	1	4
seed	구현	2	1	5
seed	실측	2	1	6
seed	문서	1	1	4
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
	// 섞기 구간이 뒤집혀 있으면 실측을 쓸 구간이 사라진다. 두 줄이 다 있을 때만 본다.
	if out.MinSample > 0 && out.BlendMax > 0 && out.BlendMax <= out.MinSample {
		return nil, fmt.Errorf("rules.txt : blend sample(%d) 은 min sample(%d) 보다 커야 합니다",
			out.BlendMax, out.MinSample)
	}
	for w := range out.Words {
		out.WordOrder = append(out.WordOrder, w)
	}
	sortByLenDesc(out.WordOrder)
	return out, nil
}

// 종류별로 있어야 하는 최소 칸수. runpre·runin·chore 는 값이 하나뿐이다.
var minFields = map[string]int{
	"word": 3, "agent": 3, "size": 3, "seed": 5, "human": 3,
	"min": 3, "blend": 3, "tool": 3, "shell": 3,
	"runpre": 2, "runin": 2, "chore": 2,
	"contfirst": 2, "contstop": 2, "cont": 3, "gap": 3, "sub": 3,
}

func applyRuleLine(out *Rules, f []string, n int) error {
	if len(f) == 0 {
		return nil
	}
	need, ok := minFields[f[0]]
	if !ok {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 종류 %q", n, f[0])
	}
	if len(f) < need {
		return fmt.Errorf("rules.txt %d번째 줄: 칸이 모자랍니다 (탭으로 %d칸 이상)", n, need)
	}
	// 종류가 쓰는 칸까지만 본다. 뒤에 붙은 것은 주석이다.
	f = f[:need]
	switch f[0] {
	case "word":
		return addClassKey(out.Words, f[1], f[2], n)
	case "agent":
		return addClassKey(out.Agents, f[1], f[2], n)
	case "tool":
		return addToolName(out, f[1], f[2], n)
	case "runpre":
		out.Prefixes = append(out.Prefixes, f[1])
		return nil
	case "runin":
		out.Contains = append(out.Contains, f[1])
		return nil
	case "chore":
		out.ChoreWord = append(out.ChoreWord, f[1])
		return nil
	case "contfirst":
		out.ContFirst = append(out.ContFirst, f[1])
		return nil
	case "contstop":
		out.ContStop = append(out.ContStop, f[1])
		return nil
	case "shell":
		return setNamedInt(&out.ShellMax, f, "max", n)
	case "cont":
		return setNamedInt(&out.ContMax, f, "max", n)
	case "gap":
		return setNamedInt(&out.GapMax, f, "max", n)
	case "sub":
		if err := setNamedInt(&out.SubMax, f, "max", n); err != nil {
			return err
		}
		// 0 이면 서브 구간이 통째로 빠져 옛 본줄 표본과 조용히 같아진다. 끄려면 --metric wall 을 쓴다.
		if out.SubMax < 1 {
			return fmt.Errorf("rules.txt %d번째 줄: sub max 는 1 이상이어야 합니다 (%s)", n, f[2])
		}
		out.subSeen = true
		return nil
	case "size":
		v, err := strconv.ParseFloat(f[2], 64)
		if err != nil {
			return fmt.Errorf("rules.txt %d번째 줄: 배율이 숫자가 아닙니다 (%s)", n, f[2])
		}
		// 0 이나 음수를 두면 추정값이 통째로 0 이 되어 조용히 틀린다.
		if v <= 0 {
			return fmt.Errorf("rules.txt %d번째 줄: 배율은 0보다 커야 합니다 (%s)", n, f[2])
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
		if err := setNamedInt(&out.MinSample, f, "sample", n); err != nil {
			return err
		}
		if out.MinSample < 1 {
			return fmt.Errorf("rules.txt %d번째 줄: min sample 은 1 이상이어야 합니다 (%s)", n, f[2])
		}
		return nil
	case "blend":
		if err := setNamedInt(&out.BlendMax, f, "sample", n); err != nil {
			return err
		}
		if out.BlendMax < 1 {
			return fmt.Errorf("rules.txt %d번째 줄: blend sample 은 1 이상이어야 합니다 (%s)", n, f[2])
		}
		return nil
	}
	return fmt.Errorf("rules.txt %d번째 줄: 모르는 종류 %q", n, f[0])
}

// addToolName 은 도구 집합에 이름 하나를 넣는다. 집합 이름은 넷뿐이다.
func addToolName(out *Rules, set, name string, n int) error {
	s, ok := out.ToolSets[set]
	if !ok {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 도구 집합 %q (쓰기·웹·읽기·셸 만 됩니다)", n, set)
	}
	s[name] = true
	return nil
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
		if v < 0 {
			return fmt.Errorf("rules.txt %d번째 줄: 분 값은 0 이상이어야 합니다 (%s)", n, f[2+i])
		}
		vals[i] = v
	}
	out.Seeds[model.Class(f[1])] = Seed{P50: vals[0], P20: vals[1], P80: vals[2]}
	return nil
}

func setNamedInt(dst *int, f []string, want string, n int) error {
	if f[1] != want {
		return fmt.Errorf("rules.txt %d번째 줄: 모르는 이름 %q (%s 만 됩니다)", n, f[1], want)
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
		out = append(out, p)
	}
	return out
}

// sortByLenDesc 는 긴 낱말부터 보게 차례를 잡는다.
// 길이가 같으면 글자 차례로 가른다 — 열쇠가 map 이라, 안 가르면 돌릴 때마다 분류가 달라진다.
func sortByLenDesc(v []string) {
	sort.Slice(v, func(i, j int) bool {
		li, lj := len([]rune(v[i])), len([]rune(v[j]))
		if li != lj {
			return li > lj
		}
		return v[i] < v[j]
	})
}

// Size 는 크기 배율이다. 모르면 1.0.
// 모르는 이름을 조용히 삼키지 않으려면 부르기 전에 HasSize 로 본다.
func (r *Rules) Size(name string) float64 {
	if v, ok := r.Sizes[strings.ToUpper(name)]; ok {
		return v
	}
	return 1.0
}

// HasSize 는 아는 크기 이름인지다.
func (r *Rules) HasSize(name string) bool {
	_, ok := r.Sizes[strings.ToUpper(name)]
	return ok
}

// SizeNames 는 아는 크기 이름을 배율 오름차순으로 준다.
func (r *Rules) SizeNames() []string {
	out := make([]string, 0, len(r.Sizes))
	for n := range r.Sizes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return r.Sizes[out[i]] < r.Sizes[out[j]] })
	return out
}
