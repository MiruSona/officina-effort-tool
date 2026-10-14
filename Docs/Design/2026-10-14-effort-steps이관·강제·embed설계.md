# effort-steps 이관 · 강제 · embed 설계 (2026-10-14)

> 대상 : 사용자 확정 2026-10-14 세 결정을 구체화한다. 결정 자체는 다시 묻지 않는다.
> ① effort-steps mod 를 ModTool 에서 **EffortTool 안으로 옮긴다** (mem `20261014-f39633b6`)
> ② **plan 없는 작업은 deny 로 막는다** (mem `20261014-e0e0d244`)
> ③ mod 소스를 **`go:embed` 로 exe 에 넣고 `effort mod install` 로 푼다** (mem `20261014-d6cd6d6c`)
> mod 가 `mark start`·`stop`·`scan` 을 부르지 않는 결정(mem `20261007-c6a0521a`)은 그대로다.
> 옛 설계는 ModTool 의 「2026-10-07 공수표 mod 설계」 문서다 (그 저장소 자리에 그대로 둔다).

## 결론

1. **소스 자리는 `EffortTool/mod/effort-steps/`**, embed 하는 Go 파일은 **`EffortTool/mod/embed.go`(package `mod`)** 다.
   `go:embed` 는 Go 파일이 있는 폴더 **아래만** 담을 수 있어서 그 짝으로 둔다.
2. **exe 에는 돌 때 쓰는 파일만 넣는다** — `.claude-plugin/plugin.json` · `hooks/` · `types/`. `tests/`·`tsconfig.json` 은 저장소에만 둔다.
3. **`effort mod install | uninstall | status`** 셋. 덮어쓰기·지우기는 늘 **`~/.claude/mod-backups/effort-steps.bak-<yyyyMMdd-HHmmss>` 로 옮기기**다 (ModTool `install.ps1` 과 같은 꼴). 지우는 코드는 없다.
4. **`effortPath` 는 userConfig 칸이다** (코드 확인 : `plugin.json` userConfig → `state.ts` `readSettings(options)`).
   exe 는 Claude Code 설정(`pluginConfigs`)을 건드리지 않고, **풀어 쓴 설치본 `plugin.json` 의 `effortPath.default` 를 자기 exe 경로로 채운다.**
   사람이 `/config` 에서 값을 적었으면 그 값이 이긴다.
5. **강제는 메인 루프 · 대화형 세션에서만** 한다. 서브에이전트(`agentId` 있음)의 편집은 막지 않는다 — 문은 `Agent` 띄우기에서 이미 지났다.
   `claude -p`(로컬 하네스 · 머리 없는 판)는 `isInteractive=false` 라 아예 안 본다.
6. **「plan 있음」 = `planAtom` 이 있고 그 `sessionId` 가 지금 `$.session.id()` 와 같다.** `/clear` 뒤에는 세션 id 가 바뀌므로 저절로 「plan 없음」이 된다.
7. 강제가 터지면 **막지 않고 지나간다(fail-open)**. 공수 표 mod 가 고장 나 일이 멈추는 것이 더 나쁘다.

## 이번에 정한 것

