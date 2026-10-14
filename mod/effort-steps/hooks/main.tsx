// effort-steps : 소단계 진행 표를 띠와 /steps 패널로 보여 준다.
// 여기서는 이벤트를 잇기만 한다. 파싱·짝짓기·표·그림은 다른 파일에 있다.
//
// 절대 규칙 : 이 mod 는 effort mark start · stop · scan 을 부르지 않는다.
// Claude(메인·서브)가 친 명령을 tool.call 에서 「보기만」 하고, 값은 mark show 로 읽는다.
//
// 강제 (설계 2026-10-14 3절) : 메인 대화형 세션에서 plan 없이 Edit·Write·NotebookEdit·Agent 를 부르면 막는다.
// 판정·글은 gate.ts(순수)에 있다. 강제 훅은 이 mod 몫(판정·기록)만 failOpen 으로 감싼다 — 이 mod 가 터져 일이 멈추면 안 된다.
// next(e) 는 감싸지 않는다 : .catch(($,e,next)=>next(e)) 꼴은 next 가 던질 때 도구·띄우기를 한 번 더 돌린다.
import { atom, read, update } from 'claude-code'
import type { EngineInterface, ProcessRunResult, Register, Timer, ToolCallResult } from 'claude-code'

import type { ExeInfo, MarkInfo, Plan, Step, StepStatus } from '../types'
import {
  ESTIMATE_TIMEOUT_MS, MARK_ID_RE, SHOW_TIMEOUT_MS, estimateArgs, exeCandidates, exeMissing, isAllowedArgs,
  marksPathFrom, readEstimate, readShow,
} from './effort'
import type { EstimateOut, ShowOut } from './effort'
import {
  allDone, denyText, hasOpenMark, isSkipPath, judgeGate, noMarkLine, statusText, subPlanText, unclosedMarks,
  unclosedToast, warnLine,
} from './gate'
import { attach, newMark, pairOf, pairOnReplan, pickFromMarksFile, settle } from './pair'
import { clean, parseBlocks, parseCommand, parseMarksFile } from './parse'
import type { ShowBlock } from './parse'
import { PANE, readSettings } from './state'
import type { Settings } from './state'
import { markdownTable, totals } from './table'
import { bandTree, countRows, paneTree } from './view'
import type { Els, ViewData } from './view'

type $ = EngineInterface

// $.state 참조. 모양은 types/index.d.ts 계약에 있다.
const planAtom = atom({ plugin: 'effort-steps', key: 'plan' } as const, null)
const marksAtom = atom({ plugin: 'effort-steps', key: 'marks' } as const, {})
const exeAtom = atom({ plugin: 'effort-steps', key: 'exe' } as const, { path: null, why: null })
// 띠는 기본으로 숨긴다 (공수 표는 /steps 패널로 본다, 2026-10-07). /steps show 로 켜면 prefs 에 남아 다음 세션도 켜진다.
const uiAtom = atom({ plugin: 'effort-steps', key: 'ui' } as const, { bandHidden: true, expanded: false })
const pausedAtom = atom({ plugin: 'effort-steps', key: 'paused' } as const, false)
const interactiveAtom = atom({ plugin: 'effort-steps', key: 'interactive' } as const, false)
const allowAtom = atom({ plugin: 'effort-steps', key: 'allow' } as const, null)
const turnAtom = atom({ plugin: 'effort-steps', key: 'turn' } as const, { warned: false, noMarkWarned: false })
const toastedAtom = atom({ plugin: 'effort-steps', key: 'toasted' } as const, [])


const CLASSES = ['조사', '설계', '구현', '실측', '문서', '검토'] as const
const SIZES = ['S', 'M', 'L', 'XL'] as const
const STATUSES: readonly StepStatus[] = ['할 일', '하는 중', '끝', '건너뜀']
const MAX_STEPS = 30
const MAX_OPEN_SHOWS = 8
const MAX_FAILS = 3
const HISTORY_KEEP = 30
const PLAN_KEYS_KEEP = 20
/** 계획 사본에 같이 남길 짝 없는 mark 수 (최근 것부터) */
const ORPHANS_KEEP = 10
const PANE_TITLE = '공수'

const PLAN_TOOL = 'mcp__effort-steps__plan'
const STEP_TOOL = 'mcp__effort-steps__step'
const TABLE_TOOL = 'mcp__effort-steps__table'

const PLAN_SCHEMA = {
  type: 'object',
  properties: {
    title: { type: 'string', description: '작업 이름 (띠 첫 줄에 나온다)', maxLength: 80 },
    steps: {
      type: 'array',
      minItems: 1,
      maxItems: MAX_STEPS,
      items: {
        type: 'object',
        properties: {
          n: { type: 'integer', minimum: 1, maximum: 999, description: '소단계 번호. mark 이름 앞 번호와 같다 (예 3 → mark start "3-검사기")' },
          name: { type: 'string', maxLength: 40, description: '소단계 이름 (짧게)' },
          class: { type: 'string', enum: [...CLASSES] },
          size: { type: 'string', enum: [...SIZES] },
        },
        required: ['n', 'name', 'class'],
      },
    },
  },
  required: ['title', 'steps'],
}

const STEP_SCHEMA = {
  type: 'object',
  properties: {
    n: { type: 'integer', minimum: 1, maximum: 999 },
    status: { type: 'string', enum: [...STATUSES] },
    note: { type: 'string', maxLength: 120, description: '까닭 한 줄 (예 어긋난 까닭, 건너뛴 까닭)' },
  },
  required: ['n'],
}

