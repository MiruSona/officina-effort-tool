// 설정 값과 이름. $.state 참조(atom)는 엔진 검사 때문에 main.tsx 에 있다 (참조는 쓰는 파일 안에 있어야 한다).

import { readMode } from './gate'
import type { Mode } from './gate'

export const PLUGIN = 'effort-steps'
export const PANE = 'effort-steps'

/** 설정 값 (userConfig). register 의 options 에서 한 번 읽는다. */
export type Settings = {
  effortPath: string
  refreshMs: number
  redRatio: number
  autoPane: boolean
  /** plan 없는 편집·Agent 를 어떻게 다루나 : deny(막음) · warn(경고만) · off(안 봄). 모르는 값은 deny */
  mode: Mode
}

export function readSettings(options: Readonly<Record<string, unknown>>): Settings {
  const sec = typeof options.refreshSec === 'number' && Number.isFinite(options.refreshSec) ? options.refreshSec : 30
  const ratio = typeof options.redRatio === 'number' && options.redRatio > 1 ? options.redRatio : 1.5
  return {
    effortPath: typeof options.effortPath === 'string' ? options.effortPath : '',
    refreshMs: Math.max(10, Math.min(600, sec)) * 1000,
    redRatio: ratio,
    autoPane: options.autoPane === true,
    mode: readMode(options.mode),
  }
}
