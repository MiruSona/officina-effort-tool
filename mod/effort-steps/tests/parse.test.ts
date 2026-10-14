// parse.ts 시험 : 실제 exe 출력 표본으로 본다.
import { describe, expect, test } from 'claude-code/testing'

import {
  clean, minutesText, parseBlocks, parseCommand, parseEstimate, parseMarksFile, textToMs,
} from '../hooks/parse'
import type { ShowBlock } from '../hooks/parse'
import {
  estimateJson, showMissing, showOpen, showStopped, showUnder1, showWait, start, stopThenStart,
} from './fixtures'

function onlyShow(text: string): ShowBlock {
  const blocks = parseBlocks(text)
  expect(blocks).toHaveLength(1)
  const b = blocks[0]!
  if (b.kind !== 'show') throw new Error('show 덩이가 아니다')
  return b
}

describe('mark show 출력', () => {
  test('닫힌 mark', () => {
    const b = onlyShow(showStopped)
    expect(b.id).toBe('m1007-473f')
    expect(b.name).toBe('3-조사문서합치기')
    expect(b.open).toBe(false)
    expect(b.recordText).toBe('3분 (2분 54초)')
    expect(b.recordMs).toBe(174_000)
    expect(b.waitText).toBe('없음')
    expect(b.failReason).toBeNull()
  })

  test('열린 mark 는 open', () => {
    const b = onlyShow(showOpen)
    expect(b.open).toBe(true)
    expect(b.flags).toEqual(['진행중'])
    expect(b.recordText).toBe('5분 (4분 30초)')
  })

  test('대기와 주의 줄', () => {
    const b = onlyShow(showWait)
    expect(b.recordText).toBe('15분')
    expect(b.recordMs).toBe(900_000)
    expect(b.waitText).toBe('5분')
    expect(b.warn).toStartWith('이 기록 파일에 mark 밖 본줄이')
  })

  test('1분 미만', () => {
    const b = onlyShow(showUnder1)
    expect(b.recordText).toBe('1분 미만 (38초)')
    expect(b.recordMs).toBe(38_000)
  })

  test('없는 mark 는 덩이 없음', () => {
    expect(parseBlocks(showMissing)).toHaveLength(0)
  })

  test('못 잰 mark', () => {
    const text = [
      'mark m1007-aaaa · 2-타일그림',
      '시작 : 2026-10-07 12:00:00 +09:00',
      '끝   : 2026-10-07 12:05:00',
      '기록 구간 : —   ← 공수 표 「실제」 칸에 쓰는 값',
      '대기 : —   (도구 한 번이 3분 넘게 걸린 틈의 합 · 기록 구간에 든 채로 보여 주기만)',
      '표시 : 묶기없음',
      '기록 구간을 못 쟀습니다 — 까닭 : mark start 를 부른 기록 파일을 최근 10분 안에서 못 찾음. 「실제」 칸에는 지어내지 말고 「못 쟀다」고 적습니다.',
    ].join('\r\n')
    const b = onlyShow(text)
    expect(b.recordText).toBe('—')
    expect(b.recordMs).toBeNull()
    expect(b.flags).toEqual(['묶기없음'])
    expect(b.failReason).toStartWith('mark start 를 부른')
  })
})

describe('mark start 출력', () => {
  test('id 와 이름', () => {
    expect(parseBlocks(start)).toEqual([{ kind: 'start', id: 'm1007-7485', name: '1-표본뜨기', warn: null }])
  })

  test('stop 뒤 start 한 번에', () => {
    const blocks = parseBlocks(stopThenStart)
    expect(blocks.map(b => `${b.kind}:${b.id}`)).toEqual(['show:m1007-7485', 'start:m1007-b3cc'])
    const stop = blocks[0] as ShowBlock
    expect(stop.open).toBe(false)
    expect(stop.recordText).toBe('3분 (3분 1초)')
  })

  test('묶기없음 주의', () => {
    const text = 'mark m1007-bbbb 시작 : 4-시험 (2026-10-07 12:00:00)\n주의 : 묶기없음 — 최근 10분 안에 바뀐 기록 파일에서 이 mark start 를 못 찾았습니다. 찍은 구간만 잽니다.\n끝낼 때 : effort mark stop m1007-bbbb'
    const b = parseBlocks(text)[0]!
    expect(b.kind).toBe('start')
    expect(b.warn).toStartWith('묶기없음')
  })
})

