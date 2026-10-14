// 띠(AbovePrompt)와 패널(Pane) 그림. 순수 함수다 : 상태 읽기·$.ui.resolve 는 main.tsx 가 하고
// 여기에는 읽은 값과 요소 표만 넘어온다 (엔진 검사가 $ 를 import 너머로 넘기지 못하게 한다).
import type { Color, Elements, RenderElement } from 'claude-code'

import type { ExeInfo, MarkInfo, Plan, Step, StepStatus, UiPrefs } from '../types'
import { stepActual } from './pair'
import { minutesText, textToMs } from './parse'
import { markdownTable, ratioOf, totals } from './table'

/** 그릴 때 쓰는 요소 표 (어느 면이든 이 넷은 있다) */
export type Els = Pick<Elements['terminal'], 'Box' | 'Text' | 'Button' | 'Markdown'>

/** 그릴 때 읽은 값 */
export type ViewData = {
  plan: Plan | null
  marks: Record<string, MarkInfo>
  exe: ExeInfo
  paused: boolean
  ui: UiPrefs
  now: number
  redRatio: number
}

/** belowRows : 아래에 포갤 다른 플러그인 띠가 차지할 줄 수 (없으면 0) */
export type BandSize = { bodyColumns: number; maxRows: number; isWorking: boolean; belowRows?: number }

const MAX_BAND_ROWS = 4
/** 띠 이름 열의 최대 폭 (칸). 넘는 이름은 끝을 「…」 로 자른다 */
const NAME_MAX = 20
/** 띠 소단계 줄에서 이름 열 말고 나머지가 대략 차지하는 칸 (좁은 폭에서 이름 열을 줄일 때 쓴다) */
const LINE_REST = 26

/**
 * 그린 트리가 차지하는 줄 수. 아래에 포갤 다른 플러그인 띠의 몫을 셀 때 쓴다.
 * 세로 Box 는 자식 합, 가로 Box(기본)는 자식 최댓값, Text 는 줄바꿈 수 + 1, 엔진 자리는 0.
 * Box 는 여기에 테두리(borderStyle 이 있으면 위아래 +2)와 위아래 padding(paddingTop/Bottom > paddingY > padding)을 더한다.
 * 줄 넘김(wrap)은 폭을 몰라 세지 않는다 — 긴 글도 1줄로 센다. 띠 줄은 모두 한 줄로 자르는(truncate) 것을 전제한다.
 */
export function countRows(node: unknown): number {
  if (node === null || node === undefined || typeof node === 'boolean') return 0
  if (typeof node === 'string' || typeof node === 'number') return String(node).split('\n').length
  if (Array.isArray(node)) return node.reduce((sum: number, k: unknown) => sum + countRows(k), 0)
  if (typeof node !== 'object') return 0
  const el = node as { type?: unknown; props?: Record<string, unknown>; children?: unknown }
  const kids: unknown[] = Array.isArray(el.children) ? el.children : []
  if (el.type === 'Text') return newlines(kids) + 1
  if (el.type === 'Box') {
    const counts = kids.map(countRows)
    const isColumn = el.props?.flexDirection === 'column' || el.props?.flexDirection === 'column-reverse'
    const inner = isColumn ? counts.reduce((a, b) => a + b, 0) : counts.length === 0 ? 0 : Math.max(...counts)
    return inner + frameRows(el.props ?? {})
  }
  // 엔진 자리('engine')·버튼 같은 나머지는 띠 줄로 세지 않는다
  return 0
}

/** Box 의 테두리·위아래 padding 이 더하는 줄 수 */
function frameRows(p: Record<string, unknown>): number {
  const num = (v: unknown): number | undefined => (typeof v === 'number' ? Math.max(0, v) : undefined)
  const all = num(p.padding) ?? 0
  const y = num(p.paddingY) ?? all
  const top = num(p.paddingTop) ?? y
  const bottom = num(p.paddingBottom) ?? y
  const border = typeof p.borderStyle === 'string' && p.borderStyle !== '' ? 2 : 0
  return top + bottom + border
}

/** Text 안 글(안쪽 Text 포함)의 줄바꿈 수 */
function newlines(kids: readonly unknown[]): number {
  let n = 0
  for (const k of kids) {
    if (typeof k === 'string') n += k.split('\n').length - 1
    else if (k !== null && typeof k === 'object' && Array.isArray((k as { children?: unknown }).children)) {
      n += newlines((k as { children: unknown[] }).children)
    }
  }
  return n
}

