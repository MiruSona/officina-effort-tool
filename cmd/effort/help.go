package main

import "fmt"

const helpAll = `effort — Claude Code 기록으로 공수를 재는 툴

쓰는 법 :
  effort scan     [--home DIR] [--projects DIR] [--project NAME|--all] [--rebuild] [--titles=false] [--quiet]
  effort stats    [--class C] [--since YYYY-MM-DD] [--by class|agent|model|session] [--wide|--json]
  effort estimate [소단계...] [--from FILE|-] [--human] [--metric wall|pure] [--wide|--json]
  effort show     <promptId|접두사> [--json]
  effort list     [--class C] [--since D] [--session ID] [--limit N] [--sort wall|pure|tok]
  effort rules    [--check]
  effort version | effort help [명령]

소단계 꼴 : [이름:]분류[:크기]   예) 조사:M  설계:M  시험:구현:L
분류 : 조사 설계 구현 실측 문서 검토 미분류
크기 : S M L XL

종료 코드 : 0 성공 · 1 사용법 오류 · 2 낼 것 없음 · 3 읽기 실패 · 4 쓰기·락 실패 · 5 손상 줄 과다
`

var helpTopic = map[string]string{
	"scan":     "scan : JSONL 을 훑어 작업을 캐시에 넣는다. 기본은 바뀐 파일만, --rebuild 는 통째로.\n원본 JSONL 은 읽기만 한다. 캐시는 지워도 --rebuild 로 되살아난다.",
	"stats":    "stats : 분류·에이전트·모델·세션별 건수·벽시계·순수시간·토큰을 찍는다. 캐시만 읽는다.",
	"estimate": "estimate : 소단계 목록을 받아 예상·범위 표를 낸다. 인자와 --from 은 같이 못 쓴다.",
	"show":     "show : 작업 하나를 자세히 본다 (서브에이전트·모델별 토큰·세션 검산).",
	"list":     "list : 작업을 한 줄씩 본다.",
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
