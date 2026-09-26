package main

import "fmt"

const helpAll = `effort — Claude Code 기록으로 공수를 재는 툴

쓰는 법 :
  effort scan     [--home DIR] [--projects DIR] [--project NAME|--all] [--rebuild] [--titles=false] [--quiet]
  effort stats    [--class C] [--since YYYY-MM-DD] [--by class|agent|model|session|group] [--group KEY] [--wide|--json]
  effort estimate [--from FILE|-] [--unit group|task] [--group KEY] [--human] [--metric total|wall|pure] [--wide|--json] [소단계...]
  effort show     <promptId|접두사> [--json]
  effort list     [--class C] [--since D] [--session ID] [--group KEY] [--limit N] [--sort wall|pure|tok]
  effort group    [--add <이름>] [--class C] [--drop <묶음id>] [--since D] [--session ID] [--group KEY] [작업id...]
  effort actual   --from FILE|- [--since D] [--session ID] [--group KEY] [--match name|order] [--metric total|wall|pure]
  effort rules    [--check] [--measure]
  effort mark     start "<이름>" | stop [<id|이름>] | show [<id|이름>] | list [--since D]
  effort version | effort help [명령]

소단계 꼴 : [이름:]분류[:크기]   예) 조사:M  설계:M  시험:구현:L
분류 : 조사 설계 구현 실측 문서 검토 미분류
크기 : S M L XL
묶기 열쇠(KEY) : mark(기본) · gap:30m · class · session · none

옵션은 명령 바로 뒤에 둔다. 위치 인자 뒤에 쓰면 조용히 무시되지 않고 오류로 막는다.

종료 코드 : 0 성공 · 1 사용법 오류 · 2 낼 것 없음 · 3 읽기 실패 · 4 쓰기·락 실패 · 5 손상 줄 과다
`

var helpTopic = map[string]string{
	"scan":     "scan : JSONL 을 훑어 작업을 캐시에 넣는다. 기본은 바뀐 파일만, --rebuild 는 통째로.\n원본 JSONL 은 읽기만 한다. 캐시는 지워도 --rebuild 로 되살아난다.",
	"stats":    "stats : 분류·에이전트·모델·세션별 건수·벽시계·순수시간·토큰을 찍는다. 캐시만 읽는다.",
	"estimate": "estimate : 소단계 목록을 받아 예상·범위 표를 낸다. 인자와 --from 은 같이 못 쓴다.\n옵션은 소단계 앞에 둔다 : effort estimate --human 조사:S (소단계 뒤에 쓰면 오류로 막는다).\n--human 은 --from 표의 「사람눈금」 열에 적은 분에 rules.txt 의 human 배율을 곱해 칸 하나를 더 찍는다.\n--metric 은 표본 시간 : total(기본, 본줄∪서브 · 서브는 하나당 rules.txt sub max 분까지) · wall(본줄만) · pure(턴 합).",
	"show":     "show : 작업 하나를 자세히 본다 (서브에이전트·모델별 토큰·세션 검산).",
	"list":     "list : 작업을 한 줄씩 본다. --group 을 주면 소단계 묶음을 대신 찍는다.\n--group 일 때는 모든 작업을 묶은 뒤 대표 분류가 일 칸인 묶음만 보여준다 (estimate 표본과 같다).\n그래서 --class 를 같이 주면 뜻이 「대표 분류가 그것인 묶음」으로 바뀐다. --all 은 일 아닌 묶음까지.",
	"group":    "group : 소단계 경계를 사람이 표시하고 묶음을 본다.\n정본은 ~/.effort/groups.txt 다. 캐시가 아니라 scan --rebuild 로도 안 날아간다.\n--add 는 끝에 덧붙이기만 하고, --drop 은 그 묶음 줄만 뺀다.",
	"actual":   "actual : 예상 표(--from)와 실제 묶음을 나란히 놓아 배율을 낸다.\n못 찾은 소단계는 — 로 두고 합에서 뺀다. 없는 값을 지어내지 않는다.\n--metric 은 예상·실제를 같이 재는 잣대 : total(기본, 본줄∪서브 · 서브는 묶음 하나당 rules.txt sub max 분까지) · wall(본줄만) · pure(턴 합).",
	"rules":    "rules : 분류 규칙·배율·시드를 보여준다. --check 는 문법만(모르는 종류도 오류), --measure 는 실측으로 배율·시드를 다시 잰 줄을 찍는다.\n다른 명령은 이 exe 가 모르는 종류·이름 줄을 경고하고 건너뛴다.",
	"mark":     "mark : 서브에이전트가 자기 판을 잰다. 시각만 exe 시계로 찍고, 값은 그 판의 기록 파일에서 두 시각 사이 본줄 구간으로 잰다.\n  effort mark start \"<이름>\"        시작. 이름은 판마다 다르게 (예 2-타일그림). 자기 기록 파일을 찾아 묶는다\n  effort mark stop  [<id|이름>]      끝. 잰 값을 바로 찍는다 — 「기록 구간」을 공수 표 「실제」 칸에 쓴다\n  effort mark show  [<id|이름>]      안 닫힌 mark 는 지금까지 값 (진행중)\n  effort mark list  [--since D]      찍은 mark 목록\n시각·길이를 받는 옵션은 없다. 정본은 ~/.effort/marks.txt (덧붙이기만 · scan --rebuild 로 안 지워진다).\n표시 : 진행중 · 끝자동(stop 없이 파일이 끝남) · 묶기없음 · 묶기모호(찍은 구간만) · 어긋남(두 구간이 20% 넘게 다름) · 상한넘음(sub max 초과).",
}

func printHelp(topic string) {
	if topic == "" {
		fmt.Print(helpAll)
		return
	}
	if t, ok := helpTopic[topic]; ok {
		fmt.Println(t)
		return
	}
	fmt.Print(helpAll)
}
