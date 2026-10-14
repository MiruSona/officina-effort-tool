// (순수) plan 없는 편집·위임을 막는 판정과 그 글. 엔진($)을 쓰지 않는다.
// 설계 : EffortTool/Docs/Design/2026-10-14-effort-steps이관·강제·embed설계.md 3절
//
// 판정 차례 (설계 3절 표 그대로) :
//   대화형 아님 · mode off · agentId 있음 · 빼는 자리 → 지나감
//   plan 있음 (같은 세션 id · 소단계가 전부 끝·건너뜀은 아님) → 지나감 (메인 편집이면 열린 mark 경고 검사)
//   /steps allow (다음 사람 말 한 번) → 지나감
//   mode warn → 지나감 + 그 턴 첫 번에 경고 한 줄
//   mode deny → 막음
import type { MarkInfo, Plan } from '../types'

export type Mode = 'deny' | 'warn' | 'off'

export const PLAN_TOOL = 'mcp__effort-steps__plan'

/** 판정에 드는 값 모음. main.tsx 가 $.state 와 이벤트에서 채워 넘긴다. */
export type GateInput = {
  interactive: boolean
  mode: Mode
  /** tool.call 의 agentId · agent.spawn 의 parentAgentId. 메인이면 undefined */
  agentId: string | undefined
  /** 빼는 자리(편집 대상이 ~/.claude · TEMP 아래)인가 */
  skip: boolean
  plan: Pick<Plan, 'sessionId' | 'steps'> | null
  /** 지금 $.session.id() */
  sessionId: string
  /** /steps allow 가 살아 있나 (다음 사람 말 한 번을 살아남는다) */
  allowed: boolean
}

/**
 * 판정 결과.
 * pass = 볼 대상이 아님 · planned = 계획 있음 · allowed = 사람이 /steps allow 로 풂 · warn = 지나가되 경고 · deny = 막음
 */
export type Verdict = 'pass' | 'planned' | 'allowed' | 'warn' | 'deny'

/**
 * 계획의 소단계가 모두 끝났나 (「끝」 또는 「건너뜀」 — 건너뛴 소단계도 더 할 일이 없다).
 * 새 판 시작 때 plan 을 다시 넣게 하려고 이때는 「plan 없음」으로 본다
 */
export function allDone(plan: Pick<Plan, 'steps'>): boolean {
  return plan.steps.length > 0 && plan.steps.every(s => s.status === '끝' || s.status === '건너뜀')
}

/** 「plan 있음」 = 같은 세션 id 의 계획이 있고 소단계가 전부 끝(끝·건너뜀)은 아님. /clear 뒤에는 세션 id 가 바뀌어 저절로 없음이 된다 */
export function planLive(plan: Pick<Plan, 'sessionId' | 'steps'> | null, sessionId: string): boolean {
  return plan !== null && plan.sessionId === sessionId && !allDone(plan)
}

export function judgeGate(g: GateInput): Verdict {
  if (!g.interactive || g.mode === 'off' || g.agentId !== undefined || g.skip) return 'pass'
  if (planLive(g.plan, g.sessionId)) return 'planned'
  if (g.allowed) return 'allowed'
  return g.mode === 'warn' ? 'warn' : 'deny'
}

/** 설정 칸 mode 풀기. 모르는 값은 deny */
export function readMode(v: unknown): Mode {
  return v === 'warn' || v === 'off' ? v : 'deny'
}

// ── 빼는 자리 ─────────────────────────────────────────────────

/** 역슬래시→슬래시, /c/x → c:/x, 끝 슬래시 떼기, 소문자. 비교 열쇠로만 쓴다 */
function key(p: string): string {
  let s = p.trim().replace(/\\/g, '/')
  const msys = /^\/([A-Za-z])(\/|$)/.exec(s)
  if (msys !== null) s = `${msys[1]!}:/${s.slice(3)}`
  return s.replace(/\/+$/, '').toLowerCase()
}

function under(p: string, root: string): boolean {
  const r = key(root)
  return r !== '' && (p === r || p.startsWith(`${r}/`))
}

/**
 * 편집 대상이 빼는 자리인가 : `<홈>/.claude` 아래(내장 기억 · 계획 파일) 또는 TEMP 아래(스크래치패드).
 * 절대 경로만 본다. `..` 조각이 끼거나 상대 경로·`~` 면 빼지 않는다(= 판정 쪽) — 흉내 낸 자리로 빠져나가지 못하게.
 */