/** 터미널 칸 수 (한글·CJK·전각은 2칸). 띠의 열을 맞출 때 쓴다. */
export function cellWidth(s: string): number {
  let w = 0
  for (const ch of s) {
    const c = ch.codePointAt(0) ?? 0
    const wide = (c >= 0x1100 && c <= 0x115f) || (c >= 0x2e80 && c <= 0xa4cf) || (c >= 0xac00 && c <= 0xd7a3) ||
      (c >= 0xf900 && c <= 0xfaff) || (c >= 0xfe30 && c <= 0xfe4f) || (c >= 0xff00 && c <= 0xff60) || (c >= 0xffe0 && c <= 0xffe6)
    w += wide ? 2 : 1
  }
  return w
}

/** w 칸 안에 들게 자른다. 넘치면 끝 한 칸을 「…」 로 바꾼다. */
export function fitCells(s: string, w: number): string {
  if (cellWidth(s) <= w) return s
  let out = ''
  let used = 0
  for (const ch of s) {
    const cw = cellWidth(ch)
    if (used + cw > w - 1) break
    out += ch
    used += cw
  }
  return out + '…'
}

/** 오른쪽에 빈칸을 채워 w 칸으로 (한글 2칸 셈) */
export function padCellsEnd(s: string, w: number): string {
  return s + ' '.repeat(Math.max(0, w - cellWidth(s)))
}

/** 왼쪽에 빈칸을 채워 w 칸으로 (한글 2칸 셈) */
export function padCellsStart(s: string, w: number): string {
  return ' '.repeat(Math.max(0, w - cellWidth(s))) + s
}

const MARK_OF: Record<StepStatus, string> = { '끝': '✓', '하는 중': '▶', '할 일': '·', '건너뜀': '–' }
/** 띠 접은 줄에 세는 차례 */
const FOLD_ORDER: readonly StepStatus[] = ['끝', '하는 중', '할 일', '건너뜀']

/** 패널 한 칸의 색. 끝은 흐리게, 하는 중은 강조, 어긋남은 노랑, 문턱 넘으면 빨강. */
function tone(step: Step, marks: Record<string, MarkInfo>, redRatio: number): { color?: Color; dimColor?: boolean; bold?: boolean } {
  const a = stepActual(step, marks)
  const ratio = ratioOf(step.estMs, a.ms)
  if (ratio !== null && ratio > redRatio) return { color: 'error', bold: step.status === '하는 중' }
  const drift = step.markIds.some(id => marks[id]?.flags.includes('어긋남')) || a.isUnsure
  if (drift) return { color: 'warning' }
  if (step.status === '하는 중') return { color: 'claude', bold: true }
  if (step.status === '끝' || step.status === '건너뜀') return { dimColor: true }
  return {}
}

function ago(now: number, at: number): string {
  return `${agoShort(now, at)} 갱신`
}

/** 마지막 갱신이 얼마 전인가 : 「10초 전」 「3분 전」 */
function agoShort(now: number, at: number): string {
  const sec = Math.max(0, Math.round((now - at) / 1000))
  return sec < 60 ? `${sec}초 전` : `${Math.round(sec / 60)}분 전`
}

/** 「3분 (2분 45초)」 → 「3분」 : 띠에는 괄호 속 초를 뺀 짧은 꼴만 쓴다 */
function shortRecord(text: string): string {
  return text.replace(/ \(.*\)$/, '')
}

/** 띠 소단계 한 줄에 쓸 값 */
type StepCells = { step: Step; actual: string; isRed: boolean; tail: string; startedAt: number }

function stepCells(step: Step, marks: Record<string, MarkInfo>, now: number, redRatio: number): StepCells {
  const a = stepActual(step, marks)
  const ratio = ratioOf(step.estMs, a.ms)
  const open = step.markIds.map(id => marks[id]).filter((m): m is MarkInfo => m !== undefined && m.open)
  let tail = ''
  if (step.status === '하는 중' && open.length > 0) {
    tail = ` · ${agoShort(now, Math.max(...open.map(m => m.lastAt)))}`
    const waitMs = open.reduce((sum, m) => sum + (textToMs(m.waitText) ?? 0), 0)
    if (waitMs >= 60_000) tail += ` · 대기 ${minutesText(waitMs)}`
  }
  return {
    step,
    actual: shortRecord(a.text) + (a.isUnsure ? '?' : ''),
    isRed: ratio !== null && ratio > redRatio,
    tail,
    startedAt: startedAtOf(step, marks),
  }
}