// 모듈 변수 : 리로드하면 처음으로 돌아간다. 그래서 여러 이벤트에서 armTimer 를 부른다.
// ($ 를 받는 함수는 엔진 검사 때문에 이 파일 맨 위 함수로만 둔다.)
let cfg: Settings = readSettings({})
let timer: Timer | null = null
let busy = false
/** exe 실행 자체가 잇달아 실패한 횟수. 이것만 전체 갱신을 멈춘다 */
let exeFails = 0

// ── 상태 쓰기 도우미 ─────────────────────────────────────────

/**
 * effort 를 부르는 유일한 길. estimate 와 mark show 밖의 인자는 여기서 막는다
 * (mark start · stop · scan 은 절대 부르지 않는다).
 */
async function runEffort($: $, exe: string, args: readonly string[], timeoutMs: number): Promise<ProcessRunResult | string> {
  if (!isAllowedArgs(args)) return `부르지 않는 effort 인자 : ${args.slice(0, 2).join(' ')}`
  try {
    return await $.process.run([exe, ...args], { timeoutMs })
  } catch (err) {
    return `실행 실패 : ${String(err)}`
  }
}

/** exe 찾기 : 후보를 차례로 fs.exists 로 본다. */
async function findExe($: $, configured: string): Promise<ExeInfo> {
  const bases: string[] = []
  const cwd = await $.session.cwd().catch(() => '')
  if (cwd !== '') bases.push(cwd)
  const root = await $.session.root().catch(() => '')
  if (root !== '' && root !== cwd) bases.push(root)
  const pathVar = (await $.env.get('PATH')) ?? (await $.env.get('Path')) ?? ''
  for (const p of exeCandidates(configured, bases, pathVar)) {
    const ok = await $.fs.exists(p).catch(() => false)
    if (ok) return { path: p, why: null }
  }
  return { path: null, why: exeMissing(configured) }
}

async function estimate($: $, exe: string | null, steps: Step[]): Promise<EstimateOut> {
  if (exe === null) return readEstimate(steps, 'effort.exe 를 못 찾아 예상을 못 받음')
  return readEstimate(steps, await runEffort($, exe, estimateArgs(steps), ESTIMATE_TIMEOUT_MS))
}

/** show 결과 + exe 를 아예 못 돌렸는지 (그때만 전체 갱신을 멈춘다) */
async function show($: $, exe: string, id: string): Promise<ShowOut & { isExeFail: boolean }> {
  if (!MARK_ID_RE.test(id)) return { block: null, error: `mark id 꼴이 아님 : ${id}`, isExeFail: false }
  const ran = await runEffort($, exe, ['mark', 'show', id], SHOW_TIMEOUT_MS)
  return { ...readShow(id, ran), isExeFail: typeof ran === 'string' }
}

/** ~/.effort/marks.txt 를 읽기만 한다. 없거나 못 읽으면 null. */
async function readMarks($: $): Promise<string | null> {
  const home = (await $.env.get('USERPROFILE')) ?? (await $.env.get('HOME'))
  const p = marksPathFrom(await $.env.get('EFFORT_HOME'), home)
  if (p === null) return null
  return $.fs.read(p).catch(() => null)
}

/** 그릴 때 읽는 값 모음 (읽기만 한다) */
async function viewData($: $): Promise<ViewData> {
  return {
    plan: await read($, planAtom),
    marks: await read($, marksAtom),
    exe: await read($, exeAtom),
    paused: await read($, pausedAtom),
    ui: await read($, uiAtom),
    now: await $.clock.now(),
    redRatio: cfg.redRatio,
  }
}

/** 손으로 다시 재기 (/steps refresh · 패널 버튼). 다른 갱신이 돌고 있어 못 했으면 false. */
async function refreshNow($: $): Promise<boolean> {
  await update($, pausedAtom, () => false)
  exeFails = 0
  // 실패로 뺐던 mark 도 한 번 더 기회를 준다
  await update($, marksAtom, m => {
    const out: Record<string, MarkInfo> = {}
    for (const [id, one] of Object.entries(m)) out[id] = one.fails >= MAX_FAILS ? { ...one, fails: 0 } : one
    return out
  })
  await syncFromFile($)
  const ran = await refresh($)
  await saveCopy($)
  return ran
}

async function sessionId($: $): Promise<string> {
  return $.session.id().catch(() => 'unknown')
}

async function ensureExe($: $): Promise<string | null> {
  const cur = await read($, exeAtom)
  if (cur.path !== null) return cur.path
  const found = await findExe($, cfg.effortPath)
  await update($, exeAtom, () => found)
  return found.path
}

/** 계획과 그 mark 를 세션 키로 store 에 남긴다 (--resume 때 되살림). */
async function saveCopy($: $): Promise<void> {
  const plan = await read($, planAtom)
  if (plan === null) return
  const marks = await read($, marksAtom)
  const own: Record<string, MarkInfo> = {}
  for (const s of plan.steps) {
    for (const id of s.markIds) {
      const m = marks[id]
      if (m !== undefined) own[id] = m
    }
  }
  // 짝 없는 mark 도 최근 것 몇 개는 같이 남긴다. 빼면 리로드 뒤 사라져, 소단계를 더해 plan 을 다시 불러도 붙일 것이 없다
  const orphans = Object.values(marks).filter(m => m.n === null).sort((a, b) => a.seenAt - b.seenAt).slice(-ORPHANS_KEEP)
  for (const m of orphans) own[m.id] = m
  try {
    const mine = `plan:${plan.sessionId}`
    await $.store.set(mine, { plan, marks: own })
    // 오래된 남의 세션 사본만 지운다 (지금 세션 키는 고르지 않는다)
    const others = (await $.store.keys()).filter(k => k.startsWith('plan:') && k !== mine)
    for (const k of others.slice(0, Math.max(0, others.length - (PLAN_KEYS_KEEP - 1)))) await $.store.delete(k)
  } catch {
    // store 가 가득 차거나 막혀도 띠는 그대로 돈다
  }
}

