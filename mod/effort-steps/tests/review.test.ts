// 코드리뷰 반영 시험 (2026-10-07) : 항목 번호는 리뷰 지적 번호다.
import { describe, expect, mock, test } from 'claude-code/testing'
import type { RenderElement } from 'claude-code'

import type { Plan } from '../types'
import { exeCandidates } from '../hooks/effort'
import { newMark, pairOnReplan } from '../hooks/pair'
import { parseBlocks, parseMarksFile } from '../hooks/parse'
import { cellWidth, countRows, fitCells, padCellsStart } from '../hooks/view'
import { SID, STEPS, expectReadOnly, showText, world } from './world'

const T0 = Date.parse('2026-10-07T03:00:00.000Z')
const START = { cwd: 'C:/repo', surface: 'terminal', isInteractive: true } as const
const presentation = { isFullscreen: false, columns: 120 } as never
const origin = { kind: 'composer' } as never

function bandProps(cols: number, maxRows = 10) {
  return { hasSurvey: false, isWorking: false, maxRows, bodyColumns: cols, scroll: { offset: 0, bodyRows: maxRows }, view: {} }
}

/** 띠 트리에서 effort-steps 몫의 줄(바깥 Box 의 자식) 수와, 칩 줄이 몇 줄을 차지하나 */
function ownRows(tree: RenderElement): RenderElement[] {
  // key 는 그린 트리에서 props 안에 있다 (예전엔 t.key 만 봐서 포갠 띠를 못 알아봤다)
  const t = tree as unknown as { key?: string; props?: { key?: unknown }; children?: RenderElement[] }
  if ((t.key ?? t.props?.key) === 'effort-steps-stack') {
    const mine = (t.children ?? [])[0] as unknown as { children?: RenderElement[] }
    return ownRows((mine.children ?? [])[0]!)
  }
  return t.children ?? []
}

const LONG_STEPS = Array.from({ length: 10 }, (_, i) => ({ n: i + 1, name: `아주긴소단계이름${i + 1}`, class: '구현', size: 'M' }))