| 무엇 | 정한 것 | 까닭 |
| --- | --- | --- |
| 소스 폴더 | `EffortTool/mod/effort-steps/` | `go:embed` 는 `..` 를 못 쓴다. `mod/embed.go` 아래에 있어야 담긴다 |
| embed 꼴 | `//go:embed effort-steps/.claude-plugin/plugin.json effort-steps/hooks effort-steps/types` | 점으로 시작하는 폴더는 폴더째 담으면 빠진다. 파일을 이름으로 적는다. 엔진이 까는 `.claude-plugin/types/` 도 이걸로 안 담긴다 |
| 담는 파일 | plugin.json · hooks · types 만 | 시험·tsconfig 는 돌 때 안 쓴다. exe 가 작아지고 설치본이 깔끔하다 |
| effortPath 넣는 법 | 설치본 plugin.json 의 `userConfig.effortPath.default` 를 고쳐 쓴다 | 설정 파일은 exe 몫이 아니다(분류기). 설치본은 exe 가 푼 파일이라 고쳐도 된다. 엔진은 안 적힌 칸을 default 로 채운다 |
| plugin.json 다시 쓰기 | `map` 으로 읽고 칸 하나만 바꿔 들여쓰기 2칸으로 쓴다. 키 차례가 바뀌는 것은 받아들인다 | 글 바꾸기(placeholder)는 칸이 늘면 깨진다. status 는 뜻으로 견준다 |
| effortPath 가 틀렸을 때 | 적힌 경로에 파일이 없으면 **그다음 후보(작업폴더 EffortTool/bin · PATH)로 계속 찾는다** | 지금은 적힌 값이 있으면 그것만 본다. 저장소를 옮기면 설치본 default 가 낡는다 |
| 임시 exe 막기 | 자기 경로가 TEMP 아래(`go run`)면 install 을 멈춘다. `--effort-path <경로>` 로 덮을 수 있다 | 빌드 안 한 임시 exe 경로가 default 로 박히면 다음 판에 사라진다 |
| 풀어 쓰는 차례 | `mod-backups/effort-steps.new-<시각>` 에 먼저 다 쓰고 → 옛 설치본을 `.bak-<시각>` 으로 옮기고 → 새 폴더를 `skills/effort-steps` 로 옮긴다 | 중간에 죽어도 skills 아래에 반쪽 설치본이 안 남는다. skills 아래 임시 폴더는 엔진이 올려 버린다 |
| uninstall | 지우지 않고 `.bak-<시각>` 으로 옮긴다 | 지우기는 승인 대상이다. 되돌리기도 쉽다 |
| dev-mods 겹침 | `~/.claude/dev-mods/*/effort-steps/` 가 있으면 경로를 찍고 **종료 1로 멈춘다**. `--force` 면 경고만 하고 깐다 | install.ps1 과 같은 꼴. 겹치면 다음 시작 때 설치본이 거절된다 |
| status 판정 | 담긴 파일과 설치본을 하나씩 견줘 `같음 / 다름 / 없음` 을 찍는다. 다르면 종료 2 | 「exe 를 다시 구웠는데 install 을 잊음」을 바로 잡는다 |
| 강제 범위 | 메인 루프(`agentId` 없음) · 대화형만 | 서브 판은 백그라운드로 턴을 넘겨 돈다. `/steps allow` 가 사람 말에 지워진 뒤 서브 편집이 막히면 아무도 못 푼다 |
| 막는 도구 | `tool.call` Edit · Write · NotebookEdit · Agent, 그리고 `agent.spawn`(메인에서 난 것) | 사용자 결정 그대로. `agent.spawn` 은 workflow `agent()`·`$.agent.spawn` 처럼 Agent 도구를 안 지나는 길을 잡는다 |
| 빼는 자리 | Edit·Write 대상이 `~/.claude/` 아래(내장 기억 · 계획 파일)나 TEMP(스크래치패드) 아래면 안 본다 | 「기억해 둬」 같은 질문 턴의 내장 기억 쓰기가 막히면 안 된다 |
| 셸 쓰기 | 안 본다 | 결정이 네 도구만 적었다. 셸 파싱은 delegate-check 몫이다 |
| userConfig `mode` | `deny`(기본) · `warn` · `off` | `off` 는 계획 규칙이 없는 저장소에서 mod 를 지우지 않고 강제만 끄는 길 |
| warn 꼴 | 지나가게 두고 그 턴 첫 번에만 도구 결과에 경고 한 줄을 붙인다 | delegate-check `warnAfter` 와 같은 꼴 |
| `/steps allow` | 사람이 터미널에서 친 것만(`origin.kind === 'composer'`). 다음 사람 말(슬래시 명령 아닌 것)에 지운다 | delegate-check `/delegate allow` 와 같은 꼴. 모델이 스스로 풀 수 없게 |
| 열린 mark 없음 | 메인 Edit·Write·NotebookEdit 가 plan 은 있는데 세션 안에 열린 mark 가 하나도 없으면 그 턴 첫 번에 경고 한 줄. Agent 띄우기에는 안 붙인다 | mark 는 서브가 자기 판에서 연다. 띄우는 순간엔 없는 게 맞다 |
| 안 닫힌 mark toast | 메인 턴 끝(`turn.complete`, `agentId` 없음) : 메인이 연 mark 중 열린 것. 서브 끝(`agentId` 있음) : 그 서브가 연 mark 중 열린 것. mark 하나에 한 번만 | 메인 턴 끝에 서브 mark 가 열려 있는 것은 정상이다 |
| 실패 처리 | 모든 강제 훅의 `.catch` 는 `next(e)` | 위 결론 7 |
| 판 번호 | plugin.json `version` 0.1.0 → 0.2.0 | 강제가 더해진 판임을 status 로 보이게 |