/** mark 하나를 marks 에 넣고, 계획 번호가 맞으면 붙인다. */
async function addMark($: $, mark: MarkInfo, startedAt: number): Promise<void> {
  const plan = await read($, planAtom)
  const n = pairOf(plan, mark.name, startedAt)
  const one: MarkInfo = { ...mark, n }
  const marks = await update($, marksAtom, m => {
    const old = m[one.id]
    // 이미 아는 mark 면 잰 값은 두고 짝·갈래만 고친다
    return { ...m, [one.id]: old === undefined ? one : { ...old, n, agentId: old.agentId ?? one.agentId } }
  })
  if (plan !== null && n !== null) {
    await update($, planAtom, p => (p === null ? p : settle(attach(p, n, one.id), marks)))
  }
}

/** show·stop 덩이를 아는 mark 에 옮긴다. */
async function applyShow($: $, block: ShowBlock, now: number): Promise<void> {
  const marks = await update($, marksAtom, m => {
    const old = m[block.id]
    if (old === undefined) return m
    const same = old.recordText === block.recordText ? old.same + 1 : 0
    return {
      ...m,
      [block.id]: {
        ...old,
        open: block.open,
        recordText: block.recordText,
        recordMs: block.recordMs,
        waitText: block.waitText,
        flags: block.flags,
        warn: block.warn,
        error: null,
        fails: 0,
        same,
        lastAt: now,
      },
    }
  })
  await update($, planAtom, p => (p === null ? p : settle(p, marks)))
}

async function markFailed($: $, id: string, error: string): Promise<number> {
  let fails = 0
  await update($, marksAtom, m => {
    const old = m[id]
    if (old === undefined) return m
    fails = old.fails + 1
    return { ...m, [id]: { ...old, error, fails } }
  })
  return fails
}

// ── 다시 재기 (타이머·명령·관찰이 부른다) ─────────────────────

/**
 * marks.txt 보조 길 : 이 세션이 계획 뒤에 연 번호 붙은 mark 중 못 본 것을 붙인다. 읽기만 한다.
 * 열린 줄 알았던 mark 가 파일에서 stop 됐으면 다음 refresh 의 show 가 닫힌 값을 읽는다.
 */
async function syncFromFile($: $): Promise<void> {
  const plan = await read($, planAtom)
  if (plan === null) return
  const text = await readMarks($)
  if (text === null) return
  const known = await read($, marksAtom)
  for (const l of pickFromMarksFile(parseMarksFile(text), plan.sessionId, plan, known)) {
    const agent = l.file.includes('/') ? l.file.slice(l.file.indexOf('/') + 1) : null
    // 파일에서 처음 본 닫힌 mark 는 값(recordText)이 없어 다음 refresh 대상에 든다 (dueIds)
    await addMark($, { ...newMark(l.id, l.name, null, agent, l.at), open: l.stopAt === null }, l.at)
  }
}

/**
 * 다시 잴 mark : 열린 것, 그리고 닫혔는데 값을 아직 못 읽은 것 (marks.txt 에서 처음 본 것 등).
 * show 가 MAX_FAILS 번 잇달아 실패한 mark 는 뺀다.
 */
function dueIds(marks: Record<string, MarkInfo>): string[] {
  return Object.values(marks)
    .filter(m => (m.open || m.recordText === null) && m.fails < MAX_FAILS)
    .map(m => m.id)
}

/** 다시 재야 할 mark(또는 고른 mark)를 mark show 로 다시 잰다. 다른 갱신이 돌고 있어 건너뛰었으면 false. */
async function refresh($: $, only?: readonly string[]): Promise<boolean> {
  if (busy) return false
  busy = true
  try {
    if (await read($, pausedAtom)) return true
    const marks = await read($, marksAtom)
    const ids = (only ?? dueIds(marks)).slice(0, MAX_OPEN_SHOWS)
    if (ids.length === 0) return true
    const exe = await ensureExe($)
    if (exe === null) return true
    for (const id of ids) {
      const out = await show($, exe, id)
      const now = await $.clock.now()
      if (out.block !== null) {
        exeFails = 0
        await applyShow($, out.block, now)
        continue
      }
      const fails = await markFailed($, id, out.error ?? 'show 실패')
      if (out.isExeFail) {
        // exe 를 아예 못 돌렸다 : 다른 mark 도 같을 테니 잇달아 실패하면 전체를 멈춘다
        exeFails++
        if (exeFails >= MAX_FAILS) {
          await update($, pausedAtom, () => true)
          $.ui.toast(`effort-steps: effort.exe 실행이 ${MAX_FAILS}번 잇달아 실패해 갱신을 멈췄다. /steps refresh 로 다시 켠다`)
          break
        }
        continue
      }
      exeFails = 0
      if (fails === MAX_FAILS) {
        // 이 mark 하나만 갱신에서 뺀다. 다른 mark 는 계속 잰다
        $.ui.toast(`effort-steps: ${id} 의 mark show 가 ${MAX_FAILS}번 잇달아 실패해 이 mark 만 갱신에서 뺐다. /steps refresh 로 다시 잰다`)
      }
    }
    return true
  } finally {
    busy = false
  }
}

