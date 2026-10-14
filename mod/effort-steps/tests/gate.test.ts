// gate.ts 시험 : 판정 표 전부와 글 내용.
import { describe, expect, test } from 'claude-code/testing'

import type { MarkInfo, Step, StepStatus } from '../types'
import {
  PLAN_TOOL, allDone, denyText, hasOpenMark, isSkipPath, judgeGate, noMarkLine, planLive, readMode, statusText,
  subPlanText, unclosedMarks, unclosedToast, warnLine,
} from '../hooks/gate'
import type { GateInput } from '../hooks/gate'
import { newMark } from '../hooks/pair'

function step(n: number, status: StepStatus): Step {
  return { n, name: `s${n}`, cls: '구현', size: 'M', estText: '—', estMs: null, markIds: [], status, note: null }
}

const LIVE = { sessionId: 'sess-1', steps: [step(1, '끝'), step(2, '하는 중')] }
const DONE = { sessionId: 'sess-1', steps: [step(1, '끝'), step(2, '끝')] }

/** 기본 : 대화형 · deny · 메인 · 빼는 자리 아님 · plan 없음 · allow 없음 → deny */
function input(over: Partial<GateInput> = {}): GateInput {
  return { interactive: true, mode: 'deny', agentId: undefined, skip: false, plan: null, sessionId: 'sess-1', allowed: false, ...over }
}

describe('judgeGate 판정 표', () => {
  test('기본은 deny', () => {
    expect(judgeGate(input())).toBe('deny')
  })
  test('비대화형은 지나감', () => {
    expect(judgeGate(input({ interactive: false }))).toBe('pass')
  })
  test('mode off 는 지나감', () => {
    expect(judgeGate(input({ mode: 'off' }))).toBe('pass')
  })
  test('agentId 있음(서브)은 지나감', () => {
    expect(judgeGate(input({ agentId: 'a1' }))).toBe('pass')
  })
  test('빼는 자리는 지나감', () => {
    expect(judgeGate(input({ skip: true }))).toBe('pass')
  })
  test('plan 있음(같은 세션 id)은 planned', () => {
    expect(judgeGate(input({ plan: LIVE }))).toBe('planned')
  })
  test('plan 의 세션 id 가 다르면 없음과 같다 (/clear 뒤)', () => {
    expect(judgeGate(input({ plan: LIVE, sessionId: 'sess-2' }))).toBe('deny')
  })
  test('소단계가 전부 끝이면 없음과 같다', () => {
    expect(judgeGate(input({ plan: DONE }))).toBe('deny')
    expect(judgeGate(input({ plan: DONE, mode: 'warn' }))).toBe('warn')
  })
  test('/steps allow 는 지나감', () => {
    expect(judgeGate(input({ allowed: true }))).toBe('allowed')
  })
  test('mode warn 은 warn', () => {
    expect(judgeGate(input({ mode: 'warn' }))).toBe('warn')
  })
  test('차례 : 앞 칸이 이긴다', () => {
    // 비대화형이면 plan·allow 를 안 본다
    expect(judgeGate(input({ interactive: false, plan: LIVE, allowed: true }))).toBe('pass')
    // plan 있으면 allow 보다 앞선다
    expect(judgeGate(input({ plan: LIVE, allowed: true }))).toBe('planned')
    // allow 는 warn 보다 앞선다
    expect(judgeGate(input({ mode: 'warn', allowed: true }))).toBe('allowed')
  })
})

describe('plan 있음 · 전부 끝', () => {
  test('allDone', () => {
    expect(allDone(DONE)).toBe(true)
    expect(allDone(LIVE)).toBe(false)
    expect(allDone({ steps: [step(1, '끝'), step(2, '건너뜀')] })).toBe(true)
    expect(allDone({ steps: [step(1, '건너뜀'), step(2, '하는 중')] })).toBe(false)
    expect(allDone({ steps: [] })).toBe(false)
  })
  test('planLive', () => {
    expect(planLive(null, 'sess-1')).toBe(false)
    expect(planLive(LIVE, 'sess-1')).toBe(true)
    expect(planLive(LIVE, 'other')).toBe(false)
    expect(planLive(DONE, 'sess-1')).toBe(false)
  })
  test('끝+건너뜀 섞임 → 없음', () => {
    const mixed = { sessionId: 'sess-1', steps: [step(1, '끝'), step(2, '건너뜀')] }
    expect(planLive(mixed, 'sess-1')).toBe(false)
    expect(judgeGate(input({ plan: mixed }))).toBe('deny')
  })
})

describe('readMode', () => {
  test('deny · warn · off, 모르는 값은 deny', () => {
    expect(readMode('warn')).toBe('warn')
    expect(readMode('off')).toBe('off')
    expect(readMode('deny')).toBe('deny')
    expect(readMode('WARN')).toBe('deny')
    expect(readMode(undefined)).toBe('deny')
    expect(readMode(3)).toBe('deny')
  })
})