/** 소단계가 언제 시작했나 : 열린 mark 가운데 가장 늦게 본 시각. 열린 mark 가 없으면 -1 */
function startedAtOf(step: Step, marks: Record<string, MarkInfo>): number {
  const open = step.markIds.map(id => marks[id]).filter((m): m is MarkInfo => m !== undefined && m.open)
  return open.length === 0 ? -1 : Math.max(...open.map(m => m.seenAt))
}

/** 하는 중이 여럿일 때 가장 최근에 시작한 것 하나 (같으면 뒤의 것). 띠와 패널 머리글이 같이 쓴다. */
function latestStarted<T extends { startedAt: number }>(list: readonly T[]): T | undefined {
  if (list.length === 0) return undefined
  return list.reduce((a, c) => (c.startedAt >= a.startedAt ? c : a))
}

/**
 * 띠에 낼 소단계 줄 고르기. budget 줄 안에 다 들면 전부.
 * 넘치면 하는 중 줄만 두고 나머지는 접은 줄 하나로. 그래도 넘치면 하는 중은 가장 최근 것 하나만.
 * fold 가 null 이면 접은 줄을 그리지 않는다.
 */
export function pickStepLines<T extends { step: Step; startedAt: number }>(cells: readonly T[], budget: number): { shown: T[]; fold: Step[] | null } {
  if (cells.length <= budget) return { shown: [...cells], fold: null }
  if (budget <= 0) return { shown: [], fold: null }
  let keep = cells.filter(c => c.step.status === '하는 중')
  const latest = latestStarted(keep)
  if (keep.length + 1 > budget && latest !== undefined) keep = [latest]
  const rest = cells.filter(c => !keep.includes(c)).map(c => c.step)
  if (keep.length + 1 > budget) return { shown: keep, fold: null } // budget 1 : 하는 중 한 줄만
  return { shown: keep, fold: rest }
}

/** 접은 줄 글 : 「✓ 2 끝 · · 5 할 일 · – 1 건너뜀」 */
export function foldText(steps: readonly Step[]): string {
  return FOLD_ORDER
    .map(st => ({ st, n: steps.filter(s => s.status === st).length }))
    .filter(x => x.n > 0)
    .map(x => `${MARK_OF[x.st]} ${x.n} ${x.st}`)
    .join(' · ')
}

/** 띠 소단계 한 줄 : 「 ▶ 13 코드리뷰      3분 / 8분 · 10초 전」 */
function stepLine(els: Els, c: StepCells, w: { num: number; name: number; act: number }): RenderElement {
  const { Text } = els
  const s = c.step
  const isActive = s.status === '하는 중'
  const isDim = s.status === '할 일' || s.status === '건너뜀'
  const markColor: { color?: Color } = s.status === '끝' ? { color: 'success' } : isActive ? { color: 'warning' } : {}
  const name = padCellsEnd(fitCells(s.name, w.name), w.name)
  return (
    <Text key={`s${s.n}`} wrap="truncate-end" {...(isActive ? { bold: true } : {})} {...(isDim ? { dimColor: true } : {})}>
      <Text {...markColor}>{` ${MARK_OF[s.status]}`}</Text>
      {` ${padCellsStart(String(s.n), w.num)} ${name}  `}
      <Text {...(c.isRed ? { color: 'error' as Color } : {})}>{padCellsStart(c.actual, w.act)}</Text>
      {` / ${s.estText}${c.tail}`}
    </Text>
  )
}

/** 열린 mark 한 줄 (계획 없음 · 짝 없는 mark) : 「 ▶ 이름  3분 · 10초 전」 */
function markLine(els: Els, m: MarkInfo, nameW: number, now: number): RenderElement {
  const { Text } = els
  const rec = m.recordText === null ? '재는 중' : shortRecord(m.recordText)
  return (
    <Text key={`live-${m.id}`} wrap="truncate-end" bold>
      <Text color="warning"> ▶</Text>
      {` ${padCellsEnd(fitCells(m.name, nameW), nameW)}  ${rec} · ${agoShort(now, m.lastAt)}`}
    </Text>
  )
}

