// 띠·패널 그림 시험 : 두 면(terminal · desktop) × 폭 50 · 80 · 140
import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

import { PANE_FRAME, cellWidth, countRows, paneHeader, paneLayout, ruleText } from '../hooks/view'
import { SID, STEPS, showText, world } from './world'
import type { World } from './world'

const T0 = Date.parse('2026-10-07T03:00:00.000Z')
const START = { cwd: 'C:/repo', surface: 'terminal', isInteractive: true } as const
const SURFACES = ['terminal', 'desktop'] as const
const WIDTHS = [50, 80, 140] as const

function bandProps(cols: number) {
  return { hasSurvey: false, isWorking: false, maxRows: 10, bodyColumns: cols, scroll: { offset: 0, bodyRows: 10 }, view: {} }
}

function paneProps(cols: number) {
  return { title: '공수', isFocused: false, bodyColumns: cols, placement: 'dock' as const, scroll: { offset: 0, bodyRows: 30 }, view: {} }
}

/** 계획 하나와 열린 mark 하나를 만든다 */
async function setup($: Engine, w: World, clock: { advance: (ms: number) => Promise<void> }) {
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '타일 검사기', steps: STEPS })
  w.bash.text = 'mark m1007-c001 시작 : 1-조사 (2026-10-07 12:00:00)'
  w.shows.set('m1007-c001', showText('m1007-c001', '1-조사', '3분 (2분 40초)', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  await clock.advance(100)
}

test('계획이 없으면 띠는 엔진 몫으로 비킨다', async ($, on) => {
  mock.clock(on, { now: T0 })
  world(on)
  await $.session.start(START)
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface, component: 'AbovePrompt', props: bandProps(80) })
    expect(await ui.find({ text: /공수/ })).toBeUndefined()
    await ui.unmount()
  }
})

test('띠 : 두 면 · 세 폭', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  for (const surface of SURFACES) {
    for (const cols of WIDTHS) {
      const ui = await $.ui.mount({ plugin: 'effort-steps', surface, component: 'AbovePrompt', props: bandProps(cols) })
      expect(await ui.find({ text: /공수 · 타일 검사기/ })).toBeDefined()
      expect(await ui.find({ text: /0\/3 끝 · 예상 10분/ })).toBeDefined()
      // 소단계마다 한 줄 : 상태 · 번호 · 이름 · 실제 / 예상 (하는 중 줄만 「N초 전」)
      expect(await ui.find({ text: /^ ▶ 1 조사  3분 \/ 4분 · 0초 전$/ })).toBeDefined()
      expect(await ui.find({ text: /^ · 2 구현    — \/ 2분$/ })).toBeDefined()
      expect(await ui.find({ text: /^ · 3 시험    — \/ 4분$/ })).toBeDefined()
      // mark id · 기록 · 대기 상세 줄은 띠에서 뺐다 (패널에만 있다)
      expect(await ui.find({ text: /m1007-c001/ })).toBeUndefined()
      await ui.unmount()
    }
  }
})

test('띠 : /steps hide 면 비킨다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  const presentation = { isFullscreen: false, columns: 120 } as never
  const origin = { kind: 'composer' } as never
  await $.command.run({ command: 'steps', args: 'hide', origin, presentation })
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /공수/ })).toBeUndefined()
  await ui.unmount()
})

test('패널 : 두 면 · 세 폭, 표 복사용 글 버튼', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  for (const surface of SURFACES) {
    for (const cols of WIDTHS) {
      const ui = await $.ui.mount({ plugin: 'effort-steps', surface, component: 'Pane', requestId: 'effort-steps', props: paneProps(cols) })
      expect(await ui.find({ text: /공수 · 타일 검사기/ })).toBeDefined()
      expect(await ui.find({ text: /^1\. 조사$/ })).toBeDefined()
      expect(await ui.find({ text: /하는 중/ })).toBeDefined()
      expect(await ui.find({ text: /합계/ })).toBeDefined()
      expect(await ui.find({ key: 'md' })).toBeUndefined()
      await ui.press({ key: 'copy' })
      const md = await ui.find({ key: 'md' })
      expect(md?.text ?? String(md?.props.text)).toContain('| 1. 조사 | 4분 | 3분 (2분 40초) | 하는 중 |')
      await ui.press({ key: 'copy' })
      expect(await ui.find({ key: 'md' })).toBeUndefined()
      await ui.unmount()
    }
  }
})

