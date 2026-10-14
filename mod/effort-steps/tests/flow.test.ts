// 흐름 시험 : plan → mark start 관찰 → 타이머 갱신 → stop 관찰 → table.
// process.run 은 가짜로 답하고, mod 가 넘긴 argv 를 모두 모아 금지 인자가 없는지 본다.
import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'
import type { ToolCallResult } from 'claude-code'

import { showStopped } from './fixtures'
import { EXE, SID, STEPS, TEMP, THROW, expectReadOnly, showText, world } from './world'

test('plan → start → 타이머 → stop → table', async ($, on) => {
  const clock = mock.clock(on, { now: Date.parse('2026-10-07T03:00:00.000Z') })
  const w = world(on)
  await $.session.start({ cwd: 'C:/repo', surface: 'terminal', isInteractive: true })

  const planned = await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '시험 작업', steps: STEPS })
  expect(String(planned.result)).toContain('| 1. 조사 | 4분 | — | 할 일 |')
  expect(String(planned.result)).toContain('| 2. 구현 | 2분 | — | 할 일 |')
  expect(w.argvs[0]).toEqual([EXE, 'estimate', '--json', '1:조사:M', '2:구현:S', '3:구현:M'])

  // Claude 가 mark start 를 친다 → mod 는 출력만 읽는다
  w.bash.text = 'mark m1007-aaaa 시작 : 1-조사 (2026-10-07 12:00:00)\n끝낼 때 : effort mark stop m1007-aaaa'
  w.shows.set('m1007-aaaa', showText('m1007-aaaa', '1-조사', '1분 미만 (5초)', true))
  await $.tool.call({ tool: 'Bash', command: '.\\EffortTool\\bin\\effort.exe mark start "1-조사"' })
  await clock.advance(100)
  let table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 1. 조사 | 4분 | 1분 미만 (5초) | 하는 중 |')

  // 30초 타이머가 다시 잰다
  w.shows.set('m1007-aaaa', showText('m1007-aaaa', '1-조사', '3분 (2분 40초)', true))
  await clock.advance(30_000)
  table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 1. 조사 | 4분 | 3분 (2분 40초) | 하는 중 |')

  // stop 과 다음 start 를 한 줄로 친다
  w.bash.text = [
    showText('m1007-aaaa', '1-조사', '4분 (3분 41초)', false),
    'mark m1007-bbbb 시작 : 2-구현 (2026-10-07 12:04:00)',
  ].join('\n')
  w.shows.set('m1007-bbbb', showText('m1007-bbbb', '2-구현', '1분 미만 (2초)', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark stop m1007-aaaa; effort mark start "2-구현"' })
  await clock.advance(100)
  table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 1. 조사 | 4분 | 4분 (3분 41초) | 끝 |')
  expect(table).toContain('| 2. 구현 | 2분 | 1분 미만 (2초) | 하는 중 |')
  expect(table).toContain('mem add --type history')

  expectReadOnly(w.argvs)
  expect(w.argvs.some(a => a[2] === 'show')).toBe(true)
})

test('결과를 못 읽으면 marks.txt 보조 길로 찾는다', async ($, on) => {
  const t0 = Date.parse('2026-10-07T03:00:00.000Z')
  const clock = mock.clock(on, { now: t0 })
  const w = world(on)
  await $.session.start({ cwd: 'C:/repo', surface: 'terminal', isInteractive: true })
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '보조 길', steps: STEPS })

  w.bash.text = '' // PowerShell 결과를 못 읽은 경우와 같다
  w.marksTxt = [
    `start\tm1007-cccc\t2026-10-07T03:00:05.000Z\tC--proj\t${SID}/agent-a9\t3-시험`,
    `start\tm1007-dddd\t2026-10-07T03:00:05.000Z\tC--proj\tother-session\t3-시험`,
  ].join('\n')
  w.shows.set('m1007-cccc', showText('m1007-cccc', '3-시험', '2분 (1분 50초)', true))
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "3-시험"' })
  await clock.advance(100)
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 3. 시험 | 4분 | 2분 (1분 50초) | 하는 중 |')
  expect(w.argvs.some(a => a[3] === 'm1007-dddd')).toBe(false)
  expectReadOnly(w.argvs)
})

