// 시험용 엔진 밑바닥 대역과 도우미.
import { expect, mock } from 'claude-code/testing'
import type { On, RenderElement } from 'claude-code'

import { estimateJson } from './fixtures'

export const EXE = 'C:/repo/EffortTool/bin/effort.exe'
export const SID = 'sess-1'

/** 그 mark 의 show 출력 (열림·닫힘) */
export function showText(id: string, name: string, rec: string, open: boolean): string {
  return [
    `mark ${id} · ${name}`,
    '시작 : 2026-10-07 12:00:00 +09:00',
    `끝   : 2026-10-07 12:04:00${open ? ' (지금 — 진행중)' : ''}`,
    `묶은 기록 : C--proj/${SID}/agent-a1`,
    `기록 구간 : ${rec}   ← 공수 표 「실제」 칸에 쓰는 값`,
    `찍은 구간 : ${rec}   (exe 시계 · 참고)`,
    '대기 : 없음   (도구 한 번이 3분 넘게 걸린 틈의 합 · 기록 구간에 든 채로 보여 주기만)',
    ...(open ? ['표시 : 진행중'] : []),
  ].join('\n')
}

export type World = {
  argvs: string[][]
  shows: Map<string, string>
  marksTxt: string
  bash: { text: string }
  /** 이 시험의 store (넣은 차례대로 keys 가 나온다) */
  store: Map<string, unknown>
  /** 지금 세션 id. /clear 시험이 바꾼다 */
  sid: string
  /** 실제로 돈 편집·Agent 도구 : 'Edit C:/x' · 'Agent d' 꼴 */
  ran: string[]
  /** 밑바닥이 던진 횟수 (경로·설명에 THROW 가 든 편집·Agent 는 밑바닥이 던진다) */
  thrown: number
}

/** 이 글이 경로·설명에 들면 밑바닥 편집·Agent 가 던진다 (next 가 던질 때 두 번 안 도는지 보는 시험용) */
export const THROW = 'THROW'

export const HOME = 'C:/home'
export const TEMP = 'C:/home/AppData/Local/Temp'

/**
 * process.run 앞에 끼우는 시험용 손잡이 : 글을 돌려주면 그 글로 막고(deny),
 * Promise 를 돌려주면 그것이 풀릴 때까지 붙잡는다. undefined 면 그대로 간다.
 */
export type BeforeRun = (argv: readonly string[]) => string | Promise<void> | undefined

/** 엔진 밑바닥 대역 : 세션·fs·process·도구 등록을 메모리로 답한다. */
export function world(on: On, store?: Record<string, unknown>, opts: { belowText?: string; beforeRun?: BeforeRun } = {}): World {
  // 띠는 기본 숨김이다. 띠 그림을 보는 시험이 많아 대역은 「/steps show 로 켠 사람」 의 prefs 를 먼저 넣어 둔다.
  // 기본 숨김을 보는 시험은 w.store.delete('prefs') 로 지운다.
  const w: World = {
    argvs: [], shows: new Map(), marksTxt: '', bash: { text: '' },
    store: new Map(Object.entries({ prefs: { bandHidden: false }, ...(store ?? {}) })),
    sid: SID, ran: [], thrown: 0,
  }
  // mock.store 대신 메모리 Map 으로 답한다 (시험이 store 를 들여다볼 수 있게)
  on('store.get', ($, e) => ({ value: w.store.get(e.key) }))
  on('store.set', ($, e) => {
    w.store.set(e.key, e.value)
    return { value: undefined }
  })
  on('store.delete', ($, e) => {
    w.store.delete(e.key)
    return { value: undefined }
  })
  on('store.keys', () => ({ value: [...w.store.keys()] }))
  mock.env(on, { USERPROFILE: HOME, TEMP, PATH: '' })
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('session.id', () => ({ value: w.sid }))
  // 강제 시험용 : 편집·Agent 도구와 띄우기, 턴 끝
  const answer = (line: string) => {
    if (line.includes(THROW)) {
      w.thrown++
      throw new Error('밑바닥이 던졌다')
    }
    w.ran.push(line)
    return { result: {} as never, text: '' }
  }
  on('tool.call', { tool: 'Edit' }, ($, e) => answer(`Edit ${e.file_path}`))
  on('tool.call', { tool: 'Write' }, ($, e) => answer(`Write ${e.file_path}`))
  on('tool.call', { tool: 'NotebookEdit' }, ($, e) => answer(`NotebookEdit ${e.notebook_path}`))
  on('tool.call', { tool: 'Agent' }, ($, e) => answer(`Agent ${e.description}`))
  on('agent.spawn', ($, e) => {
    w.ran.push(`spawn ${e.description}`)
    return { model: e.model ?? 'inherit', agentId: `s${w.ran.length}` }
  })
  on('turn.complete', ($, e) => ({ text: e.answer }))
  on('session.end', ($, e) => ({ sessionId: e.sessionId }))
  on('prompt.submit', ($, e) => ({ text: e.text }))
  on('session.cwd', () => ({ value: 'C:/repo' }))
  on('session.root', () => ({ value: 'C:/repo' }))
  on('tool.register', ($, e) => ({ value: { tool: `mcp__effort-steps__${e.name}` } }))
  on('command.register', ($, e) => ({ value: { command: e.name } }))
  // 엔진은 경로를 OS 꼴(역슬래시)로 바꿔 넘긴다
  on('fs.exists', ($, e) => ({ value: e.path.split('\\').join('/') === EXE }))
  on('fs.read', () => ({ value: w.marksTxt }))
  on('process.run', async ($, e) => {
    const argv = [...e.argv]
    const held = opts.beforeRun?.(argv)
    if (typeof held === 'string') return { deny: held }
    if (held !== undefined) await held
    w.argvs.push(argv)
    let stdout = ''
    if (argv[1] === 'estimate') stdout = estimateJson
    else if (argv[1] === 'mark' && argv[2] === 'show') stdout = w.shows.get(argv[3] ?? '') ?? ''
    return { value: { exitCode: stdout === '' ? 2 : 0, stdout, stderr: stdout === '' ? '그런 mark 가 없습니다' : '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  // 띠를 비킬 때 엔진이 그리는 자리 (엔진 자기 그림 { type: 'engine', ref: 0 })
  on('ui.render', { component: 'AbovePrompt' }, ($, e) => {
    // belowText 가 있으면 아래 플러그인(prompt-gauge 같은)의 띠 노릇을 한다
    if (opts.belowText !== undefined) {
      const { Text } = $.ui.resolve(e)
      return h(Text, { key: 'gauge' }, opts.belowText) as RenderElement
    }
    return { type: 'engine', ref: 0 } as RenderElement
  })
  on('tool.call', { tool: 'Bash' }, () => ({
    result: { stdout: w.bash.text, stderr: '', interrupted: false },
    text: w.bash.text,
  }))
  return w
}

/** 금지 인자 단언 : mod 는 mark start · stop · scan 을 절대 부르지 않는다. */
export function expectReadOnly(argvs: string[][]): void {
  for (const argv of argvs) {
    const args = argv.slice(1)
    expect(args.includes('start')).toBe(false)
    expect(args.includes('stop')).toBe(false)
    expect(args.includes('scan')).toBe(false)
    expect(['estimate', 'mark']).toContain(args[0])
    if (args[0] === 'mark') expect(args[1]).toBe('show')
  }
}

export const STEPS = [
  { n: 1, name: '조사', class: '조사', size: 'M' },
  { n: 2, name: '구현', class: '구현', size: 'S' },
  { n: 3, name: '시험', class: '구현', size: 'M' },
]