describe('1 · 띠 줄 수', () => {
  test('cellWidth 는 한글 2칸으로 세고, fitCells 는 넘치면 끝을 「…」 로 자른다', () => {
    expect(cellWidth('가나 ab')).toBe(7)
    expect(fitCells('가나다라', 8)).toBe('가나다라')
    expect(fitCells('가나다라', 7)).toBe('가나다…')
    expect(padCellsStart('—', 3)).toBe('  —')
  })

  test('소단계가 넘치면 하는 중 줄만 두고 나머지는 한 줄로 접는다 · 띠는 4줄을 넘지 않는다', async ($, on) => {
    const clock = mock.clock(on, { now: T0 })
    const w = world(on)
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '긴 계획', steps: LONG_STEPS })
    w.bash.text = 'mark m1007-c101 시작 : 1-조사 (2026-10-07 12:00:00)\nmark m1007-c102 시작 : 2-조사 (2026-10-07 12:00:00)'
    w.shows.set('m1007-c101', showText('m1007-c101', '1-조사', '1분', true))
    w.shows.set('m1007-c102', showText('m1007-c102', '2-조사', '1분', true))
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"; effort mark start "2-조사"' })
    await clock.advance(100)
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80) })
    expect(ownRows(await ui.drawn()).length).toBe(4) // 머리 · ▶1 · ▶2 · 접은 줄
    expect(await ui.find({ text: /^ ▶ 1 아주긴소단계이름1 / })).toBeDefined()
    expect(await ui.find({ text: /^ ▶ 2 아주긴소단계이름2 / })).toBeDefined()
    expect(await ui.find({ text: /^ · 8 할 일$/ })).toBeDefined()
    await ui.unmount()
  })

  test('접어도 넘치면 하는 중 줄은 가장 최근 것만 남긴다', async ($, on) => {
    const clock = mock.clock(on, { now: T0 })
    const w = world(on)
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '긴 계획', steps: LONG_STEPS })
    w.bash.text = 'mark m1007-c111 시작 : 1-조사 (2026-10-07 12:00:00)'
    w.shows.set('m1007-c111', showText('m1007-c111', '1-조사', '1분', true))
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
    await clock.advance(5_000)
    w.bash.text = 'mark m1007-c112 시작 : 2-조사 (2026-10-07 12:00:05)'
    w.shows.set('m1007-c112', showText('m1007-c112', '2-조사', '1분', true))
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "2-조사"' })
    await clock.advance(100)
    // maxRows 3 → 머리 1 + 소단계 몫 2 : 하는 중 둘 + 접은 줄은 3줄이라 최근 것(2) 하나와 접은 줄만
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(80, 3) })
    expect(ownRows(await ui.drawn()).length).toBe(3)
    expect(await ui.find({ text: /^ ▶ 2 / })).toBeDefined()
    expect(await ui.find({ text: /^ ▶ 1 아주긴/ })).toBeUndefined()
    expect(await ui.find({ text: /^ ▶ 1 하는 중 · · 8 할 일$/ })).toBeDefined()
    await ui.unmount()
  })

  test('아래 띠를 포개면 내 몫은 2줄 (4 - 2)', async ($, on) => {
    const clock = mock.clock(on, { now: T0 })
    // prompt-gauge 처럼 두 줄짜리 아래 띠 (countRows 가 실제 줄 수를 센다)
    const w = world(on, undefined, { belowText: '게이지 · 5h 42%\nweek 10%' })
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '포개기', steps: STEPS })
    w.bash.text = 'mark m1007-c103 시작 : 1-조사 (2026-10-07 12:00:00)'
    w.shows.set('m1007-c103', showText('m1007-c103', '1-조사', '1분', true))
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
    await clock.advance(100)
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(140) })
    expect(ownRows(await ui.drawn()).length).toBe(2)
    expect(await ui.find({ text: /게이지/ })).toBeDefined()
    await ui.unmount()
  })

  test('아래 띠가 3줄이면 내 몫은 1줄 (4 - 3) · countRows 는 세로 합 · 가로 최댓값 · Text 줄바꿈', async ($, on) => {
    const t = (s: string) => ({ type: 'Text', children: [s] })
    expect(countRows(null)).toBe(0)
    expect(countRows({ type: 'engine', ref: 0 })).toBe(0)
    expect(countRows(t('a\nb'))).toBe(2)
    expect(countRows({ type: 'Box', props: { flexDirection: 'column' }, children: [t('a'), t('b\nc')] })).toBe(3)
    expect(countRows({ type: 'Box', children: [t('a'), t('b\nc')] })).toBe(2)
    expect(countRows({ type: 'Text', children: ['a', { type: 'Text', children: ['\nb'] }] })).toBe(2)

    const clock = mock.clock(on, { now: T0 })
    // 띠 셋 포개기 : 아래에 agent-board 1줄 + prompt-gauge 2줄
    const w = world(on, undefined, { belowText: '에이전트 1 돌고 있음\n게이지 · 5h 42%\nweek 10%' })
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '셋 포개기', steps: STEPS })
    w.bash.text = 'mark m1007-c104 시작 : 1-조사 (2026-10-07 12:00:00)'
    w.shows.set('m1007-c104', showText('m1007-c104', '1-조사', '1분', true))
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
    await clock.advance(100)
    const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(140) })
    expect(ownRows(await ui.drawn()).length).toBe(1)
    expect(await ui.find({ text: /게이지/ })).toBeDefined()
    await ui.unmount()
  })
})

test('L10 · countRows : 테두리 Box 한 겹은 자식 줄 + 2', () => {
  const t = (s: string) => ({ type: 'Text', children: [s] })
  expect(countRows({ type: 'Box', props: { borderStyle: 'round', flexDirection: 'column' }, children: [t('a'), t('b')] })).toBe(4)
  expect(countRows({ type: 'Box', props: { borderStyle: 'single' }, children: [t('긴 글 '.repeat(50))] })).toBe(3) // 긴 글도 1줄
})

