// 마크다운 공수 표. 순수 함수만 둔다.
import type { MarkInfo, Plan } from '../types'
import { stepActual } from './pair'
import { minutesText } from './parse'

/** 실제/예상 비. 둘 중 하나라도 없으면 null. */
export function ratioOf(estMs: number | null, actualMs: number | null): number | null {
  if (estMs === null || actualMs === null || estMs <= 0) return null
  return actualMs / estMs
}

/** 어긋남 한 마디. 문턱 안이면 null. */
export function driftText(ratio: number | null, redRatio: number): string | null {
  if (ratio === null) return null
  if (ratio > redRatio) return `예상의 ${ratio.toFixed(1)}배`
  if (ratio < 1 / redRatio) return `예상의 ${Math.round(ratio * 100)}%`
  return null
}

/** 표 칸에 넣을 글 : 세로줄이 표를 깨지 않게 바꾼다. */
function cell(text: string): string {
  return text.replace(/\|/g, '｜')
}

/** 합계 : 예상 합과 끝난·하는 중 실제 합. */
export function totals(plan: Plan, marks: Record<string, MarkInfo>): {
  estMs: number | null
  actualMs: number | null
  done: number
  hasOpen: boolean
} {
  let est = 0
  let hasEst = false
  let act = 0
  let hasAct = false
  let done = 0
  let hasOpen = false
  for (const s of plan.steps) {
    if (s.estMs !== null) {
      est += s.estMs
      hasEst = true
    }
    const a = stepActual(s, marks)
    if (a.ms !== null) {
      act += a.ms
      hasAct = true
    }
    if (a.isOpen) hasOpen = true
    if (s.status === '끝' || s.status === '건너뜀') done += 1
  }
  return { estMs: hasEst ? est : null, actualMs: hasAct ? act : null, done, hasOpen }
}

/**
 * 「소단계 / 예상 / 실제 / 상태」 표. 어긋난 줄이나 까닭이 있는 줄이 있으면 「까닭」 칸을 더한다.
 * 끝에 어긋난 까닭은 mem history 로 남기라는 한 줄을 붙인다 (설계 11절).
 */
export function markdownTable(plan: Plan, marks: Record<string, MarkInfo>, redRatio: number): string {
  const rows = plan.steps.map(s => {
    const a = stepActual(s, marks)
    const drift = a.isOpen ? null : driftText(ratioOf(s.estMs, a.ms), redRatio)
    const why = [s.note, drift].filter((x): x is string => x !== null && x !== '').join(' · ')
    return {
      name: `${s.n}. ${s.name}`,
      est: s.estText,
      act: a.text + (a.isUnsure ? ' ?' : ''),
      status: s.status,
      why,
    }
  })
  const hasWhy = rows.some(r => r.why !== '')
  const head = hasWhy ? ['소단계', '예상', '실제', '상태', '까닭'] : ['소단계', '예상', '실제', '상태']
  const lines = [`| ${head.join(' | ')} |`, `| ${head.map(() => '---').join(' | ')} |`]
  for (const r of rows) {
    const cols = [r.name, r.est, r.act, r.status]
    if (hasWhy) cols.push(r.why)
    lines.push(`| ${cols.map(cell).join(' | ')} |`)
  }
  const t = totals(plan, marks)
  const sum = ['합계', minutesText(t.estMs), minutesText(t.actualMs) + (t.hasOpen ? ' (+하는 중)' : ''), `${t.done}/${plan.steps.length} 끝`]
  if (hasWhy) sum.push('')
  lines.push(`| ${sum.map(cell).join(' | ')} |`)

  const out = [`**${cell(plan.title)}**`, '', ...lines, '']
  if (plan.estNote !== null) out.push(`예상 : ${plan.estNote}`)
  out.push('실제 칸은 effort mark 의 「기록 구간」 그대로다. 어긋난 까닭은 `mem add --type history` 로 남긴다.')
  return out.join('\n')
}