## 1. 폴더 구조

```text
EffortTool/
  mod/
    embed.go                 ← package mod. var Files embed.FS (아래 셋만)
    embed_test.go            ← 담긴 파일 목록 · plugin.json 이 JSON 으로 읽히는지
    effort-steps/            ← mod 정본 (ModTool/mods/effort-steps 를 옮긴 것)
      .claude-plugin/plugin.json
      hooks/  (main.tsx · effort.ts · pair.ts · parse.ts · state.ts · table.ts · view.tsx · gate.ts(새) · hooks.json)
      types/index.d.ts
      tests/  (exe 에 안 담김)
      tsconfig.json          (exe 에 안 담김)
  internal/modinstall/       ← 풀기 · 견주기 · 백업 옮기기 (순수에 가깝게, 뿌리를 인자로 받는다)
  cmd/effort/cmd_mod.go      ← `effort mod …` 인자 풀기 · 출력 · 종료 코드
```

- `mod/` 은 Go 패키지이면서 TS 소스 폴더다. Go 도구는 `.ts`·`.tsx` 를 무시하므로 `go vet ./...` 에 안 걸린다.
- `.gitignore` 에 `mod/*/.claude-plugin/types/` 를 더한다 (엔진이 깔면 저장소에 딸려 오지 않게).

### 빌드에 얹기 (`build.ps1`)

- 커밋 도장 · `-dirty` 판정 경로에 **`mod` 를 더한다** (`-- cmd internal mod go.mod`). mod 만 고쳐도 exe 가 새 판으로 찍힌다.
- embed 는 `go build` 가 알아서 담는다. 다른 단계는 없다.
- `-Test` 는 Go 시험만 돈다. mod 시험(`claude plugin validate`·`test`)은 개발 흐름에서 따로 친다 — `claude` 가 없는 자리에서도 빌드가 되게.

### 두 흐름

| 흐름 | 순서 |
| --- | --- |
| **개발 (핫리로드)** | ① `mod/effort-steps` 를 그 세션의 `~/.claude/dev-mods/<세션id>/effort-steps/` 로 복사 ② 거기서 고치며 핫리로드로 본다 ③ `claude plugin validate` · `tsc -p` · `claude plugin test` ④ 끝나면 `mod/effort-steps` 로 되돌려 복사 (정본은 저장소) ⑤ **그 dev-mods 폴더를 지운다** |
| **빌드 뒤 설치** | ① `.\build.ps1 -Test` ② **개발 세션이 끝난 뒤** `.\bin\effort.exe mod install` ③ `mod status` 로 `같음` 확인 ④ 새 세션을 연다 |

## 2. `effort mod` 명령

공통 : 대상 이름은 `effort-steps` 하나뿐이라 이름 인자를 받지 않는다. `--claude-dir <폴더>` (기본 `%USERPROFILE%\.claude`, 시험용)만 셋이 같이 받는다.
쓰는 자리는 **`<claude-dir>/skills/effort-steps/` 와 `<claude-dir>/mod-backups/` 둘뿐**이다 — 각각 `paths.Jail` 로 묶는다.

