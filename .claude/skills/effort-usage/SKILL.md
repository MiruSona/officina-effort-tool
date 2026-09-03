---
name: effort-usage
description: Use when estimating effort before starting work, filling the 실제 column of a 공수 표, or writing a completion report with time numbers — 공수를 잡을 때 · 완료 보고에 실제 시간을 적을 때 · 소단계 표를 만들 때 EffortTool 로 재는 법.
---

# 공수는 EffortTool 로 잰다

**시간 숫자의 정본은 실측이다.** 어림하지 않는다. 옵션의 자세한 꼴은 `effort help <명령>`,
분류·셈법은 EffortTool 의 `README.md`.

## 어디서 부르나 — 명령 두 벌

| 어디서 쓰나 | 명령 앞자리 |
| --- | --- |
| 스튜디오(Officina) 저장소에서 | `.\EffortTool\bin\effort.exe …` |
| EffortTool 저장소 단독에서 | `.\bin\effort.exe …` |

아래 표와 예시는 짧은 쪽(`effort …`)으로 적는다. 실제로 칠 때는 위 표의 앞자리를 붙인다.

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

실행 파일이 없으면 EffortTool 폴더 안에서 만든다 : `go build -o bin/effort.exe ./cmd/effort` (`CGO_ENABLED=0`).

`estimate` 가 내는 표는 그대로 보고의 공수 표에 붙는다. 옵션은 명령 바로 뒤,
작업 id 는 맨 끝에 둔다. **어기면 바른 차례를 알려 주며 오류로 막는다.**

## 숫자의 뜻 (2026-09-01)

- **벽시계** = 본줄 구간 ∪ 서브에이전트 구간의 **합집합**. 큐·파일이력 같은 곁줄은 시간을 못 늘린다.
- **순수시간** = 턴 시간의 합. 벽시계를 넘으면 벽시계로 자르고 `순수시간잘림` 을 남긴다.
- **묶음 시간**에는 작업과 작업 **사이의 사람 대기가 안 든다**. 지금은 묶음 안 작업의 본줄 구간만 더한다.

## rules.txt 는 언제 손대나

- 코드가 새 규칙 종류를 더한 경우는 `scan` 이 빠진 종류의 기본 줄을 알아서 덧붙인다. 할 일 없다.
- **낱말을 손으로 고쳤으면 `effort scan --rebuild`** 로 캐시를 다시 매긴다 — 규칙만 바뀐 것은
  아직 scan 이 못 알아챈다 (todo `20260901-0178158e`).
- `groups.txt`(소단계 정본)는 캐시가 아니라 **`--rebuild` 해도 안 날아간다.**
- 크기 배율(**S 0.16 · M 1.0 · L 4.46 · XL 19.41**)은 2026-09-01 **묶음 단위** 실측(일 칸 묶음 205건)으로
  잡은 값이다. 묶음 벽시계에 서브에이전트 구간까지 넣어 다시 잰 값이다. 옮기려면 먼저 잰다.

## 알아 둘 한계 둘

1. **소단계는 `effort group --add` 로 사람이 표시**한다. 표시가 없는 자리는 세션 안 30분 간격으로
   자동으로 묶는다 (컷은 `rules.txt` 의 `gap max`).
2. **벽시계에서 사람이 기다린 시간은 이제 안 센다**(2026-09-01). 눈금 규칙을 쓸 자리는
   **처음 해 보는 실측**뿐이다 — 「값을 재서 정한다」는 ×2, 시그니처까지 정한 설계가 있으면 ÷4.

> 묶음 벽시계에 서브에이전트 구간이 들어가 **자릿수는 맞아졌다** (구현 L = 4분 → 17분, 2026-09-01).
> 다만 `list --group` 은 일 칸 작업만 모아 묶고 `estimate` 는 모든 작업을 묶은 뒤 고르므로
> **표본이 서로 다르다.** 배율은 `list` 쪽 값이라 `estimate` 결과가 조금 작게 나온다.
> 자릿수는 믿어도 되지만 **분 단위까지 믿지는 말고 순서와 크기 차이를 참고**한다.

## 끝나고 남기는 것

숫자는 툴이 갖고 있으니 파일로 안 남긴다. **예상과 실제가 왜 어긋났나 · 다음에 쓸 눈금**만
`mem` 에 남긴다 (`mem-usage` 스킬 — `--type history`, 눈금이면 `howto`).
지난 눈금은 `mem search 눈금 공수` 로 먼저 본다 (배율 `20260831-d485b6c9`).