test('show 가 세 번 잇달아 실패하면 타이머 갱신을 멈춘다', async ($, on) => {
  const clock = mock.clock(on, { now: Date.parse('2026-10-07T03:00:00.000Z') })
  const w = world(on)
  const toasts: string[] = []
  on('ui.toast', ($, e) => {
    toasts.push(e.text)
    return { value: undefined }
  })
  await $.session.start({ cwd: 'C:/repo', surface: 'terminal', isInteractive: true })
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '실패', steps: STEPS })
  w.bash.text = 'mark m1007-eeee 시작 : 1-조사 (2026-10-07 12:00:00)'
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  await clock.advance(100) // 1번째 실패 (show 출력 없음)
  await clock.advance(30_000) // 2번째
  await clock.advance(30_000) // 3번째 → 멈춤
  const before = w.argvs.length
  await clock.advance(30_000)
  expect(w.argvs.length).toBe(before)
  expect(toasts.length).toBe(1)
  expectReadOnly(w.argvs)
})

test('effort 가 아닌 명령과 mark show 출력만 있는 명령은 계획을 안 건드린다', async ($, on) => {
  const clock = mock.clock(on, { now: Date.parse('2026-10-07T03:00:00.000Z') })
  const w = world(on)
  await $.session.start({ cwd: 'C:/repo', surface: 'terminal', isInteractive: true })
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '조용', steps: STEPS })
  w.bash.text = showStopped
  await $.tool.call({ tool: 'Bash', command: 'effort mark show m1007-473f' })
  w.bash.text = 'mark m1007-ffff 시작 : 1-조사 (2026-10-07 12:00:00)'
  await $.tool.call({ tool: 'Bash', command: 'echo hello' })
  await clock.advance(100)
  const table = String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)
  expect(table).toContain('| 3. 시험 | 4분 | — | 할 일 |')
  expect(table).toContain('| 1. 조사 | 4분 | — | 할 일 |')
  expectReadOnly(w.argvs)
})

// ── 강제 : plan 없는 편집·Agent (설계 2026-10-14 3절) ──────────────

/** 강제 시험 대역 : 시계 + world */
function gw(on: Parameters<typeof world>[0]): ReturnType<typeof world> {
  mock.clock(on, { now: Date.parse('2026-10-07T03:00:00.000Z') })
  return world(on)
}

async function begin($: Engine, isInteractive = true): Promise<void> {
  await $.session.start({ cwd: 'C:/repo', surface: isInteractive ? 'terminal' : null, isInteractive } as never)
}

async function say($: Engine, text: string): Promise<void> {
  await $.prompt.submit({ text, wait: false, origin: { kind: 'composer' } } as never)
}

async function steps($: Engine, args: string, kind: 'composer' | 'sdk' = 'composer'): Promise<string> {
  const r = await $.command.run({ command: 'steps', args, origin: { kind }, presentation: 'text' } as never)
  return String((r as { text?: string }).text ?? '')
}

async function edit($: Engine, path: string, agentId?: string): Promise<ToolCallResult> {
  return $.tool.call({ tool: 'Edit', file_path: path, old_string: 'a', new_string: 'b', ...(agentId === undefined ? {} : { agentId }) } as never)
}

async function agent($: Engine, agentId?: string): Promise<ToolCallResult> {
  return $.tool.call({ tool: 'Agent', description: 'd', prompt: 'p', model: 'opus', ...(agentId === undefined ? {} : { agentId }) } as never)
}

async function plan($: Engine): Promise<void> {
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '강제 시험', steps: STEPS })
}

/** 막혔으면 그 글, 아니면 null (테스트 $ 는 deny 를 isError 결과로 받는다) */
function denied(r: ToolCallResult): string | null {
  if (r.deny !== undefined) return r.deny
  if (r.isError === true) return typeof r.text === 'string' ? r.text : String(r.result)
  return null
}