test('패널 : 지금 갱신 버튼은 mark show 만 부른다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(80) })
  const before = w.argvs.length
  w.shows.set('m1007-c001', showText('m1007-c001', '1-조사', '6분', true))
  await ui.press({ key: 'refresh' })
  expect(w.argvs.length).toBe(before + 1)
  expect(w.argvs[w.argvs.length - 1]!.slice(1)).toEqual(['mark', 'show', 'm1007-c001'])
  expect(await ui.find({ text: /^6분$/ })).toBeDefined()
  await ui.unmount()
})

test('띠 : 아래 플러그인의 띠가 있으면 내 것 아래에 포갠다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on, undefined, { belowText: '게이지 · 5h 42%' })
  await setup($, w, clock)
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface, component: 'AbovePrompt', props: bandProps(80) })
    expect(await ui.find({ text: /공수 · 타일 검사기/ })).toBeDefined()
    expect(await ui.find({ text: /게이지 · 5h 42%/ })).toBeDefined()
    const texts = (await ui.findAll({})).map(n => n.text ?? '').filter(t => /공수 · 타일|게이지/.test(t))
    expect(texts[0]).toMatch(/공수 · 타일 검사기/)
    expect(texts[texts.length - 1]).toMatch(/게이지/)
    await ui.unmount()
  }
})

// ── 띠 새 모양 (2026-10-07 줄 정리) ──

type Node = { type?: string; props?: Record<string, unknown>; children?: (Node | string)[] } | string

function textOf(n: Node): string {
  if (typeof n === 'string') return n
  return (n.children ?? []).map(textOf).join('')
}

/** 그린 트리를 터미널 글 줄로 펴 본다 (Box column = 줄 쌓기, row + space-between = 양끝 맞춤). 보고용 장면 뜨기에만 쓴다. */
function screen(n: Node, cols: number): string[] {
  if (typeof n === 'string') return [n]
  if (n.type === 'Text') return [textOf(n)]
  const kids = n.children ?? []
  if (n.props?.flexDirection === 'row') {
    const parts = kids.map(k => screen(k, cols)[0] ?? '')
    if (n.props.justifyContent === 'space-between' && parts.length === 2) {
      const w = (s: string) => [...s].reduce((a, ch) => a + (/[\u1100-\u115f\u2e80-\ua4cf\uac00-\ud7a3]/.test(ch) ? 2 : 1), 0)
      return [parts[0]! + ' '.repeat(Math.max(1, cols - w(parts[0]!) - w(parts[1]!))) + parts[1]!]
    }
    return [parts.join('')]
  }
  return kids.flatMap(k => screen(k, cols))
}

/** 사용자 피드백의 장면 : 끝 하나 · 하는 중 하나 · 할 일 하나, 번호 두 자리 */
function scenePlan(wait: string, lastAgoMs: number, est12Ms = 40_000) {
  const plan = {
    id: 'p2', title: 'ModTool 1차 뒷정리', createdAt: T0 - 600_000, sessionId: SID, estNote: null,
    steps: [
      { n: 12, name: '문서뒷정리', cls: '조사', size: 'S', estText: '1분 미만', estMs: est12Ms, markIds: ['m1007-e012'], status: '끝', note: null },
      { n: 13, name: '코드리뷰', cls: '구현', size: 'M', estText: '8분', estMs: 480_000, markIds: ['m1007-e013'], status: '하는 중', note: null },
      { n: 14, name: '기억남기기', cls: '구현', size: 'S', estText: '1분 미만', estMs: 40_000, markIds: [], status: '할 일', note: null },
    ],
  }
  const mark = (id: string, name: string, n: number, open: boolean, rec: string, ms: number) => ({
    id, name, n, agentId: null, open, recordText: rec, recordMs: ms, waitText: wait, flags: [], warn: null, error: null,
    fails: 0, same: 0, seenAt: T0 - 300_000, lastAt: T0 - lastAgoMs,
  })
  const marks = {
    'm1007-e012': mark('m1007-e012', '12-문서뒷정리', 12, false, '1분', 60_000),
    'm1007-e013': mark('m1007-e013', '13-코드리뷰', 13, true, '3분 (2분 45초)', 165_000),
  }
  return { [`plan:${SID}`]: { plan, marks } }
}