/** 띠. 그릴 것이 없으면 null (엔진 몫으로 넘긴다). */
export function bandTree(els: Els, d: ViewData, size: BandSize): RenderElement | null {
  const { plan, marks, exe, paused, ui, now, redRatio } = d
  if (ui.bandHidden) return null
  const open = Object.values(marks).filter(m => m.open)
  if (plan === null && open.length === 0) return null

  const { Box, Text } = els
  const cols = size.bodyColumns
  // 아래 띠를 포갤 때는 그 몫(줄)을 먼저 뺀다. 줄은 모두 한 줄 높이라 줄 수만 센다.
  const room = Math.max(1, Math.min(MAX_BAND_ROWS, size.maxRows) - (size.belowRows ?? 0))
  const warns: RenderElement[] = []
  if (exe.path === null && exe.why !== null) warns.push(<Text key="exe" color="warning" wrap="truncate-end">{exe.why}</Text>)
  if (paused) warns.push(<Text key="paused" color="warning" wrap="truncate-end">갱신 멈춤 — /steps refresh 로 다시 켠다</Text>)
  // 머리 줄과 경고 줄을 뺀 나머지가 소단계 줄 몫이다
  const budget = Math.max(0, room - 1 - warns.length)
  const nameCap = Math.max(4, Math.min(NAME_MAX, cols - LINE_REST))
  const rows: RenderElement[] = []

  // 짝 없는 열린 mark 는 가장 최근 것부터 남는 줄에 넣는다
  const markRows = (list: MarkInfo[], left: number) => {
    const picked = [...list].sort((a, b) => a.seenAt - b.seenAt).slice(-Math.max(0, left))
    if (picked.length === 0) return []
    const nameW = Math.min(nameCap, Math.max(...picked.map(m => cellWidth(m.name))))
    return picked.map(m => markLine(els, m, nameW, now))
  }

  if (plan === null) {
    rows.push(<Text key="head" bold wrap="truncate-end">공수 · (계획 없음)</Text>)
    rows.push(...markRows(open, budget))
  } else {
    const t = totals(plan, marks)
    rows.push(
      <Box key="head" flexDirection="row" justifyContent="space-between" width={cols}>
        <Box flexShrink={1}>
          <Text bold wrap="truncate-end">공수 · {plan.title}</Text>
        </Box>
        <Box flexShrink={0}>
          <Text dimColor wrap="truncate">
            {' '}{t.done}/{plan.steps.length} 끝 · 예상 {minutesText(t.estMs)} · 실제 {minutesText(t.actualMs)}
          </Text>
        </Box>
      </Box>,
    )
    const cells = plan.steps.map(s => stepCells(s, marks, now, redRatio))
    const pick = pickStepLines(cells, budget)
    if (pick.shown.length > 0) {
      const w = {
        num: Math.max(...pick.shown.map(c => String(c.step.n).length)),
        name: Math.min(nameCap, Math.max(...pick.shown.map(c => cellWidth(c.step.name)))),
        act: Math.max(...pick.shown.map(c => cellWidth(c.actual))),
      }
      for (const c of pick.shown) rows.push(stepLine(els, c, w))
    }
    if (pick.fold !== null && pick.fold.length > 0) {
      rows.push(<Text key="fold" dimColor wrap="truncate-end"> {foldText(pick.fold)}</Text>)
    }
    const used = rows.length - 1
    rows.push(...markRows(open.filter(m => m.n === null), budget - used))
  }

  rows.push(...warns)
  return <Box flexDirection="column">{rows.slice(0, room)}</Box>
}

/** 패널 표 바깥 테두리(Box borderStyle) 좌우 2칸 + 안쪽 여백(paddingX 1) 좌우 2칸 */
export const PANE_FRAME = 4
/** 칸 사이 구분자 : 넉넉하면 양옆 빈칸을 두고, 좁으면 세로줄만 */
const SEP_WIDE = ' │ '
const SEP_TIGHT = '│'
/** 가로줄에서 구분자 자리에 놓는 글자 (구분자와 같은 폭) */
const CROSS_OF: Record<string, string> = { [SEP_WIDE]: '─┼─', [SEP_TIGHT]: '┼' }

