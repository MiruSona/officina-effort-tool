---
name: effort-usage
description: Use when estimating effort before starting work, filling the 실제 column of a 공수 표, or writing a completion report with time numbers — 공수를 잡을 때 · 완료 보고에 실제 시간을 적을 때 · 소단계 표를 만들 때 EffortTool 로 재는 법.
---

# 공수는 EffortTool 로 잰다

**시간 숫자의 정본은 실측이다.** 어림하지 않는다. 옵션의 자세한 꼴은 `effort help <명령>`,
분류·셈법은 EffortTool 의 `README.md`.

## 어디서 부르나 — 명령 세 벌

| 어디서 쓰나 | 명령 앞자리 |
| --- | --- |
| 스튜디오(Officina) 저장소에서 | `.\EffortTool\bin\effort.exe …` |
| EffortTool 저장소 단독에서 | `.\bin\effort.exe …` |
| 서브모듈로 붙인 저장소에서 | `.\Tools\EffortTool\bin\effort.exe …` |

`bin/` 은 git 에 안 들어간다. **서브모듈을 당긴 뒤에는 그 폴더에서 `.\build.ps1` 로 다시 빌드한다**
(옛 exe 는 시각을 UTC 로 찍고 옛 버그를 그대로 갖는다).

아래 표와 예시는 짧은 쪽(`effort …`)으로 적는다. 실제로 칠 때는 위 표의 앞자리를 붙인다.

## 서브에이전트가 한 일은 누가 재나

**서브에이전트는 자기 판을 못 잰다.** 도는 동안에는 그 세션 기록에 `cost-state` 줄이 없고,
`scan` 은 그런 세션을 「진행 중」으로 보아 마지막 작업에 `cost-state없음` 표시를 달아 표본에서 뺀다.
**판이 끝난 뒤 메인 세션이 잰다** — `effort scan` → `effort list --since <날짜>` → `effort show <id>`.

- 서브 구간은 `show` 의 **「서브」 칸**과 **에이전트 표의 「벽시계」** 에 나오고 `show`·`stats`·`list` 총계에는 안 든다.
  `estimate`·`actual` 의 기본(`--metric total`)만 서브 몫을 묶음 하나당 `sub max` 분까지 넣는다.
  공수 표에는 **「서브(참고) N분」** 으로 출처를 밝혀 적는다.
- **시간을 손으로 적어 넣는 길은 없다** (`group --add` 는 경계 표시일 뿐이고 `actual` 은 잰 값만 읽는다).
  값이 없으면 지어내지 말고 **「못 쟀다」**고 적는다.

## 언제 무엇을

| 이럴 때 | 이렇게 |
| --- | --- |
| 작업 시작 전, 소단계 표의 **예상** 칸 | `effort estimate 조사:M 설계:M 구현:L` — **묶음(소단계) 단위** 분류별 p50 × 크기 배율. 작업 하나씩 보려면 `--unit task` |
| 소단계가 여럿이라 표로 주고 싶을 때 | 파이프 표를 `--from -` 으로 먹인다 (열 : 소단계·분류·크기) |
| 일하는 동안 **소단계 경계 표시** | `effort group --add "1. 벽시계 고치기" 7bcf9a45 31ee27e1` |
| 소단계가 끝나 **실제** 칸을 채울 때 | `effort scan` 뒤 `effort actual --from 공수표.md --since <날짜>` 로 예상과 실제를 나란히 본다 |
| 작업 하나를 자세히 | `effort show <작업id앞자리>` — 모델별 토큰·서브에이전트·검산까지 |
| 세션 통째 걸린 시간 | `effort stats --by session --since <날짜>` |
| 무슨 일에 시간이 쏠렸나 | `effort stats --by class` |

```powershell
# 스튜디오 저장소에서
.\EffortTool\bin\effort.exe scan --all
.\EffortTool\bin\effort.exe estimate 조사:M 설계:M 구현:L

# EffortTool 저장소 단독에서
.\bin\effort.exe scan --all
.\bin\effort.exe estimate 조사:M 설계:M 구현:L
```

실행 파일이 없으면 EffortTool 폴더 안에서 `.\build.ps1` 을 돌린다.