test('L10 · countRows : padding 1 은 +2 · paddingY·paddingTop·paddingBottom 도 센다', () => {
  const t = (s: string) => ({ type: 'Text', children: [s] })
  expect(countRows({ type: 'Box', props: { padding: 1 }, children: [t('a')] })).toBe(3)
  expect(countRows({ type: 'Box', props: { paddingY: 1, flexDirection: 'column' }, children: [t('a'), t('b')] })).toBe(4)
  expect(countRows({ type: 'Box', props: { padding: 2, paddingTop: 0 }, children: [t('a')] })).toBe(3)
  expect(countRows({ type: 'Box', props: { paddingBottom: 1, borderStyle: 'round' }, children: [t('a')] })).toBe(4)
})

test('2 · marks.txt 에서 처음 본 닫힌 mark 는 show 가 한 번 실패해도 다음 틱에 채운다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '닫힌 mark', steps: STEPS })
  w.marksTxt = [
    `start\tm1007-d201\t2026-10-07T03:00:05.000Z\tC--proj\t${SID}\t2-구현`,
    'stop\tm1007-d201\t2026-10-07T03:00:50.000Z',
  ].join('\n')
  await clock.advance(30_000) // 첫 틱 : show 출력 없음 → 실패 1
  let table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 2. 구현 | 2분 | — ? |') // 첫 show 가 실패해 값 없음 · 불확실
  w.shows.set('m1007-d201', showText('m1007-d201', '2-구현', '1분 미만 (45초)', false))
  await clock.advance(30_000) // 다음 틱이 다시 잰다
  table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 2. 구현 | 2분 | 1분 미만 (45초) | 끝 |')
  // 값을 읽었으니 그다음 틱부터는 이 mark 를 다시 재지 않는다
  const before = w.argvs.length
  await clock.advance(30_000)
  expect(w.argvs.length).toBe(before)
  expectReadOnly(w.argvs)
})

describe('3 · 10 · exe 후보', () => {
  test('일반 <base>/bin 은 보지 않는다', () => {
    const c = exeCandidates('', ['C:/repo'], '')
    expect(c).toEqual([
      'C:/repo/EffortTool/bin/effort.exe', 'C:/repo/EffortTool/bin/effort',
      'C:/repo/Tools/EffortTool/bin/effort.exe', 'C:/repo/Tools/EffortTool/bin/effort',
    ])
    expect(c.some(p => p.startsWith('C:/repo/bin/'))).toBe(false)
  })

  test('설정 effortPath 가 있으면 그것이 먼저, 그 뒤에 나머지 후보를 잇는다 (설치본 default 가 낡아도 찾게)', () => {
    expect(exeCandidates(' D:/x/effort.exe ', ['C:/repo'], 'C:/bin')).toEqual([
      'D:/x/effort.exe',
      'C:/repo/EffortTool/bin/effort.exe', 'C:/repo/EffortTool/bin/effort',
      'C:/repo/Tools/EffortTool/bin/effort.exe', 'C:/repo/Tools/EffortTool/bin/effort',
      'C:/bin/effort.exe', 'C:/bin/effort',
    ])
  })

  test('PATH : 드라이브 글자 꼴이면 ; 로 자른다 (한 칸이어도)', () => {
    expect(exeCandidates('', [], 'C:\\tools').slice(-2)).toEqual(['C:\\tools/effort.exe', 'C:\\tools/effort'])
    expect(exeCandidates('', [], 'C:\\a;D:\\b')).toHaveLength(4)
    expect(exeCandidates('', [], '/usr/bin:/opt/e')).toEqual(['/usr/bin/effort.exe', '/usr/bin/effort', '/opt/e/effort.exe', '/opt/e/effort'])
  })
})