/**
 * 패널 표 모양 : 칸 너비(0 이면 그 칸을 뺀다)와 구분자. inner 는 테두리·여백 안쪽 폭이다.
 * plain 이면 표가 안 들어가는 폭이라 칸·가로줄 없이 「이름 · 상태」 맨 줄로 그린다.
 */
export type PaneLayout = { inner: number; sep: string; name: number; cls: number; size: number; est: number; act: number; status: number; plain: boolean }

/** 예상·실제 칸의 최소 폭 : 「1시간 16분」(10칸)이 잘리지 않는 폭 */
const TIME_MIN = 10
/** 실제 칸이 넉넉할 때의 폭 : 「3분 (2분 40초)」 같은 괄호 초까지 */
const ACT_WIDE = 16
/** 상태 칸 : 「하는 중」(7칸) + 1 */
const STATUS_W = 8
/** 이름 칸이 이만큼은 되어야 실제 칸을 넓힌다 (이름 칸이 먼저) */
const NAME_WANT = 24
/** 넓은 구분자 「 │ 」 를 쓸 때 이름 칸의 최소 폭 */
const MID_NAME_MIN = 16
/** 좁은 표에서도 이름 칸이 이만큼은 있어야 한다. 모자라면 맨 줄로 (plain) */
const NAME_MIN = 4
/** 이 폭(패널 bodyColumns) 미만이면 분류·크기 칸을 뺀다 */
const CLS_MIN_COLS = 100

/**
 * 패널 폭에 맞춰 표 모양을 고른다. 이름 칸이 먼저다.
 * ① 100칸 이상이고 이름 칸이 넉넉하면 여섯 칸 + 「 │ 」 ② 분류·크기를 빼고 네 칸 + 「 │ 」
 * ③ 네 칸 + 「│」. 예상·실제 칸은 TIME_MIN 을 지키고, 실제 칸은 이름 칸이 NAME_WANT 를 넘는 만큼만 넓힌다.
 * ④ 그것도 안 들어가면(bodyColumns 39 미만) 표를 버리고 「이름 · 상태」 맨 줄 (plain).
 * 표일 때 칸 너비 + 구분자 폭의 합은 언제나 inner 와 같다 (가로줄이 표 폭과 꼭 맞게).
 */
export function paneLayout(bodyColumns: number): PaneLayout {
  const inner = Math.max(1, bodyColumns - PANE_FRAME)
  const est = TIME_MIN
  const status = STATUS_W
  const sepW = cellWidth(SEP_WIDE)
  // 이름 칸이 NAME_WANT 를 넘는 만큼만 실제 칸을 ACT_WIDE 까지 넓힌다
  const actFor = (fixed: number) => Math.max(TIME_MIN, Math.min(ACT_WIDE, inner - fixed - NAME_WANT))
  if (bodyColumns >= CLS_MIN_COLS) {
    const full = est + ACT_WIDE + status + 4 + 4 + 5 * sepW
    if (inner - full >= NAME_WANT) return { inner, sep: SEP_WIDE, name: inner - full, cls: 4, size: 4, est, act: ACT_WIDE, status, plain: false }
  }
  const midFixed = est + status + 3 * sepW
  const midAct = actFor(midFixed)
  // 「 │ 」 셋(6칸)을 「│」 로 줄이면 이름 칸이 6칸 는다. 이름이 MID_NAME_MIN 도 안 되면 좁은 구분자로 간다
  if (inner - midFixed - midAct >= MID_NAME_MIN) return { inner, sep: SEP_WIDE, name: inner - midFixed - midAct, cls: 0, size: 0, est, act: midAct, status, plain: false }
  const fixed = est + status + 3 * cellWidth(SEP_TIGHT)
  const act = actFor(fixed)
  const name = inner - fixed - act
  if (name >= NAME_MIN) return { inner, sep: SEP_TIGHT, name, cls: 0, size: 0, est, act, status, plain: false }
  // 칸을 다 줄여도 넘친다 : 맨 줄 (칸 너비는 쓰지 않는다)
  return { inner, sep: SEP_TIGHT, name: inner, cls: 0, size: 0, est: 0, act: 0, status: 0, plain: true }
}