function notes(r: ToolCallResult): string[] {
  return (r.context ?? []).filter(c => c.startsWith('effort-steps'))
}

test('강제 deny : plan 없으면 Edit · Write · NotebookEdit · Agent · spawn 이 막힌다', async ($, on) => {
  const w = gw(on)
  await begin($)
  const r = await edit($, 'C:/repo/a.ts')
  expect(denied(r)).toContain('공수 계획이 없어 막았다 (Edit)')
  expect(denied(r)).toContain('select:mcp__effort-steps__plan')
  expect(denied(await $.tool.call({ tool: 'Write', file_path: 'C:/repo/b.ts', content: 'x' } as never))).toContain('(Write)')
  expect(denied(await $.tool.call({ tool: 'NotebookEdit', notebook_path: 'C:/repo/n.ipynb', new_source: 'x' } as never))).toContain('(NotebookEdit)')
  expect(denied(await agent($))).toContain('(Agent)')
  const sp = await $.agent.spawn({ prompt: 'p', description: 'wf', model: 'opus' } as never).catch((err: unknown) => ({ deny: String(err) }))
  expect(String(sp.deny)).toContain('(Agent)')
  expect(w.ran).toEqual([])
})

test('강제 : plan 을 넣으면 지나가고, 열린 mark 가 없으면 메인 편집에 그 턴 한 번 경고', async ($, on) => {
  const w = gw(on)
  await begin($)
  await plan($)
  const r1 = await edit($, 'C:/repo/a.ts')
  expect(denied(r1)).toBe(null)
  expect(notes(r1).join('\n')).toContain('열린 mark 가 없다')
  expect(notes(await edit($, 'C:/repo/b.ts'))).toEqual([])
  // Agent 띄우기에는 열린 mark 경고를 안 붙인다
  await say($, '다음')
  expect(notes(await agent($))).toEqual([])
  // mark 를 열면 경고가 없다
  w.bash.text = 'mark m1007-aaaa 시작 : 1-조사 (2026-10-07 12:00:00)'
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  expect(notes(await edit($, 'C:/repo/c.ts'))).toEqual([])
  expect(w.ran).toEqual(['Edit C:/repo/a.ts', 'Edit C:/repo/b.ts', 'Agent d', 'Edit C:/repo/c.ts'])
})

test('강제 : /steps allow 는 사람이 친 것만, 다음 사람 말 한 번을 살아남고 그 다음 말에 지워진다', async ($, on) => {
  const w = gw(on)
  await begin($)
  expect(await steps($, 'allow', 'sdk')).toContain('사람이 터미널에서 친 것만')
  // 막혀 턴이 끝난다 → 사람이 /steps allow → 「계속」
  await say($, '고쳐 줘')
  expect(denied(await edit($, 'C:/repo/a.ts'))).not.toBe(null)
  expect(await steps($, 'allow')).toContain('다음 사람 말 한 번')
  expect(await steps($, 'status')).toContain('허용 중 : 예')
  await say($, '계속')
  expect(await steps($, 'status')).toContain('허용 중 : 예')
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
  expect(denied(await agent($))).toBe(null)
  // 슬래시 명령은 허용을 안 쓴다
  await say($, '/steps table')
  expect(denied(await edit($, 'C:/repo/b.ts'))).toBe(null)
  // 그 다음 사람 말에 다시 막는다
  await say($, '고마워')
  expect(await steps($, 'status')).toContain('허용 중 : 아니오')
  expect(denied(await edit($, 'C:/repo/c.ts'))).not.toBe(null)
  expect(w.ran).toEqual(['Edit C:/repo/a.ts', 'Agent d', 'Edit C:/repo/b.ts'])
})