test('띠 새 모양 : 머리 줄 양끝 · 열 맞춤 · 하는 중 줄만 「N초 전」 (80칸 장면)', async ($, on) => {
  mock.clock(on, { now: T0 })
  const w = world(on, scenePlan('없음', 10_000))
  w.shows.set('m1007-e013', showText('m1007-e013', '13-코드리뷰', '3분 (2분 45초)', true))
  await $.session.start(START)
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface, component: 'AbovePrompt', props: bandProps(80) })
    const lines = screen((await ui.drawn()) as unknown as Node, 80)
    // 보고용 장면을 시험 출력에 남긴다 (tsconfig lib 에 console 이 없어 globalThis 로 꺼낸다)
    if (surface === 'terminal') (globalThis as unknown as { console: { log: (s: string) => void } }).console.log(['--- 80칸 장면 ---', ...lines, '---'].join('\n'))
    expect(lines).toEqual([
      '공수 · ModTool 1차 뒷정리                           1/3 끝 · 예상 9분 · 실제 4분',
      ' ✓ 12 문서뒷정리  1분 / 1분 미만',
      ' ▶ 13 코드리뷰    3분 / 8분 · 10초 전',
      ' · 14 기억남기기    — / 1분 미만',
    ])
    await ui.unmount()
  }
})

test('띠 새 모양 : 대기 1분 이상이면 하는 중 줄 끝에 「대기 N분」, 예상의 redRatio 배를 넘으면 실제가 빨강', async ($, on) => {
  mock.clock(on, { now: T0 })
  const w = world(on, scenePlan('2분', 75_000, 30_000))
  w.shows.set('m1007-e013', showText('m1007-e013', '13-코드리뷰', '3분 (2분 45초)', true))
  await $.session.start(START)
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /^ ▶ 13 코드리뷰    3분 \/ 8분 · 1분 전 · 대기 2분$/ })).toBeDefined()
  // 끝 줄 (1분 / 1분 미만) 은 대기·「전」 을 붙이지 않는다
  expect(await ui.find({ text: /^ ✓ 12 문서뒷정리  1분 \/ 1분 미만$/ })).toBeDefined()
  const red = await ui.findAll({ type: 'Text' })
  const reds = red.filter(n => n.props.color === 'error').map(n => n.text)
  expect(reds).toEqual(['1분']) // 12 : 1분 > 30초 × 1.5 → 빨강 · 13 : 3분 < 8분 → 그대로
  await ui.unmount()
})

test('띠 새 모양 : 계획 없이 열린 mark 만 있으면 「(계획 없음)」 과 mark 한 줄씩', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  w.bash.text = 'mark m1007-c201 시작 : 메모정리 (2026-10-07 12:00:00)'
  w.shows.set('m1007-c201', showText('m1007-c201', '메모정리', '3분 (2분 40초)', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "메모정리"' })
  await clock.advance(100)
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /^공수 · \(계획 없음\)$/ })).toBeDefined()
  expect(await ui.find({ text: /^ ▶ 메모정리  3분 · 0초 전$/ })).toBeDefined()
  await ui.unmount()
})

// ── 띠 기본 숨김 (2026-10-07 : 공수 표는 /steps 패널로 본다) ──