test('4 · 다른 갱신이 돌고 있으면 /steps refresh 는 「다른 갱신 중」 이라고 답한다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  let release: () => void = () => undefined
  let reachedNow: () => void = () => undefined
  const reached = new Promise<void>(r => { reachedNow = r })
  let hold = false
  // hold 동안 show 하나를 붙잡아 갱신이 도는 중인 때를 만든다 (붙잡으면 reached 가 풀린다)
  const w = world(on, undefined, {
    beforeRun: argv => {
      if (!hold || argv[2] !== 'show') return undefined
      hold = false
      reachedNow()
      return new Promise<void>(r => { release = r })
    },
  })
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '바쁨', steps: STEPS })
  w.bash.text = 'mark m1007-e401 시작 : 1-조사 (2026-10-07 12:00:00)'
  w.shows.set('m1007-e401', showText('m1007-e401', '1-조사', '1분', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  await clock.advance(100)
  hold = true
  const ticking = clock.advance(30_000)
  await reached // 틱이 show 에서 붙잡힐 때까지 기다린다
  const busyText = (await $.command.run({ command: 'steps', args: 'refresh', origin, presentation })).text
  release()
  await ticking
  expect(busyText).toContain('다른 갱신 중')
  const okText = (await $.command.run({ command: 'steps', args: 'refresh', origin, presentation })).text
  expect(okText).toContain('다시 쟀다')
})

test('5 · plan 을 다시 불러 갈아 끼워도 잰 mark 는 번호로 다시 짝짓는다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '처음 계획', steps: STEPS })
  w.bash.text = 'mark m1007-f501 시작 : 1-조사 (2026-10-07 12:00:00)'
  w.shows.set('m1007-f501', showText('m1007-f501', '1-조사', '1분', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  await clock.advance(100)
  w.bash.text = showText('m1007-f501', '1-조사', '3분 (2분 50초)', false)
  await $.tool.call({ tool: 'Bash', command: 'effort mark stop m1007-f501' })
  await clock.advance(100)
  // 짝 문턱(60초)을 훨씬 넘긴 뒤 계획을 갈아 끼운다
  await clock.advance(10 * 60_000)
  const replanned = String((await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '고친 계획', steps: [...STEPS, { n: 4, name: '문서', class: '문서', size: 'S' }] })).result)
  expect(replanned).toContain('| 1. 조사 | 4분 | 3분 (2분 50초) | 끝 |')
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 1. 조사 | 4분 | 3분 (2분 50초) | 끝 |')
  expectReadOnly(w.argvs)
})

test('5b · 계획 갈아 끼운 뒤 옛 mark 짝짓기 : 짝 없이 남은 닫힌 mark 는 새 번호에 붙고, 열린 mark 는 끊기지 않는다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '처음 계획', steps: STEPS })
  // 2번은 열린 채로 둔다
  w.bash.text = 'mark m1007-e5df 시작 : 2-구현 (2026-10-07 12:00:00)'
  w.shows.set('m1007-e5df', showText('m1007-e5df', '2-구현', '1분', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "2-구현"' })
  await clock.advance(100)
  // 계획에 없는 5번 mark 를 열고 닫는다 → 짝 없는 mark
  await clock.advance(60_000)
  w.bash.text = 'mark m1007-1cee 시작 : 5-effort-steps 패널 손질 (2026-10-07 12:01:00)'
  w.shows.set('m1007-1cee', showText('m1007-1cee', '5-effort-steps 패널 손질', '4분 (3분 50초)', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "5-effort-steps 패널 손질"' })
  await clock.advance(100)
  w.bash.text = showText('m1007-1cee', '5-effort-steps 패널 손질', '4분 (3분 50초)', false)
  w.shows.set('m1007-1cee', w.bash.text)
  await $.tool.call({ tool: 'Bash', command: 'effort mark stop m1007-1cee' })
  await clock.advance(100)
  const before = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(before).not.toContain('패널 손질')
  // 짝 없는 mark 도 계획 사본(store)에 남는다 — 리로드 뒤에도 잃지 않게
  const copy = w.store.get(`plan:${SID}`) as { marks: Record<string, unknown> }
  expect(Object.keys(copy.marks)).toContain('m1007-1cee')
  // 짝 문턱(60초)을 넘긴 뒤 5번을 더해 계획을 갈아 끼운다
  await clock.advance(10 * 60_000)
  const steps5 = [...STEPS, { n: 5, name: '패널 손질', class: '구현', size: 'S' }]
  const replanned = String((await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '고친 계획', steps: steps5 })).result)
  expect(replanned).toContain('| 5. 패널 손질 | ')
  expect(replanned).toMatch(/\| 5\. 패널 손질 \| [^|]+ \| 4분 \(3분 50초\) \| 끝 \|/)
  expect(replanned).toMatch(/\| 2\. 구현 \| [^|]+ \| 1분 \| 하는 중 \|/)
  const saved = w.store.get(`plan:${SID}`) as { plan: { steps: { n: number; markIds: string[] }[] }; marks: Record<string, { n: number | null }> }
  expect(saved.plan.steps.find(s => s.n === 5)!.markIds).toEqual(['m1007-1cee'])
  expect(saved.plan.steps.find(s => s.n === 2)!.markIds).toEqual(['m1007-e5df'])
  expect(saved.marks['m1007-1cee']!.n).toBe(5)
  // 짝 없는 mark 줄이 사라진다
  const ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'Pane', requestId: 'effort-steps', props: { title: '공수', isFocused: false, bodyColumns: 80, placement: 'dock' as const, scroll: { offset: 0, bodyRows: 30 }, view: {} } })
  expect(await ui.find({ text: /짝 없는 mark/ })).toBeUndefined()
  await ui.unmount()
  expectReadOnly(w.argvs)
})

