// pair.ts · table.ts 시험
import { describe, expect, test } from 'claude-code/testing'

import type { MarkInfo, Plan, Step } from '../types'
import { attach, newMark, pairOf, pickFromMarksFile, settle, stepActual, stepNumber } from '../hooks/pair'
import { parseMarksFile } from '../hooks/parse'
import { driftText, markdownTable, totals } from '../hooks/table'

function step(n: number, name: string, estMs: number | null): Step {
  return {
    n, name, cls: '구현', size: 'M', estText: estMs === null ? '—' : `${Math.round(estMs / 60_000)}분`,
    estMs, markIds: [], status: '할 일', note: null,
  }
}

function plan(): Plan {
  return {
    id: 'p1', title: '타일 검사기', createdAt: 1_000_000, sessionId: 'sess-1', estNote: null,
    steps: [step(1, '조사', 300_000), step(2, '타일그림', 600_000), step(3, '시험', null)],
  }
}

function closed(id: string, name: string, n: number, text: string, ms: number): MarkInfo {
  return { ...newMark(id, name, n, null, 1_000_000), open: false, recordText: text, recordMs: ms }
}

describe('번호 짝', () => {
  test('이름 앞 번호', () => {
    expect(stepNumber('3-검사기')).toBe(3)
    expect(stepNumber('4a-리뷰')).toBe(4)
    expect(stepNumber('2A-조사')).toBe(2)
    expect(stepNumber('12')).toBe(12)
    expect(stepNumber('조사B-웹')).toBeNull()
    expect(stepNumber('canon-적용')).toBeNull()
    expect(stepNumber('2026년')).toBeNull()
  })

  test('계획에 없는 번호와 계획 전 mark 는 짝 없음', () => {
    const p = plan()
    expect(pairOf(p, '2-타일그림', 1_000_000)).toBe(2)
    expect(pairOf(p, '9-기타', 1_000_000)).toBeNull()
    // 계획보다 한참 먼저 연 mark : 번호만 같고 이름이 다르면 짝 없음
    expect(pairOf(p, '1-다른일', 1_000_000 - 120_000)).toBeNull()
    expect(pairOf(p, '1-조사', 1_000_000 - 30_000)).toBe(1)
    expect(pairOf(null, '1-조사', 1_000_000)).toBeNull()
  })

  test('계획보다 먼저 연 mark 라도 번호 뒤 이름이 소단계 이름과 같으면 짝짓는다 (빈칸·대소문자 무시)', () => {
    const p = { ...plan(), steps: [...plan().steps, step(5, 'effort-steps 패널 손질', null)] }
    expect(pairOf(p, '1-조사', 1_000_000 - 300_000)).toBe(1)
    expect(pairOf(p, '5-effort-steps 패널 손질', 1_000_000 - 300_000)).toBe(5)
    expect(pairOf(p, '5-Effort-Steps 패널손질', 1_000_000 - 300_000)).toBe(5)
    expect(pairOf(p, '5-패널', 1_000_000 - 300_000)).toBeNull()
    expect(pairOf(p, '5', 1_000_000 - 300_000)).toBeNull()
  })

  test('이름 짝에도 시각 하한 : 계획 10분 전보다 먼저 연 옛 mark 는 안 붙는다, 이전 계획이 있으면 그 시각까지 (리뷰 3)', () => {
    const p = { ...plan(), createdAt: 10_000_000 }
    // 같은 세션의 옛 작업에서 같은 이름으로 연 mark (계획 15분 전) — 안 붙는다
    expect(pairOf(p, '1-조사', 10_000_000 - 900_000)).toBeNull()
    expect(pairOf(p, '1-조사', 10_000_000 - 600_000)).toBe(1)
    // 이전 계획(1시간 전)이 있으면 그 계획 − 10분 이후는 붙는다, 그보다 먼저는 안 붙는다
    const re = { ...p, prevAt: 10_000_000 - 3_600_000 }
    expect(pairOf(re, '1-조사', 10_000_000 - 1_800_000)).toBe(1)
    expect(pairOf(re, '1-조사', 10_000_000 - 3_900_000)).toBe(1)
    expect(pairOf(re, '1-조사', 10_000_000 - 4_300_000)).toBeNull()
  })

  test('marks.txt 보조 길 : 계획 전 같은 세션 mark 도 이름이 같으면 주워 온다', () => {
    // mark 는 0시 0분 10초, 계획은 그 8분 뒤 (이름 짝 하한 10분 안)
    const p = { ...plan(), createdAt: 490_000, steps: [...plan().steps, step(5, 'effort-steps 패널 손질', null)] }
    const lines = parseMarksFile([
      'start\tm1007-1cee\t1970-01-01T00:00:10.000Z\tC--proj\tsess-1/agent-aa9f\t5-effort-steps 패널 손질',
      'stop\tm1007-1cee\t1970-01-01T00:00:20.000Z',
      'start\tm1007-0ld1\t1970-01-01T00:00:10.000Z\tC--proj\tsess-1\t2-옛 계획 일',
    ].join('\n'))
    expect(pickFromMarksFile(lines, 'sess-1', p, {}).map(l => l.id)).toEqual(['m1007-1cee'])
  })
})