describe('isSkipPath', () => {
  const HOME = 'C:\\Users\\me'
  const TEMP = 'C:\\Users\\me\\AppData\\Local\\Temp'
  test('~/.claude 아래는 뺀다 (내장 기억 · 계획 파일)', () => {
    expect(isSkipPath('C:\\Users\\me\\.claude\\projects\\x\\memory\\a.md', HOME, TEMP)).toBe(true)
    expect(isSkipPath('c:/users/ME/.claude/plans/p.md', HOME, TEMP)).toBe(true)
    expect(isSkipPath('/c/Users/me/.claude/x.md', HOME, TEMP)).toBe(true)
  })
  test('TEMP 아래는 뺀다 (스크래치패드)', () => {
    expect(isSkipPath(`${TEMP}\\claude\\proj\\s1\\scratchpad\\a.txt`, HOME, TEMP)).toBe(true)
  })
  test('저장소 · 홈의 다른 자리는 안 뺀다', () => {
    expect(isSkipPath('C:\\repo\\a.ts', HOME, TEMP)).toBe(false)
    expect(isSkipPath('C:\\Users\\me\\.claudex\\a.ts', HOME, TEMP)).toBe(false)
    expect(isSkipPath('C:\\Users\\me\\Documents\\a.ts', HOME, TEMP)).toBe(false)
  })
  test('.. 조각 · 상대 경로 · ~ 는 안 뺀다', () => {
    expect(isSkipPath('C:\\Users\\me\\.claude\\..\\repo\\a.ts', HOME, TEMP)).toBe(false)
    expect(isSkipPath('.claude/a.md', HOME, TEMP)).toBe(false)
    expect(isSkipPath('~/.claude/a.md', HOME, TEMP)).toBe(false)
  })
  test('홈·TEMP 를 모르면 안 뺀다', () => {
    expect(isSkipPath('C:\\Users\\me\\.claude\\a.md', null, null)).toBe(false)
    expect(isSkipPath('C:\\Users\\me\\.claude\\a.md', '', '')).toBe(false)
  })
})

describe('글', () => {
  test('denyText 는 설계 3절 글 그대로, 도구 이름이 들어간다', () => {
    expect(denyText('Edit')).toBe([
      'effort-steps : 이 세션에 공수 계획이 없어 막았다 (Edit).',
      '먼저 plan 도구로 소단계를 넣는다 — 도구 이름 mcp__effort-steps__plan,',
      '목록에 이름만 있으면 ToolSearch 로 "select:mcp__effort-steps__plan" 을 먼저 불러온다.',
      '질문에 답만 하는 턴이면 편집·Agent 를 쓰지 않는다.',
      '사람이 다음 말 한 번만 풀려면 /steps allow, 늘 경고로만 두려면 /config 의 mode.',
    ].join('\n'))
    expect(denyText('Agent')).toContain('(Agent)')
  })
  test('warnLine · noMarkLine · subPlanText', () => {
    expect(warnLine('Write')).toContain('(Write)')
    expect(warnLine('Write')).toContain(PLAN_TOOL)
    expect(warnLine('Write')).toContain('경고만')
    expect(noMarkLine()).toContain('effort mark start')
    expect(subPlanText()).toContain('plan 은 메인 몫이다')
  })
  test('unclosedToast 는 이름과 id 를 담는다', () => {
    const t = unclosedToast([{ id: 'm1014-aaaa', name: '1-조사' }, { id: 'm1014-bbbb', name: '2-구현' }])
    expect(t).toContain('1-조사 (m1014-aaaa)')
    expect(t).toContain('2-구현 (m1014-bbbb)')
    expect(t).toContain('effort mark stop')
  })
  test('statusText', () => {
    const t = statusText({ mode: 'warn', interactive: true, hasPlan: true, planSessionOk: true, allDone: false, allowed: true, warned: true, noMarkWarned: false })
    expect(t).toContain('mode : warn')
    expect(t).toContain('plan : 있음 (세션 id 같음)')
    expect(t).toContain('허용 중 : 예')
    expect(t).toContain('계획 없음 1 · 열린 mark 없음 0')
    const u = statusText({ mode: 'deny', interactive: false, hasPlan: true, planSessionOk: false, allDone: false, allowed: false, warned: false, noMarkWarned: false })
    expect(u).toContain('아니오 (강제 안 함)')
    expect(u).toContain('세션 id 다름')
  })
})

describe('안 닫힌 mark', () => {
  function mark(id: string, agent: string | null, open: boolean, at: number): MarkInfo {
    return { ...newMark(id, `${id}-이름`, null, agent, at), open }
  }
  const marks: Record<string, MarkInfo> = {
    'm1014-0001': mark('m1014-0001', null, true, 1),
    'm1014-0002': mark('m1014-0002', null, false, 2),
    'm1014-0003': mark('m1014-0003', 'a1', true, 3),
    'm1014-0004': mark('m1014-0004', 'agent-a2', true, 4),
  }
  test('메인은 메인이 연 열린 것만', () => {
    expect(unclosedMarks(marks, null, []).map(m => m.id)).toEqual(['m1014-0001'])
  })
  test('서브는 그 서브가 연 것만 (agent-<id> 꼴도)', () => {
    expect(unclosedMarks(marks, 'a1', []).map(m => m.id)).toEqual(['m1014-0003'])
    expect(unclosedMarks(marks, 'a2', []).map(m => m.id)).toEqual(['m1014-0004'])
  })
  test('이미 알린 것은 뺀다', () => {
    expect(unclosedMarks(marks, null, ['m1014-0001'])).toEqual([])
  })
  test('hasOpenMark', () => {
    expect(hasOpenMark(marks)).toBe(true)
    expect(hasOpenMark({ 'm1014-0002': marks['m1014-0002']! })).toBe(false)
    expect(hasOpenMark({})).toBe(false)
  })
})