test('띠 기본 숨김 : prefs 가 없으면 띠를 그리지 않고 아래 띠만 돌려준다, /steps show 로 켠다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on, undefined, { belowText: '게이지 · 5h 42%' })
  w.store.delete('prefs')
  await setup($, w, clock)
  let ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /공수/ })).toBeUndefined()
  expect(await ui.find({ text: /게이지 · 5h 42%/ })).toBeDefined()
  await ui.unmount()
  const presentation = { isFullscreen: false, columns: 120 } as never
  const origin = { kind: 'composer' } as never
  expect((await $.command.run({ command: 'steps', args: 'show', origin, presentation })).text).toContain('켰다')
  expect(w.store.get('prefs')).toEqual({ bandHidden: false })
  ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /공수 · 타일 검사기/ })).toBeDefined()
  expect(await ui.find({ text: /게이지 · 5h 42%/ })).toBeDefined()
  await ui.unmount()
})

test('띠 기본 숨김 : 아래 띠도 없으면 엔진 자리를 그대로 돌려준다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  w.store.delete('prefs')
  await setup($, w, clock)
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
  expect(await ui.find({ text: /공수/ })).toBeUndefined()
  expect((await ui.drawn()) as unknown).toEqual({ type: 'engine', ref: 0 })
  await ui.unmount()
})

// ── 패널 표 줄 (2026-10-07) ──

/** 그린 트리에서 key 가 맞는 첫 노드 */
function findKey(n: unknown, key: string): Node | undefined {
  if (n === null || typeof n !== 'object') return undefined
  if (Array.isArray(n)) {
    for (const k of n) {
      const hit = findKey(k, key)
      if (hit !== undefined) return hit
    }
    return undefined
  }
  const el = n as { props?: Record<string, unknown>; key?: unknown; children?: unknown }
  if (el.key === key || el.props?.key === key) return el as Node
  return findKey(el.children, key)
}

test('패널 표 : 테두리 · 머리글 아래 가로줄 · 소단계 사이 옅은 줄 · 세로 구분자, 세 폭에서 줄 폭이 테두리·여백 안쪽과 같다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  for (const cols of WIDTHS) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(cols) })
    const rules = (await ui.findAll({ type: 'Text' })).map(n => n.text ?? '').filter(t => /^─/.test(t))
    // 머리글 아래 1 · 소단계 사이 2 · 합계 위 1
    expect(rules.length).toBe(4)
    for (const r of rules) {
      expect(cellWidth(r)).toBe(cols - PANE_FRAME)
      expect(r).toContain('┼')
    }
    const seps = (await ui.findAll({ type: 'Text' })).filter(n => /│/.test(n.text ?? '') && n.props.dimColor === true)
    expect(seps.length).toBeGreaterThan(0)
    // 100칸 미만에서는 분류·크기 칸을 빼고(이름 칸이 먼저), 넓으면 넣는다
    if (cols < 100) expect(await ui.find({ text: /^분류$/ })).toBeUndefined()
    else expect(await ui.find({ text: /^분류$/ })).toBeDefined()
    const grid = findKey(await ui.drawn(), 'grid') as { props: Record<string, unknown> }
    expect(grid.props.borderStyle).toBe('single')
    // 표 줄은 모두 한 줄 : 테두리 2 + 머리글 1 + 가로줄 4 + 소단계 3 + 합계 1 (열린 mark 상세는 표 밖)
    expect(countRows(grid)).toBe(11)
    // 열린 mark 상세는 표 아래 한 줄
    expect(await ui.find({ text: /^하는 중 : m1007-c001 · 대기 없음 · 0초 전 갱신$/ })).toBeDefined()
    await ui.unmount()
  }
})