async function tick($: $): Promise<void> {
  try {
    await syncFromFile($)
    await refresh($)
    await saveCopy($)
  } catch {
    // 타이머 한 번이 실패해도 다음 주기는 돈다
  }
}

/** 타이머를 건다. 리로드 뒤에는 모듈 변수가 비므로 여러 이벤트에서 부른다. */
function armTimer($: $): void {
  if (timer !== null) return
  timer = $.clock.every(cfg.refreshMs, () => {
    void tick($)
  })
}

// ── 관찰 : Claude 가 친 effort mark 명령의 결과를 읽는다 ─────────

async function observe($: $, agentId: string | null, command: string, ran: ToolCallResult): Promise<void> {
  if (ran.deny !== undefined) return
  const calls = parseCommand(command)
  if (calls.length === 0) return
  const blocks = parseBlocks(outputText(ran))
  const now = await $.clock.now()
  const known = await read($, marksAtom)
  const fresh: string[] = []
  const hasStop = calls.some(c => c.verb === 'stop')
  const stopIds = calls.filter(c => c.verb === 'stop' && c.arg !== null).map(c => c.arg!)

  for (const b of blocks) {
    if (b.kind === 'start') {
      await addMark($, newMark(b.id, b.name, null, agentId, now), now)
      fresh.push(b.id)
    } else if (known[b.id] !== undefined || fresh.includes(b.id)) {
      await applyShow($, b, now)
    } else if (hasStop) {
      // 모르던 mark 의 stop (리로드 전이나 관찰 밖에서 열린 것) : 시작 시각으로 짝을 정한다
      await addMark($, { ...newMark(b.id, b.name, null, agentId, now), open: b.open }, b.startAt ?? now)
      await applyShow($, b, now)
    }
  }

  // 보조 길 : start 를 쳤는데 결과에서 못 읽었으면 marks.txt 에서 같은 이름을 찾는다
  const startNames = calls.filter(c => c.verb === 'start' && c.arg !== null).map(c => c.arg!)
  if (startNames.length > blocks.filter(b => b.kind === 'start').length) {
    const fileText = await readMarks($)
    const sid = await sessionId($)
    if (fileText !== null) {
      const have = await read($, marksAtom)
      const lines = parseMarksFile(fileText).filter(l =>
        l.stopAt === null && have[l.id] === undefined && startNames.includes(l.name) &&
        (l.file === sid || l.file.startsWith(`${sid}/`)))
      for (const l of lines.slice(-startNames.length)) {
        await addMark($, newMark(l.id, l.name, null, agentId, l.at), l.at)
        fresh.push(l.id)
      }
    }
  }

  // 새로 연 mark 는 바로 한 번 재고, 결과를 못 읽은 stop 은 show 로 마지막 값을 읽는다
  const shown = new Set(blocks.map(b => b.id))
  const later = [...fresh, ...stopIds.filter(id => !shown.has(id) && known[id] !== undefined)]
  if (later.length > 0) {
    // 도구 결과를 붙잡지 않으려고 타이머로 미룬다
    $.clock.after(50, () => {
      void refresh($, later).then(() => saveCopy($)).catch(() => undefined)
    })
  } else {
    await saveCopy($)
  }
}

// ── 강제 : plan 없는 편집·Agent ─────────────────────────────

/** 붙일 경고 줄과 그 종류 (턴마다 종류별 한 번) */
type GateLine = { text: string; kind: 'warned' | 'noMarkWarned' }

/**
 * 편집·Agent 한 번의 판정. path 는 편집 대상(Agent·spawn 이면 null).
 * 막을 글, 아니면 돈 뒤에 붙일 경고 줄을 돌려준다.
 */
async function gateCall($: $, tool: string, agentId: string | undefined, path: string | null): Promise<{ deny: string | null; line: GateLine | null }> {
  // 서브·비대화형·off 는 env 도 안 읽고 바로 지나간다 (판정 표 첫 칸)
  if (agentId !== undefined || cfg.mode === 'off' || !(await read($, interactiveAtom))) return { deny: null, line: null }
  let skip = false
  if (path !== null) {
    const home = (await $.env.get('USERPROFILE')) ?? (await $.env.get('HOME')) ?? null
    const temp = (await $.env.get('TEMP')) ?? (await $.env.get('TMP')) ?? null
    skip = isSkipPath(path, home, temp)
  }
  const verdict = judgeGate({
    interactive: true, mode: cfg.mode, agentId, skip,
    plan: await read($, planAtom), sessionId: await sessionId($), allowed: (await read($, allowAtom)) !== null,
  })
  if (verdict === 'deny') return { deny: denyText(tool), line: null }
  if (verdict === 'warn') return { deny: null, line: { text: warnLine(tool), kind: 'warned' } }
  // 계획은 있는데 열린 mark 가 없으면 메인 편집에만 한 줄 (Agent 띄우기는 서브가 자기 판에서 mark 를 연다)
  if (verdict === 'planned' && path !== null && !hasOpenMark(await read($, marksAtom))) {
    return { deny: null, line: { text: noMarkLine(), kind: 'noMarkWarned' } }
  }
  return { deny: null, line: null }
}

