---
name: effort-usage
description: Use when estimating effort before starting work, filling the 실제 column of a 공수 표, or writing a completion report with time numbers — 공수를 잡을 때 · 완료 보고에 실제 시간을 적을 때 · 소단계 표를 만들 때 EffortTool 로 재는 법.
---

# 공수는 EffortTool 로 잰다

**시간 숫자의 정본은 실측이다.** 어림하지 않는다. 옵션의 자세한 꼴은 `effort help <명령>`,
분류·셈법은 `EffortTool/README.md`.

## 언제 무엇을

| 이럴 때 | 이렇게 |
| --- | --- |
| 작업 시작 전, 소단계 표의 **예상** 칸 | `effort estimate 조사:M 설계:M 구현:L` — 분류별 p50 × 크기 배율 |
| 소단계가 여럿이라 표로 주고 싶을 때 | 파이프 표를 `--from -` 으로 먹인다 (열 : 소단계·분류·크기) |
| 소단계가 끝나 **실제** 칸을 채울 때 | `effort scan` 으로 캐시를 갱신하고 `effort list --since <날짜> --sort wall` 에서 내 작업 줄의 벽시계를 본다 |
| 작업 하나를 자세히 | `effort show <작업id앞자리>` — 모델별 토큰·서브에이전트·검산까지 |
| 세션 통째 걸린 시간 | `effort stats --by session --since <날짜>` |
| 무슨 일에 시간이 쏠렸나 | `effort stats --by class` |

```powershell
.\EffortTool\bin\effort.exe scan --all     # 없으면 : go build -o bin/effort.exe ./cmd/effort (CGO_ENABLED=0)
.\EffortTool\bin\effort.exe estimate 조사:M 설계:M 구현:L
```

`estimate` 가 내는 표는 그대로 보고의 공수 표에 붙는다. 옵션은 명령 바로 뒤,
작업 id 는 맨 끝에 둔다.

## 알아 둘 한계 둘

1. **작업 한 건 = 사용자 프롬프트 한 개**라 우리가 말하는 소단계보다 잘다. `estimate` 값이
   작게 나오면 그 탓이다. 소단계 하나가 프롬프트 여럿이면 `list --session <id>` 로 묶어 더한다.
2. **EffortTool 이 못 재는 것**(사람이 기다린 시간, 갈래 합치기, 처음 해 보는 실측)에만
   눈금 규칙을 쓴다 — 「값을 재서 정한다」는 ×2, 시그니처까지 정한 설계가 있으면 ÷4.

## 끝나고 남기는 것

숫자는 툴이 갖고 있으니 파일로 안 남긴다. **예상과 실제가 왜 어긋났나 · 다음에 쓸 눈금**만
`mem` 에 남긴다 (`mem-usage` 스킬 — `--type history`, 눈금이면 `howto`).
지난 눈금은 `mem search 눈금 공수` 로 먼저 본다 (배율 `20260831-d485b6c9`).
