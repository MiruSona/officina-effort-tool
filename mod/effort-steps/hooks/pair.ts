// mark 이름 ↔ 소단계 번호 짝짓기. 순수 함수만 둔다.
import type { MarkInfo, Plan, Step, StepStatus } from '../types'
import type { MarkLine } from './parse'
import { minutesText } from './parse'

/** 계획보다 이만큼 먼저 열린 mark 까지는 그 계획 것으로 본다 (plan 을 start 바로 뒤에 부른 경우). */
export const PLAN_SLACK_MS = 60_000
/**
 * 이름 짝의 시각 하한 : 계획보다 이만큼 먼저 연 mark 까지만 이름으로 붙인다.
 * 없으면 같은 세션의 옛 작업 mark(같은 소단계 이름)가 새 계획에 붙는다.
 */
export const NAME_SLACK_MS = 10 * 60_000

/**
 * 이름 앞 번호. 「3-검사기」 → 3, 「4a-리뷰」 → 4, 「2A-조사」 → 2.
 * 번호로 시작하지 않으면 null (짝 없는 mark 로 남는다).
 */
export function stepNumber(name: string): number | null {
  const m = /^(\d{1,3})[A-Za-z]?(?:[-_.·: ]|$)/.exec(name.trim())
  return m ? Number(m[1]) : null
}

/** 이름 비교용 : 앞 번호(「5-」 「4a.」)와 빈칸을 빼고 소문자로 */
function bareName(s: string): string {
  return s.trim().replace(/^\d{1,3}[A-Za-z]?(?:[-_.·: ]+|$)/, '').replace(/\s+/g, '').toLowerCase()
}

/**
 * 이 mark 를 계획의 몇 번 소단계에 붙일지. 붙일 곳이 없으면 null.
 * ① 번호가 계획에 있고 계획 시각 − 60초 뒤에 열었으면 붙인다.
 * ② 그보다 먼저 열었어도 번호 뒤 이름이 그 소단계 이름과 같으면 붙인다 (「5-패널 손질」 ↔ 5번 「패널 손질」).
 *    계획을 나중에 다시 넣거나(소단계를 더함) 리로드로 상태를 잃은 뒤 marks.txt 에서 다시 주울 때 쓰인다.
 *    번호만 같고 이름이 다른 옛 mark(앞 계획의 다른 일)는 붙이지 않는다.
 *    단 시각 하한이 있다 : 계획 − 10분, 이전 계획이 있으면(prevAt) 그 계획 − 10분까지 넓힌다 (nameFloor).
 */
export function pairOf(plan: Plan | null, name: string, startedAt: number): number | null {
  if (plan === null) return null
  const n = stepNumber(name)
  if (n === null) return null
  const step = plan.steps.find(s => s.n === n)
  if (step === undefined) return null
  if (startedAt >= plan.createdAt - PLAN_SLACK_MS) return n
  if (startedAt < nameFloor(plan)) return null
  return sameName(name, step.name) ? n : null
}

/** 이름 짝의 시각 하한 : 계획 − 10분. 이전 계획이 있으면 이전 계획 − 10분까지 (둘 중 이른 쪽) */
export function nameFloor(plan: Plan): number {
  const base = plan.createdAt - NAME_SLACK_MS
  return plan.prevAt === undefined ? base : Math.min(base, plan.prevAt - NAME_SLACK_MS)
}

/** mark 이름(번호 뗀 것)이 소단계 이름과 같은가 */
function sameName(markName: string, stepName: string): boolean {
  return bareName(markName) !== '' && bareName(markName) === bareName(stepName)
}

/**
 * 시각은 보지 않고 번호만으로 짝짓는다. plan 을 다시 불러 계획을 갈아 끼울 때,
 * 옛 계획에 이미 붙어 있던 mark 를 새 계획에 다시 붙이는 데 쓴다.
 */
export function pairByNumber(plan: Plan, name: string): number | null {
  const n = stepNumber(name)
  return n !== null && plan.steps.some(s => s.n === n) ? n : null
}

/**
 * plan 을 다시 불러 계획을 갈아 끼울 때, marks 상태의 mark 하나를 새 계획 몇 번에 붙일지.
 * 옛 계획 시절에 본 mark(옛 계획에 붙어 있던 것, 또는 옛 계획 짝 문턱 뒤에 본 것)는 열린 것·닫힌 것 모두
 * 시각을 보지 않고 번호로 짝짓는다 — **옛·새 계획에서 그 번호의 소단계 이름이 같을 때만**.
 * 번호가 같아도 이름이 바뀌었으면 다른 일이라 번호로는 안 붙인다 (그 mark 는 이미 잰 옛 일의 값이다).
 * 옛 계획에 그 번호가 아예 없었으면(짝 없이 남은 mark) 번호로 붙인다 — 소단계를 더해 다시 부른 경우다.
 * 단 새 계획이 옛 계획을 이어 쓴 것(같은 번호·같은 이름 소단계가 하나라도 있음)일 때만이다.
 * 다른 일로 통째 갈아 끼운 계획에 옛 계획 밖 번호의 mark(앞 일의 「7-…」)를 붙이면 안 된다 (2026-10-10 실물 버그).
 * 그 밖(이름이 바뀐 번호 · 이어 쓰지 않은 계획 · 옛 계획이 없음 · 그보다 먼저 연 mark)은
 * 처음 계획 때와 같게 pairOf 로 본다 (짝 문턱 · 이름 짝 + 시각 하한).
 */