### `effort mod install [--force] [--effort-path <경로>]`

1. dev-mods 겹침을 본다. 있으면 경로마다 한 줄 찍고 `--force` 가 없으면 **종료 1**.
2. exe 경로를 정한다 : `--effort-path` → 아니면 `os.Executable()` + `EvalSymlinks`. TEMP 아래면 **종료 1** (안내 : `build.ps1` 로 만든 exe 로 다시 치거나 `--effort-path`).
3. `mod-backups/effort-steps.new-<시각>/` 에 담긴 파일을 쓴다. plugin.json 은 `effortPath.default` 만 바꿔 쓴다.
4. 옛 `skills/effort-steps` 가 있으면 `mod-backups/effort-steps.bak-<시각>` 으로 옮긴다.
5. 새 폴더를 `skills/effort-steps` 로 옮긴다. 5 가 실패하면 4 를 되돌린다.

```text
backup : C:\Users\…\.claude\mod-backups\effort-steps.bak-20261014-213000
installed : C:\Users\…\.claude\skills\effort-steps  (effort-steps 0.2.0 · exe 35864f5)
effortPath 기본값 : C:\…\EffortTool\bin\effort.exe
설정(/config)에 effortPath 를 따로 적었다면 그 값이 먼저다.
새 세션부터 돈다.
```

### `effort mod uninstall`

`skills/effort-steps` 를 `mod-backups/effort-steps.bak-<시각>` 으로 옮긴다. 없으면 「깔려 있지 않다」 찍고 종료 0.
dev-mods 겹침은 알리기만 한다.

### `effort mod status`

```text
설치본 : C:\Users\…\.claude\skills\effort-steps  (0.2.0)
이 exe 에 담긴 판 : 0.2.0 · exe 35864f5
견줌 : 같음 (파일 9개)
effortPath 기본값 : C:\…\EffortTool\bin\effort.exe  (있음)
dev-mods 겹침 : 없음
설정 값 보기·바꾸기 : Claude Code 에서 /config → effort-steps (mode · effortPath · refreshSec · redRatio · autoPane)
```

- 견줌은 파일마다. plugin.json 은 **뜻(JSON 값)** 으로 견주되 `effortPath.default` 칸은 뺀다. 나머지는 바이트로.
- exe 는 `pluginConfigs` 를 읽지도 않는다 — 「지금 설정 값」은 `/config` 로 보라고만 쓴다.

### 종료 코드 (기존 표를 그대로 쓴다)

| 코드 | 뜻 |
| --- | --- |
| 0 | 됐다 · status 같음 · uninstall 할 것 없음 |
| 1 | 사용법 오류 · dev-mods 겹침(`--force` 없음) · TEMP 아래 exe |
| 2 | status : 설치본 없음 또는 다름 (`exitNoData`) |
| 3 | 설치본을 못 읽음 |
| 4 | 쓰기·옮기기 실패 (`exitWrite`) |

## 3. 훅 강제

### 판정 (순수 함수 `gate.ts judgeGate`)

```text
대화형 아님 · mode off · agentId 있음 · 빼는 자리     → 지나감
plan 있음(같은 세션 id)                              → 지나감 (+ 메인 편집이면 열린 mark 경고 검사)
/steps allow 이번 턴                                 → 지나감
mode warn                                            → 지나감 + 그 턴 첫 번에 경고 한 줄
mode deny                                            → { deny: 글 }
```

| 이벤트 | 볼 때 | 메인 판정 근거 |
| --- | --- | --- |
| `tool.call` Edit · Write · NotebookEdit | 돌기 전 | `e.agentId === undefined` |
| `tool.call` Agent | 돌기 전 | `e.agentId === undefined` |
| `agent.spawn` | 띄우기 전 | `e.parentAgentId === undefined` (타입 문서 : main 에서는 없다) |
| `prompt.submit` | 사람 말 | 슬래시 아닌 composer 말이면 allow · 턴 경고 표시를 지운다 |
| `session.start` | 세션 시작 · 리로드 | `isInteractive` 를 atom 에 둔다 |
| `turn.complete` | 턴 끝 | 안 닫힌 mark toast |