test('강제 : /steps allow 를 치고 바로 쓰지 않아도 사람 말 하나 뒤에 쓰이고, 두 번째 말에 지워진다', async ($, on) => {
  gw(on)
  await begin($)
  await steps($, 'allow')
  // 사람 말 전에 막힌 턴 안에서도 허용은 먹는다
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
  await say($, '계속')
  expect(denied(await edit($, 'C:/repo/b.ts'))).toBe(null)
  await say($, '다음')
  expect(denied(await edit($, 'C:/repo/c.ts'))).not.toBe(null)
  // session.end 도 허용을 지운다
  await steps($, 'allow')
  await $.session.end({ reason: 'clear', sessionId: SID, resume: {} } as never)
  expect(denied(await edit($, 'C:/repo/d.ts'))).not.toBe(null)
})

test('강제 fail-open : 판정이 던지면 막지 않고 밑바닥(next)을 정확히 한 번 부른다', async ($, on) => {
  const w = gw(on)
  await begin($)
  // 글 아닌 경로를 넘겨 판정 안의 빼는 자리 검사(key → trim)가 던지게 한다
  // (Agent · spawn 은 경로가 없어 대역에서 판정을 던지게 할 길이 없다. 같은 failOpen 을 쓴다)
  expect(denied(await edit($, 5 as never))).toBe(null)
  expect(w.ran).toEqual(['Edit 5'])
  // 그 뒤 판정은 그대로 막는다
  expect(denied(await edit($, 'C:/repo/a.ts'))).not.toBe(null)
  expect(w.ran).toEqual(['Edit 5'])
})

test('강제 : next(밑바닥)가 던지면 편집·Agent 를 다시 돌리지 않는다', async ($, on) => {
  const w = gw(on)
  await begin($)
  await plan($)
  await edit($, `C:/repo/${THROW}.ts`).catch(() => undefined)
  expect(w.thrown).toBe(1)
  await $.tool.call({ tool: 'Agent', description: THROW, prompt: 'p', model: 'opus' } as never).catch(() => undefined)
  expect(w.thrown).toBe(2)
  // 그 뒤로도 강제 훅은 멀쩡하다
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
  expect(w.ran).toEqual(['Edit C:/repo/a.ts'])
})

test('강제 warn : 지나가되 그 턴 첫 번에만 경고 한 줄', { options: { mode: 'warn' } }, async ($, on) => {
  const w = gw(on)
  await begin($)
  const r1 = await edit($, 'C:/repo/a.ts')
  expect(denied(r1)).toBe(null)
  expect(notes(r1).join('\n')).toContain('경고만 한다 (mode warn)')
  expect(notes(await agent($))).toEqual([])
  await say($, '다음 턴')
  expect(notes(await agent($)).join('\n')).toContain('(Agent)')
  expect(w.ran.length).toBe(3)
  expect(await steps($, 'status')).toContain('mode : warn')
})

test('강제 off : 막지도 경고하지도 않는다', { options: { mode: 'off' } }, async ($, on) => {
  const w = gw(on)
  await begin($)
  const r = await edit($, 'C:/repo/a.ts')
  expect(denied(r)).toBe(null)
  expect(notes(r)).toEqual([])
  expect(denied(await agent($))).toBe(null)
  expect(w.ran.length).toBe(2)
})

test('강제 : 모르는 mode 값은 deny', { options: { mode: 'banana' } }, async ($, on) => {
  gw(on)
  await begin($)
  expect(denied(await edit($, 'C:/repo/a.ts'))).toContain('공수 계획이 없어 막았다')
})

test('강제 : 서브에이전트(agentId)의 편집·Agent 는 안 막는다', async ($, on) => {
  const w = gw(on)
  await begin($)
  expect(denied(await edit($, 'C:/repo/a.ts', 'a1'))).toBe(null)
  expect(denied(await agent($, 'a1'))).toBe(null)
  expect(w.ran).toEqual(['Edit C:/repo/a.ts', 'Agent d'])
})

test('강제 : 비대화형(claude -p)은 안 본다', async ($, on) => {
  const w = gw(on)
  await begin($, false)
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
  expect(denied(await agent($))).toBe(null)
  expect(w.ran.length).toBe(2)
})