test('패널 표 : 하는 중 소단계가 여럿이면 표 아래 상세 줄 앞에 번호, 없으면 상세 줄이 없다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  w.bash.text = 'mark m1007-c002 시작 : 2-구현 (2026-10-07 12:00:00)'
  w.shows.set('m1007-c002', showText('m1007-c002', '2-구현', '1분', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "2-구현"' })
  await clock.advance(100)
  let ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(80) })
  expect(await ui.find({ text: /^1\. 하는 중 : m1007-c001 · 대기 없음 · / })).toBeDefined()
  expect(await ui.find({ text: /^2\. 하는 중 : m1007-c002 · 대기 없음 · / })).toBeDefined()
  await ui.unmount()
  // 둘 다 닫으면 하는 중 줄이 사라진다
  for (const [id, name, rec] of [['m1007-c001', '1-조사', '3분'], ['m1007-c002', '2-구현', '1분']] as const) {
    w.bash.text = showText(id, name, rec, false)
    w.shows.set(id, showText(id, name, rec, false))
    await $.tool.call({ tool: 'Bash', command: `effort mark stop ${id}` })
    await clock.advance(100)
  }
  ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(80) })
  expect(await ui.find({ text: /하는 중 : / })).toBeUndefined()
  await ui.unmount()
})

test('패널 표 : 75칸에서 긴 이름과 「1시간 16분」 이 잘리지 않는다, 넘치는 이름은 번호를 남기고 자른다', async ($, on) => {
  mock.clock(on, { now: T0 })
  const plan = {
    id: 'p3', title: '긴 이름', createdAt: T0 - 600_000, sessionId: SID, estNote: null,
    steps: [
      { n: 2, name: 'effort-steps 패널 손질', cls: '구현', size: 'M', estText: '1시간 16분', estMs: 4_560_000, markIds: [], status: '할 일', note: null },
      { n: 3, name: '3차 후보 조사와 정리, 그리고 아주 긴 꼬리말을 붙인 이름', cls: '조사', size: 'S', estText: '8분', estMs: 480_000, markIds: [], status: '할 일', note: null },
    ],
  }
  world(on, { [`plan:${SID}`]: { plan, marks: {} } })
  await $.session.start(START)
  const l = paneLayout(75)
  expect(l.cls).toBe(0)
  expect(l.est).toBeGreaterThanOrEqual(cellWidth('1시간 16분'))
  expect(l.act).toBeGreaterThanOrEqual(cellWidth('1시간 16분'))
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(75) })
  expect(await ui.find({ text: /^2\. effort-steps 패널 손질$/ })).toBeDefined()
  expect(await ui.find({ text: /^1시간 16분$/ })).toBeDefined()
  // 합계 예상도 다 들어간다 (1시간 16분 + 8분)
  expect(await ui.find({ text: /^1시간 24분$/ })).toBeDefined()
  const long = (await ui.findAll({ type: 'Text' })).map(n => n.text ?? '').find(t => t.startsWith('3. 3차 후보'))
  expect(long).toBeDefined()
  expect(long!.endsWith('…')).toBe(true)
  expect(cellWidth(long!)).toBe(l.name)
  await ui.unmount()
})

test('패널 표 모양 : 칸 너비와 구분자의 합은 언제나 테두리·여백 안쪽 폭', () => {
  for (const cols of [40, 50, 53, 57, 60, 66, 67, 75, 80, 99, 100, 140]) {
    const l = paneLayout(cols)
    const ws = [l.name, l.cls, l.size, l.est, l.act, l.status].filter(x => x > 0)
    expect(ws.reduce((a, b) => a + b, 0) + (ws.length - 1) * cellWidth(l.sep)).toBe(cols - PANE_FRAME)
    expect(cellWidth(ruleText(l))).toBe(cols - PANE_FRAME)
  }
})

test('패널 표 : 아주 좁으면(39칸 미만) 「이름 · 상태」 맨 줄로, 모든 줄이 안쪽 폭 안 (리뷰 12)', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await setup($, w, clock)
  for (const cols of [30, 38]) {
    expect(paneLayout(cols).plain).toBe(true)
    const inner = cols - PANE_FRAME
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(cols) })
    const grid = findKey(await ui.drawn(), 'grid') as Node
    expect(grid).toBeDefined()
    // 소단계 셋 + 합계 한 줄, 가로줄 없음
    expect(lineWidths(grid).length).toBe(4)
    for (const line of lineWidths(grid)) expect(line).toBeLessThanOrEqual(inner)
    expect(await ui.find({ text: /^1\. 조사 · 하는 중$/ })).toBeDefined()
    await ui.unmount()
  }
  expect(paneLayout(39).plain).toBe(false)
})