export function isSkipPath(path: string, home: string | null, temp: string | null): boolean {
  const p = key(path)
  if (!/^[a-z]:\//.test(p) && !p.startsWith('/')) return false
  if (p.split('/').some(part => part === '..')) return false
  if (home !== null && home.trim() !== '' && under(p, `${key(home)}/.claude`)) return true
  return temp !== null && temp.trim() !== '' && under(p, temp)
}

// ── 글 ───────────────────────────────────────────────────────

/** 막을 때 모델이 받는 글 (설계 3절 그대로). tool 은 막힌 도구 이름 */
export function denyText(tool: string): string {
  return [
    `effort-steps : 이 세션에 공수 계획이 없어 막았다 (${tool}).`,
    `먼저 plan 도구로 소단계를 넣는다 — 도구 이름 ${PLAN_TOOL},`,
    `목록에 이름만 있으면 ToolSearch 로 "select:${PLAN_TOOL}" 을 먼저 불러온다.`,
    '질문에 답만 하는 턴이면 편집·Agent 를 쓰지 않는다.',
    '사람이 다음 말 한 번만 풀려면 /steps allow, 늘 경고로만 두려면 /config 의 mode.',
  ].join('\n')
}

/** mode warn 일 때 그 턴 첫 번에 도구 결과에 붙이는 한 줄 */
export function warnLine(tool: string): string {
  return `effort-steps : 이 세션에 공수 계획이 없다 (${tool}) — 지금은 경고만 한다 (mode warn). 먼저 plan 도구(${PLAN_TOOL})로 소단계를 넣는다.`
}

/** 계획은 있는데 열린 mark 가 하나도 없을 때 메인 편집에 붙이는 한 줄 */
export function noMarkLine(): string {
  return 'effort-steps : 계획은 있는데 열린 mark 가 없다. 소단계를 시작할 때 `effort mark start "<번호>-<이름>"` 을 친다 (실제 칸이 안 채워진다).'
}

/** 서브에이전트가 plan 도구를 부를 때 돌려주는 글. 서브가 메인 계획을 덮지 않게 한다 */
export function subPlanText(): string {
  return 'effort-steps : plan 은 메인 몫이다 — 서브에이전트는 계획을 넣거나 갈아 끼우지 않는다. 소단계 공수는 `effort mark start "<번호>-<이름>"`·`mark stop <id>` 로만 잰다.'
}

/** 안 닫힌 mark 알림 (toast). marks 는 비어 있지 않다 */
export function unclosedToast(marks: readonly Pick<MarkInfo, 'id' | 'name'>[]): string {
  const list = marks.map(m => `${m.name} (${m.id})`).join(', ')
  return `effort-steps : 안 닫힌 mark — ${list}. 끝났으면 effort mark stop <id>`
}

/** mark 의 갈래가 그 agentId 인가. marks.txt 에서 주운 mark 는 「agent-<id>」 꼴로 적혀 있다 */
function sameAgent(markAgent: string | null, agentId: string | null): boolean {
  if (agentId === null) return markAgent === null
  return markAgent === agentId || markAgent === `agent-${agentId}`
}

/**
 * toast 할 안 닫힌 mark : 그 갈래(메인이면 null)가 연 것 중 열린 것, 이미 알린 것은 뺀다.
 * 메인 턴 끝에 서브 mark 가 열려 있는 것은 정상이라 메인은 메인 것만 본다.
 */
export function unclosedMarks(marks: Readonly<Record<string, MarkInfo>>, agentId: string | null, toasted: readonly string[]): MarkInfo[] {
  return Object.values(marks)
    .filter(m => m.open && sameAgent(m.agentId, agentId) && !toasted.includes(m.id))
    .sort((a, b) => a.seenAt - b.seenAt)
}

/** 세션 안에 열린 mark 가 하나라도 있나 (갈래 상관없이) */
export function hasOpenMark(marks: Readonly<Record<string, MarkInfo>>): boolean {
  return Object.values(marks).some(m => m.open)
}

/** /steps status 글 */
export type StatusInput = {
  mode: Mode
  interactive: boolean
  hasPlan: boolean
  planSessionOk: boolean
  allDone: boolean
  allowed: boolean
  warned: boolean
  noMarkWarned: boolean
}

export function statusText(s: StatusInput): string {
  const plan = s.hasPlan
    ? s.planSessionOk
      ? s.allDone ? '있음 (소단계 전부 끝 — 「없음」으로 본다)' : '있음 (세션 id 같음)'
      : '있음 (세션 id 다름 — 「없음」으로 본다)'
    : '없음'
  return [
    `mode : ${s.mode}`,
    `대화형 : ${s.interactive ? '예' : '아니오 (강제 안 함)'}`,
    `plan : ${plan}`,
    `허용 중 : ${s.allowed ? '예 (/steps allow — 다음 사람 말 한 번, 그 턴 끝까지)' : '아니오'}`,
    `이번 턴 경고 : 계획 없음 ${s.warned ? 1 : 0} · 열린 mark 없음 ${s.noMarkWarned ? 1 : 0}`,
  ].join('\n')
}