Agent 도구 호출은 `tool.call` 과 `agent.spawn` 을 둘 다 지난다. 앞에서 막히면 뒤는 안 온다. 둘 다 보는 것은 Agent 도구를 안 지나는 띄우기 때문이다.

### 막을 때 글

```text
effort-steps : 이 세션에 공수 계획이 없어 막았다 (Edit).
먼저 plan 도구로 소단계를 넣는다 — 도구 이름 mcp__effort-steps__plan,
목록에 이름만 있으면 ToolSearch 로 "select:mcp__effort-steps__plan" 을 먼저 불러온다.
질문에 답만 하는 턴이면 편집·Agent 를 쓰지 않는다.
사람이 이번 턴만 풀려면 /steps allow, 늘 경고로만 두려면 /config 의 mode.
```

plan 도구가 지연 도구(deferred)로 뜨는 것을 이 세션에서 봤다. 그래서 불러오는 법까지 적는다.

### 뒷문

- `/steps allow` — composer 에서 친 것만. 다음 사람 말 전까지. `/steps status` 에 「허용 중」으로 보인다.
- userConfig `mode` — `deny`·`warn`·`off`. 고치면 엔진이 모듈을 다시 올린다(`options` 새 값).
- `/steps status`(새) — mode · 대화형 · plan 있음(세션 id 같음) · 허용 중 · 이번 턴 경고 수.

### 서브에이전트 안에서

- **같은 훅이 본다** : 서브에이전트의 `tool.call` 도 이 mod 의 훅을 지나고 `e.agentId` 가 붙는다
  (확인 : delegate-check 는 `agentId !== undefined` 로 서브를 빼고, 지금 effort-steps `observe` 는 서브 mark 를 `e.agentId` 로 가른다. 엔진 타입 문서도 `tool.call`·`turn.start`·`turn.complete` 가 `agentId` 를 단다고 적는다).
- 그래도 **서브는 막지 않는다** (위 표 「강제 범위」). model-guard·commit-prep 은 서브까지 잡지만, 그 둘은 「그 명령 한 번」이 틀린 경우라 서브가 스스로 고칠 수 있다. plan 은 메인 몫이라 서브는 못 고친다.
- 서브가 다시 서브를 띄우는 것(`agent.spawn` 에 `parentAgentId` 있음)도 안 막는다.

### 세션 재개 · `/clear`

- `--resume` : `session.start` 가 store `plan:<세션id>` 사본을 되살린다(지금 코드). 같은 세션 id 면 plan 있음 → 안 막힌다.
  재개가 새 id 로 이어지면 사본을 못 찾아 「plan 없음」 → 다시 plan 을 넣으라고 막는다. 다시 넣는 값이 싸므로 받아들인다.
- `/clear` : 엔진 문서상 **`session.start` 가 안 돌고 세션 id 만 바뀐다**. `$.state` 가 비는지는 문서로 못 확인했다.
  그래서 판정을 `plan.sessionId === $.session.id()` 로 둔다 — 비든 안 비든 「plan 없음」이 된다. `session.end`(reason `clear`)에서 allow 도 지운다.
- 리로드(파일 고침·mode 바꿈) : `$.state` 는 남는다. 모듈 변수만 비므로 판정은 atom 만 읽는다.

### 「질문만 받은 턴」이 안 걸리는 근거

막는 길목은 편집 셋과 Agent 띄우기뿐이다. Read · Grep · Glob · Bash · PowerShell · WebFetch · MCP 도구는 안 본다.
답만 하는 턴은 이 넷을 안 부르므로 걸리지 않는다. 내장 기억(`~/.claude/…/memory`) 쓰기는 「빼는 자리」로 지나간다.
`claude -p` 판은 대화형이 아니라 아예 안 본다.

## 4. 지금 mod 코드에서 바뀌는 곳