/** 가로줄 한 줄 : 칸마다 「─」, 구분자 자리에 「┼」. 폭은 inner 와 같다. */
export function ruleText(l: PaneLayout): string {
  const cross = CROSS_OF[l.sep] ?? '┼'
  return [l.name, l.cls, l.size, l.est, l.act, l.status].filter(w => w > 0).map(w => '─'.repeat(w)).join(cross)
}

/**
 * 패널 머리글 두 줄에 쓸 값. 줄 1 = head + body, 줄 2 = 막대 (찬 칸 filled · 전체 barWidth).
 * 지금 단계는 하는 중 가운데 가장 최근 시작한 것, 없으면 첫 할 일. 둘 다 없으면 전부 끝(끝·건너뜀)이다.
 * 번호는 소단계 번호(n)가 아니라 계획 안 차례(1부터)다 — 「3/7」 의 앞뒤가 같은 눈금이 되게.
 * 줄 1 은 width 칸 안에 든다 : 이름 쪽을 먼저 자르고 「 · 예상 …」 은 남긴다.
 */
export type PaneHeader = { head: string; body: string; current: Step | null; filled: number; barWidth: number }

export function paneHeader(plan: Plan, marks: Record<string, MarkInfo>, width: number): PaneHeader {
  const total = plan.steps.length
  const barWidth = Math.max(1, width)
  const done = totals(plan, marks).done
  const filled = total === 0 ? 0 : Math.min(barWidth, Math.round((done / total) * barWidth))
  const active = plan.steps.filter(s => s.status === '하는 중').map(s => ({ step: s, startedAt: startedAtOf(s, marks) }))
  const current = latestStarted(active)?.step ?? plan.steps.find(s => s.status === '할 일') ?? null
  if (current === null) {
    const head = fitCells(`✓ ${total}/${total}`, barWidth)
    return { head, body: fitOrEmpty('  모두 끝', barWidth - cellWidth(head)), current, filled, barWidth }
  }
  // 할 일을 고른 때도 「▶」 : 다음에 할 단계라는 뜻 (색은 paneTree 가 tone 으로 달리한다)
  const head = fitCells(`▶ ${plan.steps.indexOf(current) + 1}/${total}`, barWidth)
  const room = barWidth - cellWidth(head)
  // 예상이 비었으면(— 는 estimate 전 자리) 「 · 예상 …」 를 뺀다
  const hasEst = current.estText !== '' && current.estText !== '—'
  const est = hasEst ? ` · 예상 ${current.estText}` : ''
  const nameRoom = room - 2 - cellWidth(est)
  // 이름 칸이 너무 좁으면(2칸 미만) 예상까지 한 덩어리로 자른다
  const body = nameRoom >= 2 ? `  ${fitCells(current.name, nameRoom)}${est}` : fitOrEmpty(`  ${current.name}${est}`, room)
  return { head, body, current, filled, barWidth }
}

/** fitCells 와 같되, 자리가 1칸도 없으면 빈 글 (fitCells 는 0칸에도 「…」 를 낸다) */
function fitOrEmpty(s: string, w: number): string {
  return w <= 0 ? '' : fitCells(s, w)
}

/** 패널 버튼이 부르는 처리기 (main 이 $ 를 품은 채로 넘긴다) */
export type PaneActions = { toggleExpanded: () => unknown; refreshNow: () => unknown }

type CellStyle = { color?: Color; dimColor?: boolean; bold?: boolean }

