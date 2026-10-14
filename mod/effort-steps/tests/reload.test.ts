// 타이머 다시 걸기 · 리로드(session.start 가 또 옴) · --resume 되살림 시험
import { expect, mock, test } from 'claude-code/testing'

import { SID, STEPS, expectReadOnly, showText, world } from './world'

const T0 = Date.parse('2026-10-07T03:00:00.000Z')
const START = { cwd: 'C:/repo', surface: 'terminal', isInteractive: true } as const

test('session.start 가 두 번 와도 타이머는 하나', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '리로드', steps: STEPS })
  w.bash.text = 'mark m1007-a001 시작 : 1-조사 (2026-10-07 12:00:00)'
  w.shows.set('m1007-a001', showText('m1007-a001', '1-조사', '1분', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  await clock.advance(100)
  const before = w.argvs.filter(a => a[2] === 'show').length
  await clock.advance(30_000)
  const after = w.argvs.filter(a => a[2] === 'show').length
  expect(after - before).toBe(1)
  expectReadOnly(w.argvs)
})

test('열린 mark 가 없으면 타이머는 effort 를 부르지 않는다', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '조용', steps: STEPS })
  const before = w.argvs.length
  await clock.advance(120_000)
  expect(w.argvs.length).toBe(before)
})

test('--resume 이면 store 의 계획 사본을 되살린다', async ($, on) => {
  mock.clock(on, { now: T0 })
  const plan = {
    id: 'p1', title: '되살린 계획', createdAt: T0 - 600_000, sessionId: SID, estNote: null,
    steps: [{ n: 1, name: '조사', cls: '조사', size: 'M', estText: '4분', estMs: 226_961, markIds: ['m1007-b001'], status: '끝', note: null }],
  }
  const marks = {
    'm1007-b001': {
      id: 'm1007-b001', name: '1-조사', n: 1, agentId: null, open: false, recordText: '5분', recordMs: 300_000,
      waitText: '없음', flags: [], warn: null, error: null, fails: 0, same: 0, seenAt: T0 - 500_000, lastAt: T0 - 100_000,
    },
  }
  world(on, { [`plan:${SID}`]: { plan, marks } })
  await $.session.start(START)
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('**되살린 계획**')
  expect(table).toContain('| 1. 조사 | 4분 | 5분 | 끝 |')
})

test('/steps 명령 : table · hide · show · clear', async ($, on) => {
  mock.clock(on, { now: T0 })
  world(on)
  await $.session.start(START)
  const presentation = { isFullscreen: false, columns: 120 } as never
  const origin = { kind: 'composer' } as never
  const run = (args: string) => $.command.run({ command: 'steps', args, origin, presentation })
  expect((await run('table')).text).toContain('계획이 없다')
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '명령', steps: STEPS })
  expect((await run('table')).text).toContain('| 1. 조사 |')
  expect((await run('hide')).text).toContain('껐다')
  expect((await run('show')).text).toContain('켰다')
  expect((await run('clear')).text).toContain('기록으로 옮기고')
  expect((await run('table')).text).toContain('계획이 없다')
  expect((await run('zzz')).text).toContain('쓰는 법')
})

test('step 도구는 상태와 까닭만 받는다', async ($, on) => {
  mock.clock(on, { now: T0 })
  world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '스텝', steps: STEPS })
  const r = String((await $.tool.call({ tool: 'mcp__effort-steps__step', n: 3, status: '건너뜀', note: '이번 판 밖' })).result)
  expect(r).toContain('3번 소단계 : 건너뜀 · 이번 판 밖')
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 3. 시험 | 4분 | — | 건너뜀 | 이번 판 밖 |')
  const bad = String((await $.tool.call({ tool: 'mcp__effort-steps__step', n: 9 })).result)
  expect(bad).toContain('9 번 소단계가 없다')
})

test('plan 입력 검사 : 겹친 번호와 모르는 분류', async ($, on) => {
  mock.clock(on, { now: T0 })
  world(on)
  await $.session.start(START)
  const dup = await $.tool.call({ tool: 'mcp__effort-steps__plan', title: 'x', steps: [{ n: 1, name: 'a', class: '조사' }, { n: 1, name: 'b', class: '조사' }] })
  expect(String(dup.result)).toContain('두 번 나온다')
  const cls = await $.tool.call({ tool: 'mcp__effort-steps__plan', title: 'x', steps: [{ n: 1, name: 'a', class: '놀기' }] })
  expect(String(cls.result)).toContain('를 모른다')
})

test('step 도구에 명시 상태가 오면 붙은 mark 를 떼고 그 상태로 둔다, 「끝」 이면 mark 를 둔다 (실물 버그 2026-10-10)', async ($, on) => {
  const clock = mock.clock(on, { now: T0 })
  const w = world(on)
  await $.session.start(START)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '스텝', steps: STEPS })
  // 1번·2번에 닫힌 mark 를 하나씩 붙인다
  for (const [id, name] of [['m1007-b001', '1-조사'], ['m1007-b002', '2-구현']] as const) {
    w.bash.text = `mark ${id} 시작 : ${name} (2026-10-07 12:00:00)`
    w.shows.set(id, showText(id, name, '1분', true))
    await $.tool.call({ tool: 'Bash', command: `effort mark start "${name}"` })
    await clock.advance(100)
    w.bash.text = showText(id, name, '3분 (2분 50초)', false)
    w.shows.set(id, w.bash.text)
    await $.tool.call({ tool: 'Bash', command: `effort mark stop ${id}` })
    await clock.advance(100)
  }
  const before = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(before).toContain('| 1. 조사 | 4분 | 3분 (2분 50초) | 끝 |')
  // 할 일 : mark 를 떼어 짝 없는 mark 로 돌린다
  const r = String((await $.tool.call({ tool: 'mcp__effort-steps__step', n: 1, status: '할 일', note: '잘못 붙은 mark 뗌' })).result)
  expect(r).toContain('1번 소단계 : 할 일')
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 1. 조사 | 4분 | — | 할 일 |')
  const saved = w.store.get(`plan:${SID}`) as { plan: { steps: { n: number; markIds: string[] }[] }; marks: Record<string, { n: number | null }> }
  expect(saved.plan.steps.find(s => s.n === 1)!.markIds).toEqual([])
  expect(saved.marks['m1007-b001']!.n).toBeNull()
  // 다음 갱신이 와도 다시 붙지 않는다
  await clock.advance(60_000)
  const later = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(later).toContain('| 1. 조사 | 4분 | — | 할 일 |')
  // 끝 : mark 가 있으면 그대로 둔다
  await $.tool.call({ tool: 'mcp__effort-steps__step', n: 2, status: '끝' })
  const done = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(done).toContain('| 2. 구현 | ')
  expect(done).toMatch(/\| 2\. 구현 \| [^|]+ \| 3분 \(2분 50초\) \| 끝 \|/)
  expectReadOnly(w.argvs)
})
