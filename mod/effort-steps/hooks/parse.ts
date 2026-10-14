// effort.exe 출력 파싱. 순수 함수만 둔다 ($ 를 받지 않는다).
// 출력 꼴의 근거 : EffortTool/cmd/effort/cmd_mark.go (start 127행, printMarkValue 746행),
// internal/render/dur.go (Minutes), internal/store/marks.go (startLine).

const MARK_ID = 'm\\d{4}-[0-9a-f]{4,8}'
const SHOW_HEAD = new RegExp(`^mark (${MARK_ID}) · (.+)$`)
const START_HEAD = new RegExp(`^mark (${MARK_ID}) 시작 : (.+) \\(\\d{4}-\\d\\d-\\d\\d \\d\\d:\\d\\d:\\d\\d\\)$`)
const RECORD = /^기록 구간\s*:\s*(.+?)(?:\s{2,}←.*)?$/
const WAIT = /^대기\s*:\s*(.+?)(?:\s{2,}\(.*)?$/
const FLAGS = /^표시\s*:\s*(.+)$/
const END = /^끝\s*:\s*(.+)$/
const WARN = /^주의\s*:\s*(.+)$/
const STARTED = /^시작\s*:\s*(\d{4}-\d\d-\d\d) (\d\d:\d\d:\d\d) ([+-]\d\d:\d\d)$/
/** mark 이름 길이 상한 (바깥 출력·파일에서 온 글이라 다듬어 둔다) */
const MARK_NAME_MAX = 80
const FAILED = /^기록 구간을 못 쟀습니다(?: — 까닭 : (.+?))?\./

/** mark start 출력 한 덩이 */
export type StartBlock = { kind: 'start'; id: string; name: string; warn: string | null }

/** mark show·stop 출력 한 덩이 (둘은 같은 꼴이다) */
export type ShowBlock = {
  kind: 'show'
  id: string
  name: string
  /** 시작 줄의 시각 (ms). 못 읽으면 null */
  startAt: number | null
  /** 끝 줄이 「(지금 — 진행중)」 이거나 표시에 진행중이 있으면 true */
  open: boolean
  recordText: string | null
  recordMs: number | null
  waitText: string | null
  flags: string[]
  warn: string | null
  /** 「기록 구간을 못 쟀습니다」 줄이 있으면 그 까닭 (없으면 빈 글) */
  failReason: string | null
}

export type Block = StartBlock | ShowBlock

/** 밀리초를 effort 와 같은 글로 바꾼다 (render.Minutes 와 같은 반올림). */
export function minutesText(ms: number | null): string {
  if (ms === null || ms <= 0) return '—'
  if (ms < 60_000) return '1분 미만'
  const min = Math.floor((ms + 30_000) / 60_000)
  if (min < 60) return `${min}분`
  const h = Math.floor(min / 60)
  const m = min % 60
  return m === 0 ? `${h}시간` : `${h}시간 ${m}분`
}

/**
 * 「3분 (2분 54초)」 같은 구간 글을 밀리초로 바꾼다. 합계용이다.
 * 괄호 안 분·초가 있으면 그것을 쓰고, 「1분 미만」 은 30초, 「—」 는 null.
 */
export function textToMs(text: string | null): number | null {
  if (text === null) return null
  const t = text.trim()
  if (t === '' || t.startsWith('—')) return null
  const paren = /\((?:(\d+)분\s*)?(?:(\d+)초)?\)/.exec(t)
  if (paren && (paren[1] !== undefined || paren[2] !== undefined)) {
    return (Number(paren[1] ?? 0) * 60 + Number(paren[2] ?? 0)) * 1000
  }
  if (t.startsWith('1분 미만')) return 30_000
  const h = /(\d+)시간/.exec(t)
  const m = /(\d+)분/.exec(t)
  if (!h && !m) return null
  return (Number(h?.[1] ?? 0) * 60 + Number(m?.[1] ?? 0)) * 60_000
}

/** 출력 전체를 덩이로 나눈다. stop 뒤 start 를 한 번에 친 출력도 차례대로 나온다. */
export function parseBlocks(text: string): Block[] {
  const blocks: Block[] = []
  let cur: Block | null = null
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (line === '') continue
    const start = START_HEAD.exec(line)
    if (start) {
      cur = { kind: 'start', id: start[1]!, name: clean(start[2], MARK_NAME_MAX), warn: null }
      blocks.push(cur)
      continue
    }
    const head = SHOW_HEAD.exec(line)
    if (head) {
      cur = {
        kind: 'show', id: head[1]!, name: clean(head[2], MARK_NAME_MAX), startAt: null, open: false,
        recordText: null, recordMs: null, waitText: null, flags: [], warn: null, failReason: null,
      }
      blocks.push(cur)
      continue
    }
    if (cur === null) continue
    const warn = WARN.exec(line)
    if (warn) {
      cur.warn = cur.warn ?? warn[1]!
      continue
    }
    if (cur.kind !== 'show') continue
    let m: RegExpExecArray | null
    if ((m = RECORD.exec(line))) {
      cur.recordText = m[1]!
      cur.recordMs = textToMs(m[1]!)
    } else if ((m = STARTED.exec(line))) {
      const at = Date.parse(`${m[1]}T${m[2]}${m[3]}`)
      cur.startAt = Number.isNaN(at) ? null : at
    } else if ((m = WAIT.exec(line))) {
      cur.waitText = m[1]!
    } else if ((m = FLAGS.exec(line))) {
      cur.flags = m[1]!.split('·').map(s => s.trim()).filter(s => s !== '')
      if (cur.flags.includes('진행중')) cur.open = true
    } else if ((m = END.exec(line))) {
      if (m[1]!.includes('진행중')) cur.open = true
    } else if ((m = FAILED.exec(line))) {
      cur.failReason = m[1] ?? ''
    }
  }
  return blocks
}