/** 패널 */
export function paneTree(els: Els, d: ViewData, bodyColumns: number, act: PaneActions): RenderElement {
  const { Box, Text, Button, Markdown } = els
  const { plan, marks, ui, exe, now, redRatio } = d
  const l = paneLayout(bodyColumns)
  const sepW = cellWidth(l.sep)

  // 한 줄 = 칸 Box 들 사이에 옅은 세로 구분자. 너비 0 인 칸은 뺀다.
  const cellRow = (key: string, cells: { text: string; width: number; style?: CellStyle }[]) => {
    const kids: RenderElement[] = []
    cells.filter(c => c.width > 0).forEach((c, i) => {
      if (i > 0) {
        kids.push(
          <Box key={`${key}-sep${i}`} width={sepW} flexShrink={0}>
            <Text dimColor>{l.sep}</Text>
          </Box>,
        )
      }
      kids.push(
        <Box key={`${key}-${i}`} width={c.width}>
          <Text wrap="truncate" {...(c.style ?? {})}>{c.text}</Text>
        </Box>,
      )
    })
    return <Box key={key} flexDirection="row">{kids}</Box>
  }
  // 머리글 아래·합계 위는 진한 가로줄, 소단계 사이는 옅은 가로줄
  const rule = (key: string, isDim: boolean) => (
    <Text key={key} wrap="truncate" {...(isDim ? { dimColor: true } : {})}>{ruleText(l)}</Text>
  )

  if (plan === null) {
    return (
      <Box flexDirection="column">
        <Text dimColor>계획이 없다. Claude 가 plan 도구로 소단계를 넣으면 여기에 표가 뜬다.</Text>
        {exe.path === null && exe.why !== null ? <Text color="warning">{exe.why}</Text> : null}
        {orphanLines(marks).map((line, i) => <Text key={`o${i}`} dimColor wrap="truncate">{line}</Text>)}
      </Box>
    )
  }

  const rows: RenderElement[] = []
  if (l.plain) {
    // 좁은 폭 : 칸 없이 「이름 · 상태」 한 줄씩. 상태는 남기고 이름 쪽을 자른다
    plan.steps.forEach(s => {
      const tail = ` · ${s.status}`
      const line = fitCells(`${fitCells(`${s.n}. ${s.name}`, Math.max(1, l.inner - cellWidth(tail)))}${tail}`, l.inner)
      rows.push(<Text key={`s${s.n}`} wrap="truncate" {...tone(s, marks, redRatio)}>{line}</Text>)
    })
    const t = totals(plan, marks)
    rows.push(<Text key="sum" bold wrap="truncate">{fitCells(`합계 · ${t.done}/${plan.steps.length} 끝`, l.inner)}</Text>)
  } else {
    rows.push(
      cellRow('head', [
        { text: '소단계', width: l.name, style: { bold: true } },
        { text: '분류', width: l.cls, style: { bold: true } },
        { text: '크기', width: l.size, style: { bold: true } },
        { text: '예상', width: l.est, style: { bold: true } },
        { text: '실제', width: l.act, style: { bold: true } },
        { text: '상태', width: l.status, style: { bold: true } },
      ]),
    )
    rows.push(rule('rule-head', false))
    plan.steps.forEach((s, i) => {
      if (i > 0) rows.push(rule(`rule-${s.n}`, true))
      const a = stepActual(s, marks)
      const style = tone(s, marks, redRatio)
      // 표 줄은 모두 한 줄이다. 이름이 넘치면 끝을 자르되 앞 번호는 남는다
      rows.push(
        cellRow(`s${s.n}`, [
          { text: fitCells(`${s.n}. ${s.name}`, l.name), width: l.name, style },
          { text: s.cls, width: l.cls, style },
          { text: s.size, width: l.size, style },
          { text: s.estText, width: l.est, style },
          { text: a.text + (a.isUnsure ? ' ?' : ''), width: l.act, style },
          { text: s.status, width: l.status, style },
        ]),
      )
    })
    rows.push(rule('rule-sum', false))
    const t = totals(plan, marks)
    // 합계 첫 칸은 소단계·분류·크기 칸과 그 사이 구분자를 합친 너비다
    const front = [l.name, l.cls, l.size].filter(w => w > 0)
    rows.push(
      cellRow('sum', [
        { text: '합계', width: front.reduce((a, b) => a + b, 0) + (front.length - 1) * sepW, style: { bold: true } },
        { text: minutesText(t.estMs), width: l.est, style: { bold: true } },
        { text: minutesText(t.actualMs) + (t.hasOpen ? ' (+하는 중)' : ''), width: l.act, style: { bold: true } },
        { text: `${t.done}/${plan.steps.length} 끝`, width: l.status, style: { bold: true } },
      ]),
    )
  }

  // 머리글 두 줄 : 지금 단계 한 줄 + 진행 막대 (찬 칸 초록 █ · 빈 칸 흐린 ░)
  const hd = paneHeader(plan, marks, bodyColumns)
  const headTone: CellStyle = hd.current === null ? { color: 'success' } : tone(hd.current, marks, redRatio)
  return (
    <Box flexDirection="column">
      <Text bold wrap="truncate">공수 · {plan.title}</Text>
      <Text key="now" bold wrap="truncate">
        <Text {...headTone}>{hd.head}</Text>
        {hd.body}
      </Text>
      <Text key="bar" wrap="truncate">
        <Text color="success">{'█'.repeat(hd.filled)}</Text>
        <Text dimColor>{'░'.repeat(hd.barWidth - hd.filled)}</Text>
      </Text>
      <Box key="grid" flexDirection="column" borderStyle="single" borderDimColor paddingX={1} width={bodyColumns}>
        {rows}
      </Box>
      {detailLines(els, plan, marks, now)}
      {orphanLines(marks).map((line, i) => <Text key={`o${i}`} dimColor wrap="truncate">{line}</Text>)}
      {plan.estNote !== null ? <Text color="warning" wrap="truncate">예상 : {plan.estNote}</Text> : null}
      {exe.path === null && exe.why !== null ? <Text color="warning" wrap="truncate">{exe.why}</Text> : null}
      <Box flexDirection="row" columnGap={2}>
        <Button
          key="copy"
          label={ui.expanded ? '표 접기' : '표 복사용 글'}
          onPress={act.toggleExpanded}
        />
        <Button key="refresh" label="지금 갱신" onPress={act.refreshNow} />
      </Box>
      {ui.expanded ? <Markdown key="md" text={'```\n' + markdownTable(plan, marks, redRatio) + '\n```'} /> : null}
    </Box>
  )
}