const OPEN_GATE: { deny: string | null; line: GateLine | null } = { deny: null, line: null }

/**
 * 이 mod 몫의 일(판정·기록)만 감싼다. 던지면 fallback 을 돌려준다(fail-open).
 * next(e) 는 이 밖에서 정확히 한 번 부르고, 그 예외는 그대로 올린다.
 */
async function failOpen<T>(work: () => Promise<T>, fallback: T): Promise<T> {
  try {
    return await work()
  } catch {
    return fallback
  }
}

/** 돈 뒤 : 다른 쪽이 막지 않았고 이번 턴 그 종류 첫 번이면 도구 결과에 한 줄을 붙인다 */
async function warnOnce($: $, ran: ToolCallResult, line: GateLine): Promise<ToolCallResult> {
  if (ran.deny !== undefined) return ran
  // 첫 번 판정은 update 콜백 안에서 한다. 따로 read 하면 병렬 편집 둘이 둘 다 「첫」 으로 읽는다
  let first = false
  await update($, turnAtom, t => {
    first = !t[line.kind]
    return { ...t, [line.kind]: true }
  })
  if (!first) return ran
  return { ...ran, context: [...(ran.context ?? []), line.text] } as ToolCallResult
}

/** 편집 셋(Edit·Write·NotebookEdit) 공용 몸통 */
async function gatedEdit($: $, tool: string, agentId: string | undefined, path: string, run: () => Promise<ToolCallResult>): Promise<ToolCallResult> {
  const g = await failOpen(() => gateCall($, tool, agentId, path), OPEN_GATE)
  if (g.deny !== null) return { deny: g.deny } as ToolCallResult
  const ran = await run()
  return g.line === null ? ran : warnOnce($, ran, g.line).catch(() => ran)
}

/** 턴 끝 : 그 갈래(메인이면 null)가 연 mark 중 안 닫힌 것을 toast 한다. mark 하나에 한 번만 */
async function toastUnclosed($: $, agentId: string | null): Promise<void> {
  if (cfg.mode === 'off' || !(await read($, interactiveAtom))) return
  const open = unclosedMarks(await read($, marksAtom), agentId, await read($, toastedAtom))
  if (open.length === 0) return
  await update($, toastedAtom, t => [...t, ...open.map(m => m.id)].slice(-200))
  $.ui.toast(unclosedToast(open))
}

/** /steps status 글 */
async function gateStatus($: $): Promise<string> {
  const plan = await read($, planAtom)
  const turn = await read($, turnAtom)
  return statusText({
    mode: cfg.mode,
    interactive: await read($, interactiveAtom),
    hasPlan: plan !== null,
    planSessionOk: plan !== null && plan.sessionId === (await sessionId($)),
    allDone: plan !== null && allDone(plan),
    allowed: (await read($, allowAtom)) !== null,
    warned: turn.warned,
    noMarkWarned: turn.noMarkWarned,
  })
}

// ── 도구 (모델이 부른다) ─────────────────────────────────────

async function planTool($: $, input: Record<string, unknown>): Promise<string> {
  const title = clean(input.title, 80) || '이름 없는 작업'
  const raw: unknown[] = Array.isArray(input.steps) ? input.steps.slice(0, MAX_STEPS) : []
  const seen = new Set<number>()
  const steps: Step[] = []
  for (const r of raw) {
    if (r === null || typeof r !== 'object') continue
    const o = r as Record<string, unknown>
    const n = typeof o.n === 'number' && Number.isInteger(o.n) && o.n >= 1 && o.n <= 999 ? o.n : steps.length + 1
    if (seen.has(n)) return `소단계 번호 ${n} 이 두 번 나온다. 번호는 서로 달라야 한다.`
    seen.add(n)
    const cls = CLASSES.find(c => c === o.class)
    if (cls === undefined) return `소단계 ${n} 의 class 「${clean(o.class, 10)}」 를 모른다. ${CLASSES.join('·')} 중 하나.`
    const size = SIZES.find(s => s === o.size) ?? 'M'
    steps.push({
      n, name: clean(o.name, 40) || `소단계${n}`, cls, size,
      estText: '—', estMs: null, markIds: [], status: '할 일', note: null,
    })
  }
  if (steps.length === 0) return '소단계가 없다. steps 에 하나 이상 넣는다.'

  const exe = await ensureExe($)
  const est = await estimate($, exe, steps)
  const now = await $.clock.now()
  const sid = await sessionId($)
  const old = await read($, planAtom)
  if (old !== null) await archive($, old)

  let plan: Plan = {
    id: `p${now.toString(36)}`, title, createdAt: now, sessionId: sid, steps: est.steps, estNote: est.note,
    ...(old !== null ? { prevAt: old.createdAt } : {}),
  }
  // 갈아 끼우기 : 옛 계획 시절의 mark(붙어 있던 것·짝 없이 남은 것, 열림·닫힘 모두)는 번호로 다시 짝짓는다.
  // 나머지는 계획 바로 앞에 연 mark (짝 문턱 안) 만 붙인다. 규칙은 pair.ts pairOnReplan.
  const pairNow = (p: Plan, m: MarkInfo) => pairOnReplan(old, p, m)
  const marks = await read($, marksAtom)
  for (const m of Object.values(marks)) {
    const n = pairNow(plan, m)
    if (n !== null) plan = attach(plan, n, m.id)
  }
  plan = settle(plan, marks)
  const fixed = plan
  await update($, planAtom, () => fixed)
  await update($, marksAtom, m => {
    const out: Record<string, MarkInfo> = {}
    for (const [id, one] of Object.entries(m)) out[id] = { ...one, n: pairNow(fixed, one) }
    return out
  })
  await update($, pausedAtom, () => false)
  // 상태에 없는 mark(리로드로 잃은 짝 없는 mark 등)를 marks.txt 에서 바로 주워 온다.
  // 값은 다음 갱신(바로 아래 타이머)의 mark show 가 채운다
  const before = Object.keys(await read($, marksAtom)).length
  await syncFromFile($)
  if (Object.keys(await read($, marksAtom)).length > before) {
    $.clock.after(50, () => {
      void refresh($).then(() => saveCopy($)).catch(() => undefined)
    })
  }
  await saveCopy($)
  armTimer($)
  if (cfg.autoPane) void $.ui.open({ id: PANE, title: PANE_TITLE })

  return [
    markdownTable((await read($, planAtom)) ?? fixed, await read($, marksAtom), cfg.redRatio),
    '',
    '소단계마다 `effort mark start "<번호>-<이름>"` 을 치면 띠의 실제 칸이 채워진다 (번호가 위 표의 번호와 같아야 한다).',
  ].join('\n')
}