| 파일 | 함수·자리 | 바뀜 |
| --- | --- | --- |
| `hooks/gate.ts` (새) | `judgeGate` · `isSkipPath` · `denyText` · `warnLine` · `noMarkLine` · `unclosedToast` | 순수 판정과 글. `$` 안 씀 |
| `hooks/main.tsx` | `register` | 훅 더하기 : `prompt.submit` · `tool.call` Edit/Write/NotebookEdit/Agent · `agent.spawn` · `turn.complete` · `session.end`. 각 `.catch(($,e,next)=>next(e))` |
| `hooks/main.tsx` | `session.start` 훅 | `isInteractive` 를 `interactiveAtom` 에. `/steps` argumentHint 에 `allow`·`status` |
| `hooks/main.tsx` | `command.run steps` | `allow`(composer 만) · `status` 갈래 더하기. `default` 쓰는 법 글 고침 |
| `hooks/main.tsx` | 새 맨 위 함수 `hasPlan($)` · `gateCall($, tool, agentId, path)` · `warnOnce($, ran, line)` · `toastUnclosed($, agentId)` | `$` 쓰는 일만 |
| `hooks/main.tsx` | `planTool` 설명(`session.start` 의 `tool.register`) | 「편집·Agent 를 쓰기 전에 먼저 부른다」 한 줄 |
| `hooks/state.ts` | `Settings` · `readSettings` | `mode: 'deny'\|'warn'\|'off'` 더하기 (모르는 값은 `deny`) |
| `hooks/effort.ts` | `exeCandidates` | 적힌 경로 뒤에도 나머지 후보를 잇는다 |
| `hooks/effort.ts` | `exeMissing` | 안내에 `effort mod install` 을 더한다 |
| `types/index.d.ts` | `PluginState['effort-steps']` | `interactive: boolean` · `allow: {at:number}\|null` · `turn: {warned:boolean; noMarkWarned:boolean}` · `toasted: string[]` |
| `.claude-plugin/plugin.json` | `userConfig` · `version` · `description` | `mode` 칸(options deny·warn·off, 기본 deny) · 0.2.0 · 「plan 없는 편집을 막는다」 |
| `tests/` | `gate.test.ts`(새) · `flow.test.ts` | 판정 표 전부 · deny/allow/warn/off · 서브 통과 · 비대화형 통과 · `/clear` 뒤 id 바뀜 · 빼는 자리 · toast 한 번 |

## 5. 옮긴 뒤 ModTool 쪽 뒷정리

| 할 일 | 비고 |
| --- | --- |
| `ModTool/mods/effort-steps/` 폴더 지우기 | **승인 대상.** EffortTool 쪽 판이 깔린 뒤, 같은 커밋에서 |
| ModTool README mod 표 · 「effort-steps.」 절 · 띠 보기 절 · 설치 절의 effortPath 줄 | 표 줄을 「EffortTool 로 옮겼다 — `effort mod install`」 한 줄로. 띠 포개기 설명의 effort-steps 이름은 남긴다(같은 자리에 포개진다) |
| `install.ps1` | 코드 고칠 것 없음 (mods 아래를 돌기만 한다). 단 **폴더를 지우기 전에 `install.ps1` 을 돌리면 옛 판이 새 설치본을 덮는다** — 지우기와 설치를 한 흐름에서 한다 |
| 옛 설계 문서 | ModTool 에 기록으로 그대로 둔다 |
| agent-board 의 「effort-steps 에서 복사」 주석 | 그대로 둔다 (복사본이라 경로 의존 없음) |

## 6. 함정

- **dev-mods 찌꺼기** : 지금 `~/.claude/dev-mods/e8d712db-c07d-4999-bd50-593d8a9f69a5/effort-steps` 가 남아 있다(같은 자리에 agent-board · prompt-gauge 도).
  있으면 설치본이 「name already taken by a session-only plugin」 으로 거절된다. **먼저 지운다 (승인 대상).** `mod install` 도 이걸 보고 멈춘다.