/**
 * 표 아래 상세 줄. 표 줄을 한 줄로 두려고 mark 상세·주의·까닭을 여기로 뺐다.
 * 열린 mark : 「하는 중 : m1007-xxxx · 대기 — · 14초 전 갱신」 (열린 mark 가 여럿이면 앞에 「2. 」 처럼 소단계 번호)
 * show 실패 : 「2. 확인 못 함 : m1007-xxxx · <까닭>」 (노랑) · mark 주의 : 「2. 주의 : …」 · 까닭 : 「2. 까닭 : …」
 * 열린 mark 가 없으면 하는 중 줄은 그리지 않는다.
 */
function detailLines(els: Els, plan: Plan, marks: Record<string, MarkInfo>, now: number): RenderElement[] {
  const { Text } = els
  const open = plan.steps.flatMap(s => s.markIds.map(id => ({ n: s.n, m: marks[id] })))
    .filter((x): x is { n: number; m: MarkInfo } => x.m !== undefined && x.m.open)
  const isMany = open.length > 1
  const out: RenderElement[] = []
  for (const { n, m } of open) {
    const bits = [`${isMany ? `${n}. ` : ''}하는 중 : ${m.id}`, `대기 ${m.waitText ?? '—'}`]
    if (m.agentId !== null) bits.push(`갈래 ${m.agentId.slice(0, 14)}…`)
    bits.push(ago(now, m.lastAt))
    out.push(<Text key={`m-${m.id}`} dimColor wrap="truncate">{bits.join(' · ')}</Text>)
  }
  for (const s of plan.steps) {
    for (const id of s.markIds) {
      const m = marks[id]
      if (m === undefined) continue
      if (m.error !== null) out.push(<Text key={`e-${id}`} color="warning" wrap="truncate">{s.n}. 확인 못 함 : {m.id} · {m.error}</Text>)
      if (m.warn !== null) out.push(<Text key={`w-${id}`} color="warning" wrap="truncate">{s.n}. 주의 : {m.warn}</Text>)
    }
    if (s.note !== null) out.push(<Text key={`n-${s.n}`} dimColor wrap="truncate">{s.n}. 까닭 : {s.note}</Text>)
  }
  return out
}

/** 짝 없는 mark 줄 (번호가 없거나 계획에 없는 번호, 계획 전에 연 mark). 최근 3개만. */
export function orphanLines(marks: Record<string, MarkInfo>): string[] {
  return Object.values(marks)
    .filter(m => m.n === null)
    .slice(-3)
    .map(m => `짝 없는 mark: ${m.id} "${m.name}" ${m.recordText ?? '—'}${m.open ? ' (열림)' : ''}`)
}