async function stepTool($: $, input: Record<string, unknown>): Promise<string> {
  const plan = await read($, planAtom)
  if (plan === null) return '계획이 없다. 먼저 plan 도구를 부른다.'
  const n = typeof input.n === 'number' ? input.n : NaN
  if (!plan.steps.some(s => s.n === n)) return `계획에 ${clean(input.n, 10)} 번 소단계가 없다.`
  const status = STATUSES.find(s => s === input.status)
  const note = input.note === undefined ? undefined : clean(input.note, 120)
  // 손으로 정한 상태(끝 말고)는 mark 보다 앞선다 : 붙은 mark 를 떼어 짝 없는 mark 로 돌린다.
  // 끝이면 mark 가 있어도 그대로 둔다 (mark 가 닫혀 있으면 어차피 끝이다)
  const detach = status !== undefined && status !== '끝'
  const freed = detach ? (plan.steps.find(s => s.n === n)?.markIds ?? []) : []
  const marks = freed.length === 0
    ? await read($, marksAtom)
    : await update($, marksAtom, m => {
        const out = { ...m }
        for (const id of freed) if (out[id] !== undefined) out[id] = { ...out[id]!, n: null }
        return out
      })
  const next = await update($, planAtom, p => {
    if (p === null) return p
    const steps = p.steps.map(s =>
      s.n === n
        ? {
            ...s,
            markIds: detach ? [] : s.markIds,
            status: status ?? s.status,
            note: note === undefined ? s.note : note || null,
          }
        : s,
    )
    // 뗀 소단계는 markIds 가 비어 settle 이 손 상태를 그대로 둔다
    return settle({ ...p, steps }, marks)
  })
  await saveCopy($)
  const after = next?.steps.find(s => s.n === n)
  const freedText = freed.length > 0 ? ` · 붙어 있던 mark ${freed.length}개를 뗐다` : ''
  return `${n}번 소단계 : ${after?.status ?? '?'}${after?.note ? ` · ${after.note}` : ''}${freedText} (실제 값은 mark 로만 채운다)`
}

async function tableText($: $): Promise<string> {
  const plan = await read($, planAtom)
  if (plan === null) return '계획이 없다. plan 도구로 소단계를 넣는다.'
  return markdownTable(plan, await read($, marksAtom), cfg.redRatio)
}

/** 끝난 계획을 store history 로 옮긴다 (최근 30개). */
async function archive($: $, plan: Plan): Promise<void> {
  const t = totals(plan, await read($, marksAtom))
  try {
    const prev = await $.store.get('history')
    const list: unknown[] = Array.isArray(prev) ? prev : []
    const row = { title: plan.title, at: plan.createdAt, steps: plan.steps.length, done: t.done, estMs: t.estMs, actualMs: t.actualMs }
    await $.store.set('history', [...list, row].slice(-HISTORY_KEEP))
    await $.store.delete(`plan:${plan.sessionId}`)
  } catch {
    // 기록 실패는 띠에 영향 없음
  }
}