test('5d · 리로드로 상태에서 잃은 짝 없는 mark 도 plan 을 다시 부르면 marks.txt 에서 이름으로 주워 붙인다 (실물 버그)', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  // 되살릴 계획 사본 : 1~3번뿐이고, 짝 없는 mark 는 사본에 없다 (고치기 전 판이 남긴 꼴)
  const plan = {
    id: 'p1', title: '되살린 계획', createdAt: T0 - 600_000, sessionId: SID, estNote: null,
    steps: STEPS.map(s => ({ n: s.n, name: s.name, cls: s.class, size: s.size, estText: '—', estMs: null, markIds: [], status: '할 일', note: null })),
  }
  const w = world(on, { [`plan:${SID}`]: { plan, marks: {} } })
  // 계획 사본보다 앞에 연(=시각 문턱에 걸리는) 5번 mark 가 marks.txt 에 있다
  w.marksTxt = [
    `start\tm1007-1cee\t${new Date(T0 - 900_000).toISOString()}\tC--proj\t${SID}/agent-aa9f\t5-패널 손질`,
    `stop\tm1007-1cee\t${new Date(T0 - 700_000).toISOString()}`,
  ].join('\n')
  w.shows.set('m1007-1cee', showText('m1007-1cee', '5-패널 손질', '5분 (4분 33초)', false))
  await $.session.start(START)
  await clock.advance(10 * 60_000)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '다시 넣은 계획', steps: [...STEPS, { n: 5, name: '패널 손질', class: '구현', size: 'S' }] })
  await clock.advance(100)
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toMatch(/\| 5\. 패널 손질 \| [^|]+ \| 5분 \(4분 33초\) \| 끝 \|/)
  expectReadOnly(w.argvs)
})

test('5c · 갈아 끼울 때 번호 짝은 옛·새 계획의 소단계 이름이 같은 번호만 (리뷰 4)', () => {
  const st = (n: number, name: string) => ({ n, name, cls: '구현', size: 'S', estText: '—', estMs: null, markIds: [], status: '할 일' as const, note: null })
  const old: Plan = { id: 'p1', title: '옛', createdAt: 1_000_000, sessionId: SID, estNote: null, steps: [st(5, '다섯'), st(6, '육')] }
  const next: Plan = { ...old, id: 'p2', createdAt: 2_000_000, prevAt: 1_000_000, steps: [st(5, '다섯'), st(6, '여섯'), st(7, '일곱')] }
  // 옛 계획 시절에 연 mark : 5번은 이름이 그대로라 mark 이름이 달라도 번호로 붙는다
  const late5 = { ...newMark('m-5', '5-다른일', null, null, 1_500_000), open: false }
  expect(pairOnReplan(old, next, late5)).toBe(5)
  // 6번은 이름이 바뀌었다 (육 → 여섯) : 번호만으로는 안 붙는다, mark 이름이 새 이름과 같으면 붙는다
  expect(pairOnReplan(old, next, { ...newMark('m-6', '6-육', null, null, 1_500_000), open: false })).toBeNull()
  expect(pairOnReplan(old, next, { ...newMark('m-6b', '6-여섯', null, null, 1_500_000), open: false })).toBe(6)
  // 옛 계획에 없던 7번 : 짝 없이 남은 mark 라 번호로 붙는다 (소단계를 더해 다시 부른 경우 · 5b)
  expect(pairOnReplan(old, next, { ...newMark('m-7', '7-다른일', null, null, 1_500_000), open: false })).toBe(7)
  // 첫 계획보다 한참(10분 넘게) 먼저 연 mark 는 이름이 같아도 안 붙는다 (이름 짝 하한 = 이전 계획 − 10분)
  expect(pairOnReplan(old, next, { ...newMark('m-early', '5-다섯', null, null, 1_000_000 - 900_000), open: false })).toBeNull()
  // 옛 계획이 없으면 시각 문턱 · 이름만 본다
  expect(pairOnReplan(null, next, late5)).toBeNull()
})

