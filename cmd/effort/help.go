package main

import "fmt"

const helpAll = `effort — Claude Code 기록으로 공수를 재는 툴

쓰는 법 :
  effort scan     [--home DIR] [--projects DIR] [--project NAME|--all] [--rebuild] [--titles=false] [--quiet]
  effort stats    [--class C] [--since YYYY-MM-DD] [--by class|agent|model|session|group] [--group KEY] [--wide|--json]
  effort estimate [소단계...] [--from FILE|-] [--unit group|task] [--group KEY] [--human] [--metric wall|pure] [--wide|--json]
  effort show     <promptId|접두사> [--json]
  effort list     [--class C] [--since D] [--session ID] [--group KEY] [--limit N] [--sort wall|pure|tok]
  effort group    [--add <이름>] [--class C] [--drop <묶음id>] [--since D] [--session ID] [--group KEY] [작업id...]
  effort actual   --from FILE|- [--since D] [--session ID] [--group KEY] [--match name|order] [--metric wall|pure]
  effort rules    [--check]
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
	"estimate": "estimate : 소단계 목록을 받아 예상·범위 표를 낸다. 인자와 --from 은 같이 못 쓴다.",
	"show":     "show : 작업 하나를 자세히 본다 (서브에이전트·모델별 토큰·세션 검산).",
	"list":     "list : 작업을 한 줄씩 본다. --group 을 주면 소단계 묶음을 대신 찍는다.",
	"group":    "group : 소단계 경계를 사람이 표시하고 묶음을 본다.\n정본은 ~/.effort/groups.txt 다. 캐시가 아니라 scan --rebuild 로도 안 날아간다.\n--add 는 끝에 덧붙이기만 하고, --drop 은 그 묶음 줄만 뺀다.",
	"actual":   "actual : 예상 표(--from)와 실제 묶음을 나란히 놓아 배율을 낸다.\n못 찾은 소단계는 — 로 두고 합에서 뺀다. 없는 값을 지어내지 않는다.",
	"rules":    "rules : 분류 규칙·배율·시드를 보여준다. --check 는 문법만 본다.",
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