export const register: Register = (on, options) => {
  cfg = readSettings(options)
  timer = null
  busy = false
  exeFails = 0

  // ── 이벤트 잇기 ──────────────────────────────────────────────

  on('session.start', async ($, e, next) => {
    await $.tool.register({
      name: 'plan',
      description:
        '작업을 소단계로 나눈 계획을 넣는다. effort estimate 로 예상을 채운 「소단계/예상/실제/상태」 표를 돌려주고, 프롬프트 위 띠에 진행을 보여 준다. ' +
        '그 뒤 소단계마다 `effort mark start "<번호>-<이름>"`·`mark stop <id>` 를 치면 실제 칸이 자동으로 채워진다. 다시 부르면 계획을 갈아 끼운다. ' +
        '편집(Edit·Write·NotebookEdit)·Agent 를 쓰기 전에 먼저 부른다 — 계획이 없거나 소단계가 전부 끝·건너뜀이면 그 도구가 막힌다. 메인 세션만 부른다.',
      inputSchema: PLAN_SCHEMA,
    })
    await $.tool.register({
      name: 'step',
      description: 'mark 없이 한 소단계의 상태(끝·건너뜀·하는 중·할 일)와 까닭 한 줄을 적는다. 시간은 받지 않는다 — 실제 값은 mark 로만 채운다. ' +
        '끝 말고 상태를 주면 그 소단계에 붙은 mark 를 떼어 짝 없는 mark 로 돌린다 (잘못 붙은 mark 를 뗄 때). 끝이면 mark 는 그대로 둔다.',
      inputSchema: STEP_SCHEMA,
    })
    await $.tool.register({
      name: 'table',
      description: '지금 공수 표를 마크다운으로 돌려준다. 완료 보고의 공수 표로 그대로 쓴다.',
      inputSchema: { type: 'object', properties: {} },
    })
    await $.command.register({
      name: 'steps',
      description: '공수 소단계 패널 열기/닫기 · table · refresh · hide · show · clear · allow(다음 말 한 번 plan 없이 허용) · status',
      argumentHint: '[table|refresh|hide|show|clear|allow|status]',
    })
    await update($, interactiveAtom, () => e.isInteractive === true)

    // --resume 이면 store 의 계획 사본을 되살린다
    if ((await read($, planAtom)) === null) {
      const sid = await sessionId($)
      const copy: unknown = await $.store.get(`plan:${sid}`).catch(() => undefined)
      if (copy !== null && typeof copy === 'object' && 'plan' in copy && 'marks' in copy) {
        const c = copy as { plan: Plan; marks: Record<string, MarkInfo> }
        await update($, marksAtom, m => ({ ...c.marks, ...m }))
        await update($, planAtom, () => c.plan)
      }
    }
    // prefs 에 사람이 /steps show|hide 로 고른 값이 있을 때만 기본(숨김)을 덮는다
    const prefs: unknown = await $.store.get('prefs').catch(() => undefined)
    if (prefs !== null && typeof prefs === 'object' && 'bandHidden' in prefs) {
      const hidden = (prefs as { bandHidden: unknown }).bandHidden === true
      await update($, uiAtom, u => ({ ...u, bandHidden: hidden }))
    }
    await ensureExe($)
    armTimer($)
    return next(e)
  })

  on('turn.start', ($, e, next) => {
    armTimer($)
    return next(e)
  })

  // 자기 도구는 스스로 답한다. 안에서 무엇이 터져도 모델에게는 글 하나로 답한다.
  const failed = { result: 'effort-steps: 처리 중 오류가 났다. /steps refresh 뒤 다시 부른다.' }
  // 서브에이전트의 plan 은 거절한다 : 작업서에 「plan 넣어라」 를 적은 서브가 메인 계획을 덮지 않게
  on('tool.call', { tool: PLAN_TOOL }, async ($, e) =>
    e.agentId !== undefined ? { result: subPlanText() } : { result: await planTool($, e as unknown as Record<string, unknown>) })
    .catch(() => failed)
  on('tool.call', { tool: STEP_TOOL }, async ($, e) => ({ result: await stepTool($, e as unknown as Record<string, unknown>) }))
    .catch(() => failed)
  on('tool.call', { tool: TABLE_TOOL }, async $ => ({ result: await tableText($) }))
    .catch(() => failed)

  // Bash · PowerShell 결과를 본다. 결과는 언제나 손대지 않고 그대로 돌려준다.
  on('tool.call', { tool: 'Bash' }, async ($, e, next) => {
    const ran = await next(e)
    await failOpen(async () => {
      armTimer($)
      await observe($, e.agentId ?? null, e.command, ran)
    }, undefined)
    return ran
  })
  on('tool.call', { tool: 'PowerShell' }, async ($, e, next) => {
    const ran = await next(e)
    await failOpen(async () => {
      armTimer($)
      await observe($, e.agentId ?? null, e.command, ran)
    }, undefined)
    return ran
  })

  // ── 강제 훅 : 모두 fail-open ─────────────────────────────────

  // 사람이 터미널에서 친 말(슬래시 아님)이면 이번 턴 경고 표시를 지운다.
  // 허용은 그 말 한 번을 살아남는다 : 아직 안 쓴 허용이면 「씀」 으로 두고, 이미 쓴 허용이면 지운다
  on('prompt.submit', async ($, e, next) => {
    if (e.origin.kind === 'composer' && !e.text.trimStart().startsWith('/')) {
      await failOpen(async () => {
        await update($, allowAtom, a => (a !== null && !a.used ? { ...a, used: true } : null))
        await update($, turnAtom, () => ({ warned: false, noMarkWarned: false }))
      }, undefined)
    }
    return next(e)
  })

  on('tool.call', { tool: 'Edit' }, ($, e, next) => gatedEdit($, 'Edit', e.agentId, e.file_path, () => next(e)))
  on('tool.call', { tool: 'Write' }, ($, e, next) => gatedEdit($, 'Write', e.agentId, e.file_path, () => next(e)))
  on('tool.call', { tool: 'NotebookEdit' }, ($, e, next) => gatedEdit($, 'NotebookEdit', e.agentId, e.notebook_path, () => next(e)))
  on('tool.call', { tool: 'Agent' }, async ($, e, next) => {
    const g = await failOpen(() => gateCall($, 'Agent', e.agentId, null), OPEN_GATE)
    if (g.deny !== null) return { deny: g.deny }
    const ran = await next(e)
    return g.line === null ? ran : warnOnce($, ran, g.line).catch(() => ran)
  })
  // Agent 도구를 안 지나는 띄우기(workflow agent() · $.agent.spawn). 메인에서 난 것은 parentAgentId 가 없다.
  // 경고 줄을 붙일 도구 결과가 없어 warn 이면 그냥 지나간다
  on('agent.spawn', async ($, e, next) => {
    const g = await failOpen(() => gateCall($, 'Agent', e.parentAgentId, null), OPEN_GATE)
    if (g.deny !== null) return { deny: g.deny }
    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    await failOpen(() => toastUnclosed($, e.agentId ?? null), undefined)
    return next(e)
  })

  // /clear 는 session.start 없이 세션 id 만 바뀐다. 판정은 세션 id 로 하므로 plan 은 저절로 「없음」, 허용은 여기서 지운다
  on('session.end', async ($, e, next) => {
    await failOpen(async () => {
      await update($, allowAtom, () => null)
      await update($, turnAtom, () => ({ warned: false, noMarkWarned: false }))
    }, undefined)
    return next(e)
  })

  on('command.run', { command: 'steps' }, async ($, e) => {
    armTimer($)
    const arg = e.args.trim().split(/\s+/)[0] ?? ''
    switch (arg) {
      case '': {
        const isOpen = (await $.ui.panes()).some(p => p.id === PANE)
        if (isOpen) {
          await $.ui.close({ id: PANE })
          return { text: '공수 패널을 닫았다.' }
        }
        await $.ui.open({ id: PANE, title: PANE_TITLE })
        return { text: '공수 패널을 열었다.' }
      }
      case 'table':
        return { text: await tableText($) }
      case 'refresh': {
        await update($, exeAtom, x => (x.path === null ? { path: null, why: null } : x))
        const ran = await refreshNow($)
        const exe = await read($, exeAtom)
        if (exe.path === null) return { text: `갱신 못 함 : ${exe.why ?? 'effort.exe 없음'}` }
        return { text: ran ? '열린 mark 를 다시 쟀다.' : '다른 갱신 중이라 이번엔 건너뛰었다. 잠시 뒤 다시 친다.' }
      }
      case 'hide':
      case 'show': {
        const hidden = arg === 'hide'
        await update($, uiAtom, u => ({ ...u, bandHidden: hidden }))
        await $.store.set('prefs', { bandHidden: hidden }).catch(() => undefined)
        return { text: hidden ? '공수 띠를 껐다. /steps show 로 켠다.' : '공수 띠를 켰다. /steps hide 로 끈다.' }
      }
      case 'clear': {
        const plan = await read($, planAtom)
        if (plan === null) return { text: '지울 계획이 없다.' }
        await archive($, plan)
        await update($, planAtom, () => null)
        await update($, marksAtom, () => ({}))
        await update($, pausedAtom, () => false)
        exeFails = 0
        return { text: `「${plan.title}」 계획을 기록으로 옮기고 띠를 비웠다.` }
      }
      case 'allow': {
        // 모델이 스스로 풀 수 없게 사람이 터미널에서 친 것만 받는다
        if (e.origin.kind !== 'composer') return { text: '/steps allow 는 사람이 터미널에서 친 것만 받는다.' }
        const now = await $.clock.now()
        await update($, allowAtom, () => ({ at: now, used: false }))
        return { text: '다음 사람 말 한 번(그 턴 끝까지) plan 없이 편집·Agent 를 허용했다. 그 다음 말에 다시 막는다.' }
      }
      case 'status':
        return { text: await gateStatus($) }
      default:
        return { text: '쓰는 법 : /steps [table|refresh|hide|show|clear|allow|status] — 인자 없이 치면 패널을 열고 닫는다. allow = 다음 사람 말 한 번 plan 없이 허용, status = 강제 상태.' }
    }
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    if (e.props.hasSurvey) return next(e)
    // 띠 자리는 플러그인 전체에 하나다. 아래 플러그인(prompt-gauge 등)의 띠가 있으면 내 것 아래에 포갠다.
    // 엔진 자리(type 'engine')는 비켜 줄 때만 쓰는 것이라 버린다. next 는 한 번만 부른다.
    // 아래 띠가 있으면 그 띠가 실제로 차지하는 줄 수(countRows)를 내 띠의 줄 수에서 먼저 뺀다.
    const below = await next(e)
    const hasBelow = below !== null && below !== undefined && below.type !== 'engine'
    const els = $.ui.resolve(e) as unknown as Els
    const tree = bandTree(els, await viewData($), { ...e.props, belowRows: hasBelow ? countRows(below) : 0 })
    if (tree === null) return below
    if (!hasBelow) return tree
    const { Box } = els
    return (
      <Box key="effort-steps-stack" flexDirection="column">
        <Box key="effort-steps" flexDirection="column">{tree}</Box>
        <Box key="below" flexDirection="column">{below}</Box>
      </Box>
    )
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const els = $.ui.resolve(e) as unknown as Els
    return paneTree(els, await viewData($), e.props.bodyColumns, {
      toggleExpanded: () => update($, uiAtom, u => ({ ...u, expanded: !u.expanded })),
      refreshNow: async () => {
        if (!(await refreshNow($))) $.ui.toast('effort-steps: 다른 갱신 중이라 이번엔 건너뛰었다. 잠시 뒤 다시 누른다')
      },
    })
  })
}

/** 도구 결과에서 글을 꺼낸다 : core 가 주는 text, 없으면 stdout. */
function outputText(ran: ToolCallResult): string {
  if (typeof ran.text === 'string' && ran.text !== '') return ran.text
  const r: unknown = ran.result
  if (r !== null && typeof r === 'object' && 'stdout' in r && typeof r.stdout === 'string') return r.stdout
  return typeof r === 'string' ? r : ''
}