test('5e · 다른 일로 갈아 끼운 계획에는 옛 계획에 없던 번호의 짝 없는 mark 를 번호로 붙이지 않는다 (실물 버그 2026-10-10)', () => {
  // 실물 값 : store plan:b359b749… 의 계획 둘과 ~/.effort/marks.txt 의 mark 둘
  const st = (n: number, name: string) => ({ n, name, cls: '구현', size: 'S', estText: '—', estMs: null, markIds: [], status: '할 일' as const, note: null })
  const old: Plan = {
    id: 'p-old', title: 'AIMemoryTool 기억 점검·정리 기능', createdAt: 1791624042536, sessionId: SID, estNote: null,
    steps: [
      st(1, '설계 문서(빈 곳 다섯·붙일 자리)'), st(2, '설계 검토·사용자 확인'), st(3, '구현(review 큐·규칙·gc 접기·value 버그)'),
      st(4, '시험·코드리뷰'), st(5, '스튜디오 기억 점검 실측(1117건)'), st(6, '문서·mem·커밋'),
    ],
  }
  const next: Plan = {
    id: 'p-new', title: 'JudgeModel 판정 넷 확장 (commit-prep 뒷정리 포함)', createdAt: 1791636457713, prevAt: old.createdAt,
    sessionId: SID, estNote: null,
    steps: [
      st(1, 'commit-prep 오탐 둘'), st(2, '판정 확장 조사'), st(3, '확장 설계 문서'), st(4, '판정 셋 새로 만들기'),
      st(5, 'JudgeModel 학습·내보내기 넷 칸'), st(6, '미니PC 덧학습·blind 재측정'),
      st(7, 'AIMemoryTool 손님 넷 칸·merge 2차'), st(8, '코드리뷰·문서·커밋'),
    ],
  }
  // 옛 계획(1~6) 시절에 연, 계획 밖 번호의 mark (m1010-ce30 · m1010-bf3a) — 짝 없이 남아 있었다
  const m7 = { ...newMark('m1010-ce30', '7-문턱보완', null, 'a8c7ca9481a96a1b4', 1791627958739), open: false, recordText: '6분 (6분 28초)', recordMs: 388000 }
  const m8 = { ...newMark('m1010-bf3a', '8-문턱보완2', null, 'ae2c3ea30c60e44c8', 1791628469510), open: false, recordText: '8분 (8분 7초)', recordMs: 487000 }
  expect(pairOnReplan(old, next, m7)).toBeNull()
  expect(pairOnReplan(old, next, m8)).toBeNull()
  // 이름이 같으면 이름 짝으로는 붙는다 (하한 = 이전 계획 − 10분 안)
  expect(pairOnReplan(old, next, { ...m7, id: 'm-same', name: '7-AIMemoryTool 손님 넷 칸·merge 2차' })).toBe(7)
})

test('6 · mark 이름은 제어 글자를 빼고 80자로 자른다', () => {
  const long = '1-' + '가'.repeat(200)
  const blocks = parseBlocks(`mark m1007-0601 시작 : ${long} (2026-10-07 12:00:00)`)
  expect(blocks[0]!.name).toHaveLength(80)
  const lines = parseMarksFile(`start\tm1007-0602\t2026-10-07T03:00:00.000Z\tC--proj\tsess-1\t2-a${String.fromCharCode(7)}b`)
  expect(lines[0]!.name).toBe('2-a b')
})