/** estimate --json 결과에서 소단계 이름(번호) → 중앙값 ms. 꼴이 틀리면 빈 Map. */
export function parseEstimate(json: string): Map<string, number> {
  const out = new Map<string, number>()
  let data: unknown
  try {
    data = JSON.parse(json)
  } catch {
    return out
  }
  if (!Array.isArray(data)) return out
  for (const row of data) {
    if (row === null || typeof row !== 'object') continue
    const r = row as Record<string, unknown>
    if (typeof r.Name === 'string' && typeof r.P50Ms === 'number' && r.P50Ms > 0) {
      out.set(r.Name, r.P50Ms)
    }
  }
  return out
}

/** 명령 글 속 effort mark 호출 하나 */
export type MarkCall = { verb: 'start' | 'stop' | 'show'; arg: string | null }

/**
 * Bash·PowerShell 명령 글에서 effort mark 호출을 차례대로 뽑는다.
 * 경로 붙은 exe(.\EffortTool\bin\effort.exe)도 잡는다. effort 낱말이 없으면 빈 목록.
 */
export function parseCommand(command: string): MarkCall[] {
  if (!/effort/i.test(command)) return []
  const out: MarkCall[] = []
  const re = /\bmark\s+(start|stop|show)\b((?:\s+--\S+(?:\s+[^\s"'-]\S*)?)*)(?:\s+(?:"([^"\r\n]*)"|'([^'\r\n]*)'|([^\s"';|&]+)))?/g
  let m: RegExpExecArray | null
  while ((m = re.exec(command)) !== null) {
    const arg = m[3] ?? m[4] ?? m[5] ?? null
    out.push({ verb: m[1] as MarkCall['verb'], arg: arg === null ? null : arg.trim() })
  }
  return out
}

/** marks.txt 의 start 줄 하나 (stop 이 있으면 stopAt 채움) */
export type MarkLine = { id: string; at: number; file: string; name: string; stopAt: number | null }

/**
 * ~/.effort/marks.txt 를 읽는다. 줄 꼴 : start\tid\t시각\tslug\tfile\tname[\tbind] · stop\tid\t시각.
 * 모르는 줄은 건너뛴다 (EffortTool 도 그렇게 한다).
 */
export function parseMarksFile(text: string): MarkLine[] {
  const byId = new Map<string, MarkLine>()
  for (const raw of text.split(/\r?\n/)) {
    if (raw.startsWith('#') || raw.trim() === '') continue
    const f = raw.split('\t')
    if (f[0] === 'start' && f.length >= 6) {
      const at = Date.parse(f[2]!)
      if (Number.isNaN(at)) continue
      byId.set(f[1]!, { id: f[1]!, at, file: f[4]!, name: clean(f[5], MARK_NAME_MAX), stopAt: null })
    } else if (f[0] === 'stop' && f.length >= 3) {
      const one = byId.get(f[1]!)
      const at = Date.parse(f[2]!)
      if (one && !Number.isNaN(at)) one.stopAt = at
    }
  }
  return [...byId.values()]
}

/** 바깥에서 온 글을 줄 하나로 다듬는다 : 제어 글자 빼기, 앞뒤 공백, 길이 자르기. */
export function clean(text: unknown, max: number): string {
  const s = typeof text === 'string' ? text : String(text ?? '')
  // eslint-disable-next-line no-control-regex
  return s.replace(/[\u0000-\u001f\u007f]/g, ' ').replace(/\s+/g, ' ').trim().slice(0, max)
}

/** 출력의 첫 줄 (실패 알림용) */
export function firstLine(text: string): string {
  return clean(text.split(/\r?\n/).find(l => l.trim() !== '') ?? '', 120)
}