describe('시간 글', () => {
  test('minutesText 는 effort 와 같은 반올림', () => {
    expect(minutesText(null)).toBe('—')
    expect(minutesText(38_000)).toBe('1분 미만')
    expect(minutesText(89_000)).toBe('1분')
    expect(minutesText(90_000)).toBe('2분')
    expect(minutesText(3_600_000)).toBe('1시간')
    expect(minutesText(4_440_000)).toBe('1시간 14분')
  })

  test('textToMs', () => {
    expect(textToMs('1시간 14분')).toBe(4_440_000)
    expect(textToMs('1분 미만')).toBe(30_000)
    expect(textToMs('—')).toBeNull()
    expect(textToMs('1분 (59초)')).toBe(59_000)
  })
})

describe('estimate --json', () => {
  test('번호 → 중앙값', () => {
    const m = parseEstimate(estimateJson)
    expect(m.get('1')).toBe(226_961)
    expect(m.get('3')).toBe(260_276)
    expect(m.size).toBe(3)
  })

  test('JSON 이 아니면 빈 값', () => {
    expect(parseEstimate('캐시가 없습니다').size).toBe(0)
  })
})

describe('명령 글', () => {
  test('경로 붙은 exe 와 따옴표 이름', () => {
    expect(parseCommand('.\\EffortTool\\bin\\effort.exe mark start "2-타일그림"')).toEqual([{ verb: 'start', arg: '2-타일그림' }])
  })

  test('stop; start 한 줄', () => {
    const calls = parseCommand('cd C:/x; .\\EffortTool\\bin\\effort.exe mark stop m1007-b3cc; .\\EffortTool\\bin\\effort.exe mark start "3-parse"')
    expect(calls).toEqual([{ verb: 'stop', arg: 'm1007-b3cc' }, { verb: 'start', arg: '3-parse' }])
  })

  test('effort 낱말이 없으면 안 본다', () => {
    expect(parseCommand('git mark start foo')).toEqual([])
  })

  test('따옴표 없는 이름', () => {
    expect(parseCommand('effort mark start 4-시험 && echo ok')).toEqual([{ verb: 'start', arg: '4-시험' }])
  })
})

describe('marks.txt', () => {
  test('start 와 stop 짝', () => {
    const text = [
      '# effort marks — effort mark 가 덧붙인다. 손으로 고치지 않는다.',
      'start\tm1007-0001\t2026-10-07T03:00:00.000Z\tC--proj\tsess-1/agent-a1\t1-조사',
      'start\tm1007-0002\t2026-10-07T03:01:00.000Z\t-\t-\t2-구현\t묶기없음',
      'stop\tm1007-0001\t2026-10-07T03:05:00.000Z',
      'future\tsomething',
    ].join('\n')
    const lines = parseMarksFile(text)
    expect(lines).toHaveLength(2)
    expect(lines[0]).toMatchObject({ id: 'm1007-0001', file: 'sess-1/agent-a1', name: '1-조사' })
    expect(lines[0]!.stopAt).toBe(Date.parse('2026-10-07T03:05:00.000Z'))
    expect(lines[1]!.stopAt).toBeNull()
  })
})

test('clean 은 제어 글자와 길이를 막는다', () => {
  expect(clean('a\tb\nc\u0007d', 100)).toBe('a b c d')
  expect(clean('가'.repeat(100), 10)).toHaveLength(10)
  expect(clean(undefined, 5)).toBe('')
})