/** 표 Box 안 한 줄씩의 칸 폭 : Text 는 글 폭, 가로 Box 는 칸 Box 너비의 합 */
function lineWidths(grid: Node): number[] {
  if (typeof grid === 'string') return [cellWidth(grid)]
  return (grid.children ?? []).map(k => {
    if (typeof k === 'string') return cellWidth(k)
    if (k.type === 'Text') return cellWidth(textOf(k))
    return (k.children ?? []).reduce((a, c) => a + (typeof c === 'string' ? cellWidth(c) : Number(c.props?.width ?? cellWidth(textOf(c)))), 0)
  })
}

// ── 패널 머리글 : 지금 단계 한 줄 + 진행 막대 (2026-10-08) ──

type HeadStatus = '할 일' | '하는 중' | '끝' | '건너뜀'

/** 일곱 단계 계획. statuses 차례대로 상태를 붙이고, 하는 중 단계에는 열린 mark 를 하나씩 단다 (seenAt 은 opens 값) */
function headPlan(statuses: HeadStatus[], opens: Record<number, number> = {}, names: Record<number, string> = {}) {
  const base = ['문자표 정하기', '틀 잡기', '검증기 만들기', '연결', '시험', '문서', '뒷정리']
  const steps = statuses.map((status, i) => {
    const n = i + 1
    const id = `m1008-h00${n}`
    return {
      n, name: names[n] ?? base[i]!, cls: '구현', size: 'M', estText: n === 6 ? '—' : `${n * 10}분`, estMs: n === 6 ? null : n * 600_000,
      markIds: opens[n] === undefined ? [] : [id], status, note: null,
    }
  })
  const marks: Record<string, unknown> = {}
  for (const [n, seenAt] of Object.entries(opens)) {
    const id = `m1008-h00${n}`
    marks[id] = {
      id, name: `${n}-x`, n: Number(n), agentId: null, open: true, recordText: '1분', recordMs: 60_000, waitText: '없음',
      flags: [], warn: null, error: null, fails: 0, same: 0, seenAt, lastAt: T0,
    }
  }
  const plan = { id: 'p8', title: '공수표 머리글 붙이기', createdAt: T0 - 600_000, sessionId: SID, estNote: null, steps }
  return { plan, marks }
}

/** 막대 줄 (█ 와 ░ 만으로 된 Text) */
async function barLine(ui: { findAll: (q: { type: string }) => Promise<{ text?: string }[]> }): Promise<string | undefined> {
  return (await ui.findAll({ type: 'Text' })).map(n => n.text ?? '').find(t => /^[█░]+$/.test(t))
}

test('패널 머리글 : 하는 중이 여럿이면 가장 최근 시작한 것 · 「▶ 3/7  이름 · 예상」 · 막대는 끝 수만큼 초록', async ($, on) => {
  mock.clock(on, { now: T0 })
  // 3·4 가 하는 중, 3 이 더 늦게 시작했다
  const { plan, marks } = headPlan(['끝', '건너뜀', '하는 중', '하는 중', '할 일', '할 일', '할 일'], { 3: T0 - 60_000, 4: T0 - 300_000 })
  const w = world(on, { [`plan:${SID}`]: { plan, marks } })
  w.shows.set('m1008-h003', showText('m1008-h003', '3-x', '1분', true))
  w.shows.set('m1008-h004', showText('m1008-h004', '4-x', '1분', true))
  await $.session.start(START)
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(80) })
  expect(await ui.find({ text: /^공수 · 공수표 머리글 붙이기$/ })).toBeDefined()
  expect(await ui.find({ text: /^▶ 3\/7  검증기 만들기 · 예상 30분$/ })).toBeDefined()
  const bar = await barLine(ui)
  expect(bar).toBeDefined()
  expect([...bar!].length).toBe(80)
  // 끝 2개(끝 1 + 건너뜀 1) / 7 × 80 = 22.9 → 23 칸
  expect([...bar!].filter(c => c === '█').length).toBe(23)
  const greens = (await ui.findAll({ type: 'Text' })).filter(n => n.props.color === 'success').map(n => n.text)
  expect(greens).toContain('█'.repeat(23))
  // 머리글은 표 상자 바깥이라 표 줄 수는 그대로
  expect(countRows(findKey(await ui.drawn(), 'grid'))).toBe(2 + 1 + 1 + 7 + 6 + 1 + 1)
  await ui.unmount()
})