describe('7 · 8 · 실패와 멈춤', () => {
  test('show 가 잇달아 실패하는 mark 하나는 그것만 빠지고 다른 mark 는 계속 잰다', async ($, on) => {
    const clock = mock.clock(on, { now: T0 })
    const w = world(on)
    const toasts: string[] = []
    on('ui.toast', ($, e) => {
      toasts.push(e.text)
      return { value: undefined }
    })
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '하나만 실패', steps: STEPS })
    w.bash.text = 'mark m1007-a701 시작 : 1-조사 (2026-10-07 12:00:00)\nmark m1007-a702 시작 : 2-구현 (2026-10-07 12:00:00)'
    w.shows.set('m1007-a702', showText('m1007-a702', '2-구현', '1분', true)) // a701 은 show 출력이 없다
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"; effort mark start "2-구현"' })
    await clock.advance(100)
    await clock.advance(30_000)
    await clock.advance(30_000) // a701 3번째 실패 → a701 만 뺀다
    w.shows.set('m1007-a702', showText('m1007-a702', '2-구현', '5분', true))
    await clock.advance(30_000)
    const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
    expect(table).toContain('| 2. 구현 | 2분 | 5분 | 하는 중 |')
    expect(toasts.filter(t => t.includes('m1007-a701'))).toHaveLength(1)
    const last = w.argvs.slice(-1)[0]!
    expect(last[3]).toBe('m1007-a702')
    expect(w.argvs.filter(a => a[3] === 'm1007-a701')).toHaveLength(3)
    expectReadOnly(w.argvs)
  })

  test('exe 실행이 잇달아 실패하면 전체를 멈추고, /steps clear 가 멈춤을 푼다', async ($, on) => {
    const clock = mock.clock(on, { now: T0 })
    const w = world(on, undefined, { beforeRun: argv => (argv[2] === 'show' ? '실행 막힘' : undefined) })
    await $.session.start(START)
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: 'exe 실패', steps: STEPS })
    w.bash.text = 'mark m1007-b801 시작 : 1-조사 (2026-10-07 12:00:00)'
    await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
    await clock.advance(100)
    await clock.advance(30_000)
    await clock.advance(30_000)
    let ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(140) })
    expect(await ui.find({ text: /갱신 멈춤/ })).toBeDefined()
    await ui.unmount()
    expect((await $.command.run({ command: 'steps', args: 'clear', origin, presentation })).text).toContain('기록으로 옮기고')
    await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '새 계획', steps: STEPS })
    ui = await $.ui.mount({ plugin: 'effort-steps', surface: 'terminal', component: 'AbovePrompt', props: bandProps(140) })
    expect(await ui.find({ text: /갱신 멈춤/ })).toBeUndefined()
    await ui.unmount()
  })
})

test('9 · 오래된 plan 키를 지울 때 지금 세션 키는 지우지 않는다', async ($, on) => {
  mock.clock(on, { now: T0 })
  const plan = {
    id: 'p1', title: '되살린 계획', createdAt: T0 - 600_000, sessionId: SID, estNote: null,
    steps: [{ n: 1, name: '조사', cls: '조사', size: 'M', estText: '4분', estMs: 226_961, markIds: [], status: '할 일', note: null }],
  }
  // 지금 세션 키가 맨 앞(가장 오래된 자리)에 있고, 남의 세션 키가 25개 뒤따른다
  const store: Record<string, unknown> = { [`plan:${SID}`]: { plan, marks: {} } }
  for (let i = 0; i < 25; i++) store[`plan:other-${i}`] = { plan: { ...plan, sessionId: `other-${i}` }, marks: {} }
  const w = world(on, store)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__step', n: 1, status: '하는 중' })
  expect(w.store.get(`plan:${SID}`)).toBeDefined()
  const keys = [...w.store.keys()].filter(k => k.startsWith('plan:'))
  expect(keys.length).toBe(20)
  expect(keys).not.toContain('plan:other-0')
})