describe('실제 값', () => {
  test('mark 하나면 원문 그대로', () => {
    let p = attach(plan(), 2, 'm1')
    const marks = { m1: closed('m1', '2-타일그림', 2, '14분', 840_000) }
    p = settle(p, marks)
    expect(stepActual(p.steps[1]!, marks)).toMatchObject({ text: '14분', ms: 840_000, isOpen: false })
    expect(p.steps[1]!.status).toBe('끝')
  })

  test('같은 번호 두 번이면 합', () => {
    let p = attach(attach(plan(), 2, 'm1'), 2, 'm2')
    const marks = {
      m1: closed('m1', '2a-타일', 2, '3분 (2분 54초)', 174_000),
      m2: { ...closed('m2', '2b-타일', 2, '5분 (4분 30초)', 270_000), open: true },
    }
    p = settle(p, marks)
    expect(stepActual(p.steps[1]!, marks)).toMatchObject({ text: '7분', ms: 444_000, isOpen: true })
    expect(p.steps[1]!.status).toBe('하는 중')
  })

  test('건너뜀은 mark 가 바꾸지 않는다', () => {
    const p = plan()
    p.steps[2]!.status = '건너뜀'
    expect(settle(p, {}).steps[2]!.status).toBe('건너뜀')
  })
})

test('marks.txt 보조 길은 이 세션 · 계획 뒤 · 번호 있는 것만', () => {
  const lines = [
    { id: 'a', at: 1_000_100, file: 'sess-1/agent-x', name: '1-조사', stopAt: null },
    { id: 'b', at: 1_000_100, file: 'sess-2/agent-x', name: '1-조사', stopAt: null },
    { id: 'c', at: 1_000_100, file: '-', name: '2-타일', stopAt: null },
    { id: 'd', at: 500_000, file: 'sess-1', name: '2-타일', stopAt: null },
    { id: 'e', at: 1_000_100, file: 'sess-1', name: '검토', stopAt: null },
    { id: 'f', at: 1_000_100, file: 'sess-1', name: '3-시험', stopAt: 1_200_000 },
  ]
  expect(pickFromMarksFile(lines, 'sess-1', plan(), {}).map(l => l.id)).toEqual(['a', 'f'])
  expect(pickFromMarksFile(lines, 'sess-1', plan(), { a: newMark('a', '1-조사', 1, null, 0) }).map(l => l.id)).toEqual(['f'])
})

describe('표', () => {
  test('합계와 — 와 어긋남 까닭', () => {
    let p = attach(attach(plan(), 1, 'm1'), 2, 'm2')
    const marks = {
      m1: closed('m1', '1-조사', 1, '5분', 300_000),
      m2: closed('m2', '2-타일그림', 2, '20분', 1_200_000),
    }
    p = settle(p, marks)
    const t = totals(p, marks)
    expect(t).toMatchObject({ estMs: 900_000, actualMs: 1_500_000, done: 2, hasOpen: false })
    const md = markdownTable(p, marks, 1.5)
    expect(md).toContain('| 소단계 | 예상 | 실제 | 상태 | 까닭 |')
    expect(md).toContain('| 2. 타일그림 | 10분 | 20분 | 끝 | 예상의 2.0배 |')
    expect(md).toContain('| 3. 시험 | — | — | 할 일 |  |')
    expect(md).toContain('| 합계 | 15분 | 25분 | 2/3 끝 |  |')
    expect(md).toContain('mem add --type history')
  })

  test('어긋남이 없으면 네 칸', () => {
    const md = markdownTable(plan(), {}, 1.5)
    expect(md).toContain('| 소단계 | 예상 | 실제 | 상태 |\n')
    expect(md).toContain('| 합계 | 15분 | — | 0/3 끝 |')
  })

  test('어긋남 문턱', () => {
    expect(driftText(1.4, 1.5)).toBeNull()
    expect(driftText(1.6, 1.5)).toBe('예상의 1.6배')
    expect(driftText(0.5, 1.5)).toBe('예상의 50%')
    expect(driftText(null, 1.5)).toBeNull()
  })
})
