// effort.exe 부르기의 순수한 반쪽 : 인자 만들기, 결과 풀기, exe 후보, marks.txt 경로.
// 실제 process.run · fs 호출은 main.tsx 에 있다 (엔진 검사가 $ 를 import 너머로 넘기지 못하게 한다).
//
// 이 mod 는 effort 를 「읽기」로만 부른다. mark start · stop · scan 을 부르면 묶기가 깨지거나
// (묶기없음) 캐시를 다시 짓는다 — 그래서 isAllowedArgs 가 그 밖의 인자를 막는다.
import type { ProcessRunResult } from 'claude-code'

import type { Step } from '../types'
import { firstLine, minutesText, parseBlocks, parseEstimate } from './parse'
import type { ShowBlock } from './parse'

export const SHOW_TIMEOUT_MS = 5_000
export const ESTIMATE_TIMEOUT_MS = 10_000

/** 부를 수 있는 인자인지. estimate 와 mark show 만 된다. */
export function isAllowedArgs(args: readonly string[]): boolean {
  if (args[0] === 'estimate') return true
  return args[0] === 'mark' && args[1] === 'show' && args.length === 3
}

function join(dir: string, name: string): string {
  return dir.endsWith('/') || dir.endsWith('\\') ? dir + name : `${dir}/${name}`
}

/**
 * exe 후보 차례 : ① 설정 effortPath ② <base>/EffortTool/bin ③ <base>/Tools/EffortTool/bin ④ PATH.
 * base 는 cwd, 다르면 저장소 뿌리도. 일반 <base>/bin 은 남의 effort 를 잡을 수 있어 보지 않는다
 * (EffortTool 저장소 안에서 쓸 때는 effortPath 설정에 그 exe 를 적는다).
 * 적힌 effortPath 에 파일이 없어도 ②~④ 를 이어서 본다 — 저장소를 옮기면 설치본 default 가 낡기 때문이다.
 */
export function exeCandidates(configured: string, bases: readonly string[], pathVar: string): string[] {
  const out: string[] = []
  if (configured.trim() !== '') out.push(configured.trim())
  for (const base of bases) {
    for (const rel of ['EffortTool/bin', 'Tools/EffortTool/bin']) {
      for (const file of ['effort.exe', 'effort']) out.push(join(join(base, rel), file))
    }
  }
  // 드라이브 글자(C:)로 시작하거나 ; 가 있으면 Windows 꼴이라 ; 로 자른다 (C: 의 : 로 자르면 안 된다)
  const sep = /^[A-Za-z]:/.test(pathVar.trim()) || pathVar.includes(';') ? ';' : ':'
  for (const dir of pathVar.split(sep)) {
    if (dir.trim() === '') continue
    for (const file of ['effort.exe', 'effort']) out.push(join(dir.trim(), file))
  }
  return out
}

/** exe 를 못 찾았을 때 띠에 띄울 글 */
export function exeMissing(configured: string): string {
  return configured.trim() !== ''
    ? `설정 effortPath 에 파일이 없고 다른 후보에도 없음 : ${configured.trim()} — EffortTool 에서 effort mod install 을 다시 치거나 /config 의 effortPath 를 고친다`
    : 'effort.exe 를 못 찾음 — EffortTool 에서 effort mod install 을 치거나 /config 에서 effort-steps 의 effortPath 를 적는다'
}

/** marks.txt 경로. EFFORT_HOME 이 있으면 그 아래, 아니면 ~/.effort. */
export function marksPathFrom(effortHome: string | undefined, home: string | undefined): string | null {
  if (effortHome !== undefined && effortHome !== '') return join(effortHome, 'marks.txt')
  if (home === undefined || home === '') return null
  return join(join(home, '.effort'), 'marks.txt')
}

/** estimate 인자. 이름에 「:」 이 들어가면 estimate 가 못 받으므로 번호만 넘긴다 (「3:구현:M」). */
export function estimateArgs(steps: readonly Step[]): string[] {
  return ['estimate', '--json', ...steps.map(s => `${s.n}:${s.cls}:${s.size}`)]
}

/** estimate 결과 : 소단계마다 예상 값을 채운 목록과, 못 채웠을 때 까닭. */
export type EstimateOut = { steps: Step[]; note: string | null }

/** estimate 결과를 푼다. out 이 글이면 실행 실패 까닭이다. 실패하면 예상 칸은 「—」. */
export function readEstimate(steps: readonly Step[], out: ProcessRunResult | string): EstimateOut {
  const blank = steps.map(s => ({ ...s, estText: '—', estMs: null }))
  if (typeof out === 'string') return { steps: blank, note: out }
  const map = out.exitCode === 0 ? parseEstimate(out.stdout) : new Map<string, number>()
  if (map.size === 0) {
    const why = firstLine(out.stderr) || firstLine(out.stdout) || `종료 ${out.exitCode}`
    const scan = out.exitCode === 2 || /scan/.test(why) ? ' — effort scan 을 한 번 돌려야 한다 (이 mod 는 scan 을 부르지 않는다)' : ''
    return { steps: blank, note: `estimate 실패 : ${why}${scan}` }
  }
  return {
    steps: steps.map(s => {
      const ms = map.get(String(s.n)) ?? null
      return { ...s, estMs: ms, estText: minutesText(ms) }
    }),
    note: null,
  }
}

export const MARK_ID_RE = /^m\d{4}-[0-9a-f]{4,8}$/

/** mark show 결과 : 덩이 하나, 또는 실패 까닭. */
export type ShowOut = { block: ShowBlock | null; error: string | null }

/** mark show 결과를 푼다. out 이 글이면 실행 실패 까닭이다. */
export function readShow(id: string, out: ProcessRunResult | string): ShowOut {
  if (typeof out === 'string') return { block: null, error: `show 실패: ${firstLine(out)}` }
  const block = parseBlocks(out.stdout).find((b): b is ShowBlock => b.kind === 'show' && b.id === id) ?? null
  if (out.exitCode !== 0 || block === null || block.recordText === null) {
    const why = firstLine(out.stderr) || firstLine(out.stdout) || `종료 ${out.exitCode}`
    return { block: null, error: `show 실패: ${why}` }
  }
  return { block, error: null }
}