export function pairOnReplan(old: Plan | null, plan: Plan, m: MarkInfo): number | null {
  if (old !== null) {
    const wasAttached = old.steps.some(s => s.markIds.includes(m.id))
    const n = pairByNumber(plan, m.name)
    const before = n === null ? undefined : old.steps.find(s => s.n === n)
    const now = n === null ? undefined : plan.steps.find(s => s.n === n)
    const kept = now !== undefined &&
      (before === undefined ? continues(old, plan) : bareName(before.name) === bareName(now.name))
    if (kept && (wasAttached || m.seenAt >= old.createdAt - PLAN_SLACK_MS)) return n
  }
  return pairOf(plan, m.name, m.seenAt)
}

/** 새 계획이 옛 계획을 이어 쓴 것인가 : 같은 번호에 같은 이름인 소단계가 하나라도 있다 */
function continues(old: Plan, plan: Plan): boolean {
  return plan.steps.some(s => old.steps.some(o => o.n === s.n && bareName(o.name) === bareName(s.name)))
}

/** mark 하나를 계획에 붙인 새 계획. 이미 붙어 있으면 그대로. */
export function attach(plan: Plan, n: number, markId: string): Plan {
  return {
    ...plan,
    steps: plan.steps.map(s =>
      s.n === n && !s.markIds.includes(markId) ? { ...s, markIds: [...s.markIds, markId] } : s,
    ),
  }
}

/** 소단계의 실제 값. mark 하나면 그 원문, 여럿이면 합을 effort 글꼴로. */
export function stepActual(step: Step, marks: Record<string, MarkInfo>): {
  text: string
  ms: number | null
  isOpen: boolean
  isUnsure: boolean
} {
  const list = step.markIds.map(id => marks[id]).filter((m): m is MarkInfo => m !== undefined)
  if (list.length === 0) return { text: '—', ms: null, isOpen: false, isUnsure: false }
  const isOpen = list.some(m => m.open)
  const isUnsure = list.some(m => m.error !== null)
  const known = list.filter(m => m.recordMs !== null)
  const ms = known.length === 0 ? null : known.reduce((a, m) => a + (m.recordMs ?? 0), 0)
  let text: string
  if (list.length === 1) text = list[0]!.recordText ?? '—'
  else text = ms === null ? '—' : minutesText(ms)
  return { text, ms, isOpen, isUnsure }
}

/** mark 상태로 소단계 상태를 다시 셈한다. 건너뜀은 손으로 정한 것이라 그대로 둔다. */
export function statusOf(step: Step, marks: Record<string, MarkInfo>): StepStatus {
  if (step.status === '건너뜀') return step.status
  const list = step.markIds.map(id => marks[id]).filter((m): m is MarkInfo => m !== undefined)
  if (list.length === 0) return step.status
  return list.some(m => m.open) ? '하는 중' : '끝'
}

/** 모든 소단계 상태를 marks 에 맞춘다. */
export function settle(plan: Plan, marks: Record<string, MarkInfo>): Plan {
  return { ...plan, steps: plan.steps.map(s => ({ ...s, status: statusOf(s, marks) })) }
}

/**
 * marks.txt 보조 길 : 이 세션(갈래 포함)이 열었고, pairOf 로 짝이 맞고(계획 뒤에 시작했거나 이름이 같고),
 * 아직 모르는 mark 를 고른다. 묶기 실패(file 「-」) 줄은 어느 세션 것인지 몰라 건너뛴다.
 */
export function pickFromMarksFile(
  lines: readonly MarkLine[],
  sessionId: string,
  plan: Plan,
  known: Record<string, MarkInfo>,
): MarkLine[] {
  return lines.filter(l =>
    known[l.id] === undefined &&
    (l.file === sessionId || l.file.startsWith(`${sessionId}/`)) &&
    pairOf(plan, l.name, l.at) !== null,
  )
}

/** 새로 본 mark 의 처음 값 */
export function newMark(id: string, name: string, n: number | null, agentId: string | null, now: number): MarkInfo {
  return {
    id, name, n, agentId, open: true,
    recordText: null, recordMs: null, waitText: null, flags: [], warn: null,
    error: null, fails: 0, same: 0, seenAt: now, lastAt: now,
  }
}