test('패널 머리글 : 전부 끝이면 「✓ 7/7  모두 끝」 과 꽉 찬 막대', async ($, on) => {
  mock.clock(on, { now: T0 })
  const { plan, marks } = headPlan(['끝', '끝', '끝', '건너뜀', '끝', '끝', '끝'])
  world(on, { [`plan:${SID}`]: { plan, marks } })
  await $.session.start(START)
  for (const cols of WIDTHS) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(cols) })
    expect(await ui.find({ text: /^✓ 7\/7  모두 끝$/ })).toBeDefined()
    expect(await barLine(ui)).toBe('█'.repeat(cols))
    await ui.unmount()
  }
})

test('패널 머리글 : 좁은 폭(50)에서 긴 이름은 「…」 로 잘리고 두 줄 모두 폭 안, 막대 글자 수 = 폭', async ($, on) => {
  mock.clock(on, { now: T0 })
  const longName = '아주 긴 소단계 이름이라서 오십 칸 패널에는 다 들어가지 않는 이름'
  const { plan, marks } = headPlan(['끝', '하는 중', '할 일', '할 일', '할 일', '할 일', '할 일'], { 2: T0 - 60_000 }, { 2: longName })
  const w = world(on, { [`plan:${SID}`]: { plan, marks } })
  w.shows.set('m1008-h002', showText('m1008-h002', '2-x', '1분', true))
  await $.session.start(START)
  for (const cols of [30, 50, 140]) {
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: paneProps(cols) })
    const line = (await ui.findAll({ type: 'Text' })).map(n => n.text ?? '').find(t => t.startsWith('▶ 2/7'))
    expect(line).toBeDefined()
    expect(cellWidth(line!)).toBeLessThanOrEqual(cols)
    if (cols === 50) expect(line!).toMatch(/^▶ 2\/7  아주 긴 .*… · 예상 20분$/)
    if (cols === 140) expect(line!).toBe(`▶ 2/7  ${longName} · 예상 20분`)
    expect([...((await barLine(ui)) ?? '')].length).toBe(cols)
    await ui.unmount()
  }
})

test('패널 머리글 (순수 함수) : 하는 중이 없으면 첫 할 일, 예상이 비면(—) 「· 예상」 을 뺀다, 0칸 폭도 막대 1칸', () => {
  const asPlan = (p: unknown) => p as Parameters<typeof paneHeader>[0]
  const asMarks = (m: unknown) => m as Parameters<typeof paneHeader>[1]
  const a = headPlan(['끝', '끝', '끝', '끝', '끝', '할 일', '할 일'])
  const h = paneHeader(asPlan(a.plan), asMarks(a.marks), 60)
  expect(h.head + h.body).toBe('▶ 6/7  문서')
  expect(h.current?.n).toBe(6)
  expect(h.filled).toBe(Math.round((5 / 7) * 60))
  expect(h.barWidth).toBe(60)
  const z = paneHeader(asPlan(a.plan), asMarks(a.marks), 0)
  expect(z.barWidth).toBe(1)
  expect(cellWidth(z.head + z.body)).toBeLessThanOrEqual(1)
  const e = headPlan([])
  const he = paneHeader(asPlan(e.plan), asMarks(e.marks), 20)
  expect(he.head + he.body).toBe('✓ 0/0  모두 끝')
  expect(he.filled).toBe(0)
})