`estimate` 가 내는 표는 그대로 보고의 공수 표에 붙는다. **옵션은 명령 바로 뒤**,
소단계·작업 id 는 맨 끝에 둔다 (`effort estimate --human 조사:S`).
**어기면 바른 차례를 알려 주며 오류로 막는다.**
`--human` 은 `--from` 표의 「사람눈금」 열에 분을 적은 줄에만 찬다 — 인자 꼴에서는 `—` 로 남는다.

## 숫자의 뜻 (2026-09-04)

- **벽시계** = **본줄 구간만**. 큐·파일이력 같은 곁줄은 시간을 못 늘린다.
  서브에이전트 구간은 `show`·`list --group` 의 「서브(참고)」 칸에만 나오고 총계에 안 든다.
- **순수시간** = 턴 시간의 합. 벽시계를 넘으면 벽시계로 자르고 `순수시간잘림` 을 남긴다.
- **묶음 시간**에는 작업과 작업 **사이의 사람 대기가 안 든다**. 지금은 묶음 안 작업의 본줄 구간만 더한다.

## rules.txt 는 언제 손대나

- 코드가 새 규칙 종류를 더한 경우는 `scan` 이 빠진 종류의 기본 줄을 알아서 덧붙인다. 할 일 없다.
- **낱말을 손으로 고쳤어도 그냥 `effort scan`** 이면 된다 — scan 이 `rules.txt` 지문을 견줘
  바뀌었으면 캐시 분류를 알아서 통째로 다시 매긴다 (`--rebuild` 는 이제 캐시가 수상할 때만).
- `groups.txt`(소단계 정본)는 캐시가 아니라 **`--rebuild` 해도 안 날아간다.**
- `list --group` 은 **`estimate` 와 같은 표본**을 본다 — 모든 작업을 묶은 뒤 대표 분류가 일 칸인 묶음만.
  그래서 `--class 조사 --group gap:30m` 의 묶음 수가 `estimate --unit group 조사:M` 의 `n` 과 같다.
- 크기 배율(**S 0.35 · M 1.0 · L 2.35 · XL 6.49**)은 2026-09-04 **묶음 단위** 실측(일 칸 묶음 252건,
  본줄만)으로 잡은 값이다. 다시 재려면 `effort rules --measure` — 붙여 넣을 `size`·`seed` 줄을 찍어 준다
  (파일은 안 고친다).

## 알아 둘 한계 둘

1. **소단계는 `effort group --add` 로 사람이 표시**한다. 표시가 없는 자리는 세션 안 30분 간격으로
   자동으로 묶는다 (컷은 `rules.txt` 의 `gap max`).
2. **벽시계는 사람이 기다린 시간도, 서브에이전트가 돈 시간도 안 센다**(2026-09-04).
   남는 것은 **본줄이 실제로 움직인 시간**뿐이다. 눈금 규칙을 쓸 자리는 **처음 해 보는 실측**뿐이다 —
   「값을 재서 정한다」는 ×2, 시그니처까지 정한 설계가 있으면 ÷4.

> **`estimate` 는 기본으로 본줄 ∪ 서브에이전트 구간을 표본으로 쓴다**(`--metric total`, 2026-09-23).
> 서브 몫은 묶음 하나당 `rules.txt` 의 `sub max`(기본 120분)까지만 더한다. 그래서 `구현:L` 이 10분쯤,
> `실측:S` 가 11분쯤으로 나온다 — **서브에이전트 한 판의 AI 시간** 자릿수다.
> 그래도 **사람 시간은 아니다** — 메인 쪽 사람 대기는 안 들지만 서브 구간 안의 대기(승인 등)는 상한까지 든다. 사람 시간 눈금은 `actual` 로
> 회차를 15~20건 쌓아 `human` 배율을 잡은 뒤에야 나온다. 옛 본줄만 값은 `--metric wall`.
> `actual` 도 기본이 `total` 이라 예상·실제가 같은 잣대다(`--metric wall` 이면 둘 다 본줄만).
> `show`·`stats`·`list` 의 벽시계는 여전히 **본줄만**이다 — 두 값을 섞어 견주지 않는다.

## 끝나고 남기는 것

숫자는 툴이 갖고 있으니 파일로 안 남긴다. **예상과 실제가 왜 어긋났나 · 다음에 쓸 눈금**만
`mem` 에 남긴다 (`mem-usage` 스킬 — `--type history`, 눈금이면 `howto`).
지난 눈금은 `mem search 눈금 공수` 로 먼저 본다 (배율 `20260831-d485b6c9`).