- **서브모듈 두 번 커밋** : EffortTool 은 서브모듈이다. ① EffortTool 안에서 커밋·푸시 ② 스튜디오에서 `git add EffortTool` + ModTool 폴더 삭제 + README. ModTool 은 서브모듈이 아니라 ② 한 번에 든다.
- **교체 차례** : ① 찌꺼기 지우기 ② `build.ps1 -Test` ③ **개발 세션을 닫은 뒤** `effort mod install` (옛 ModTool 설치본은 `.bak` 으로 옮겨진다) ④ `mod status` 같음 ⑤ 새 세션에서 plan 없이 Edit 를 쳐 막히는지 본다 ⑥ ModTool 폴더 지우기. plugin 이름이 같아 `/config` 값(`pluginConfigs`)은 그대로 이어진다.
- **mcp 도구 이름** : `mcp__effort-steps__plan` 은 plugin 이름에서 나온다. 이름을 바꾸면 CLAUDE.md·스킬의 도구 이름이 다 깨진다 — 안 바꾼다.
- **exe 하나가 계정 전체를 맡는다** : 게임 저장소의 `Tools/EffortTool` exe 와 스튜디오 exe 판이 다르면, **마지막에 install 한 exe** 가 default 로 박힌다. status 로 본다.
- **서브의 plan 호출** : 지금 `planTool` 은 서브가 불러도 세션 계획을 갈아 끼운다. 강제가 생기면 작업서에 「plan 넣어라」를 적은 서브가 메인 계획을 덮을 수 있다 (아래 물을 것 ①).

## 7. 소단계 (구현 판용)

| 소단계 | 종류 | 크기 |
| --- | --- | --- |
| 1. 찌꺼기 지우기 승인 · 소스를 `EffortTool/mod/effort-steps` 로 옮기기 · `.gitignore` | 구현 | S |
| 2. `mod/embed.go` · `embed_test.go` · `build.ps1` 도장 경로 | 구현 | S |
| 3. `internal/modinstall` (풀기 · plugin.json default · 백업 옮기기 · 견주기) + 시험 | 구현 | L |
| 4. `cmd_mod.go` · `main.go` · `help.go` (install/uninstall/status · 종료 코드) + 시험 | 구현 | M |
| 5. `gate.ts` 판정 · 글 + `gate.test.ts` | 구현 | M |
| 6. `main.tsx` 훅 잇기 · `state.ts` · `effort.ts` · `types` · `plugin.json` + `flow.test.ts` | 구현 | L |
| 7. 핫리로드 손 시험 (deny · allow · warn · 서브 통과 · `/clear`) | 실측 | M |
| 8. 설치 교체 (install · status · 새 세션 확인) | 실측 | S |
| 9. README · effort-usage 스킬 · ModTool README · ModTool 폴더 지우기(승인) | 문서 | M |
| 10. 코드리뷰 한 판 | 검토 | M |

## 못 정한 것 · 물을 것

1. **서브에이전트의 plan 호출을 막을까?** 메인 계획을 덮는 문제 (함정 마지막). 안 : 서브(`agentId` 있음)의 plan 은 「plan 은 메인 몫」 글로 거절. 이 설계 범위 밖이라 묻는다.
2. **`mode` 에 `off` 를 둘까?** 이 설계는 둔다. 「끄려면 uninstall」 로 충분하다면 뺀다.
3. **`/steps mode warn` 처럼 세션만 바꾸는 명령도 둘까?** delegate-check 에는 있다. 이 설계는 `/config` 만 둔다.
4. 엔진이 `~/.claude/skills` 아래 플러그인에도 **userConfig default 를 options 에 채우는지**는 시험 키트 문서(「defaults filled in」)로만 봤다. 소단계 8 에서 `/steps status` 에 잡힌 exe 경로로 확인한다. 안 채우면 설치본 옆 파일을 `$.fs` 로 읽는 길로 바꾼다.
5. `/clear` 뒤 `$.state` 가 비는지 — 판정은 세션 id 로 하므로 결과는 같지만, 소단계 7 에서 실물로 본다.