test('강제 : 빼는 자리(~/.claude 아래 · TEMP 아래)는 plan 없이도 지나간다', async ($, on) => {
  const w = gw(on)
  await begin($)
  expect(denied(await edit($, 'C:\\home\\.claude\\projects\\x\\memory\\a.md'))).toBe(null)
  expect(denied(await edit($, `${TEMP}/claude/x/sess-1/scratchpad/t.txt`))).toBe(null)
  expect(denied(await edit($, 'C:/home/.claude/../repo/a.ts'))).not.toBe(null)
  expect(w.ran.length).toBe(2)
})

test('강제 : /clear 뒤에는 세션 id 가 바뀌어 plan 없음 → 막힌다', async ($, on) => {
  const w = gw(on)
  await begin($)
  await plan($)
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
  await steps($, 'allow')
  await $.session.end({ reason: 'clear', sessionId: SID, resume: {} } as never)
  w.sid = 'sess-2'
  expect(denied(await edit($, 'C:/repo/b.ts'))).toContain('공수 계획이 없어 막았다')
  expect(await steps($, 'status')).toContain('세션 id 다름')
})

test('강제 : 소단계가 전부 끝이면 plan 없음으로 보고 막는다', async ($, on) => {
  gw(on)
  await begin($)
  await plan($)
  for (const n of [1, 2, 3]) await $.tool.call({ tool: 'mcp__effort-steps__step', n, status: '끝' })
  expect(denied(await edit($, 'C:/repo/a.ts'))).toContain('공수 계획이 없어 막았다')
  expect(await steps($, 'status')).toContain('소단계 전부 끝')
  // 새 판 plan 을 넣으면 다시 지나간다
  await plan($)
  expect(denied(await edit($, 'C:/repo/a.ts'))).toBe(null)
})

test('서브에이전트의 plan 호출은 「메인 몫」 글로 거절하고 메인 계획을 안 덮는다', async ($, on) => {
  gw(on)
  await begin($)
  const r = await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '서브 계획', steps: STEPS, agentId: 'a1' } as never)
  expect(String(r.result)).toContain('plan 은 메인 몫이다')
  expect(String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)).toContain('계획이 없다')
  await plan($)
  await $.tool.call({ tool: 'mcp__effort-steps__plan', title: '서브 계획', steps: [STEPS[0]], agentId: 'a1' } as never)
  expect(String((await $.tool.call({ tool: 'mcp__effort-steps__table' })).result)).toContain('3. 시험')
})

test('안 닫힌 mark toast : 메인 턴 끝은 메인 mark 만, 서브 끝은 그 서브 mark 만, mark 하나에 한 번', async ($, on) => {
  const w = gw(on)
  const toasts: string[] = []
  on('ui.toast', ($, e) => {
    toasts.push(e.text)
    return { value: undefined }
  })
  await begin($)
  await plan($)
  w.bash.text = 'mark m1007-aaaa 시작 : 1-조사 (2026-10-07 12:00:00)'
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "1-조사"' })
  w.bash.text = 'mark m1007-bbbb 시작 : 2-구현 (2026-10-07 12:00:00)'
  await $.tool.call({ tool: 'Bash', command: 'effort mark start "2-구현"', agentId: 'a1' } as never)
  const done = (agentId?: string) => ({ answer: '', durationMs: 1, isAborted: false, turnId: 't', reason: 'answer', ...(agentId === undefined ? {} : { agentId }) })
  await $.turn.complete(done() as never)
  expect(toasts.length).toBe(1)
  expect(toasts[0]).toContain('1-조사 (m1007-aaaa)')
  expect(toasts[0]).not.toContain('m1007-bbbb')
  await $.turn.complete(done() as never)
  expect(toasts.length).toBe(1)
  await $.turn.complete(done('a1') as never)
  expect(toasts.length).toBe(2)
  expect(toasts[1]).toContain('2-구현 (m1007-bbbb)')
  await $.turn.complete(done('a1') as never)
  expect(toasts.length).toBe(2)
})
