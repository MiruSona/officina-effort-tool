// effort-steps 의 $.state 계약. 값 모양은 여기 한 곳에만 적는다.

/** 소단계 상태. AGENTS 1-1 표의 「상태」 칸 글 그대로다. */
export type StepStatus = '할 일' | '하는 중' | '끝' | '건너뜀'

/** 계획의 소단계 하나. 실제 값은 여기 두지 않고 marks 에서 그때그때 모은다. */
export type Step = {
  n: number
  name: string
  cls: string
  size: string
  /** estimate 가 준 중앙값 글. 못 받았으면 「—」 */
  estText: string
  estMs: number | null
  /** 이 소단계에 짝지은 mark id. 같은 번호로 여러 번 열면 덧붙는다 */
  markIds: string[]
  status: StepStatus
  /** step 도구로 남긴 까닭 한 줄 */
  note: string | null
}

export type Plan = {
  id: string
  title: string
  /** $.clock.now() 밀리초. 이보다 먼저 시작된 mark 는 짝짓지 않는다 */
  createdAt: number
  /** 갈아 끼운 계획이면 이전 계획의 createdAt. 이름 짝 시각 하한을 여기까지 넓힌다 (pair.ts nameFloor) */
  prevAt?: number
  sessionId: string
  steps: Step[]
  /** estimate 를 못 불렀을 때 까닭 (예 「scan 한 번 필요」) */
  estNote: string | null
}

/** 본 mark 하나. start 출력이나 show·stop 출력에서 채운다. */
export type MarkInfo = {
  id: string
  name: string
  /** 이름 앞 번호. 번호가 없거나 계획보다 먼저 열린 mark 는 null (짝 없음) */
  n: number | null
  agentId: string | null
  open: boolean
  /** show·stop 의 「기록 구간」 원문 */
  recordText: string | null
  recordMs: number | null
  waitText: string | null
  flags: string[]
  /** show 출력의 「주의 :」 줄 */
  warn: string | null
  /** 마지막 show 실패 까닭 */
  error: string | null
  fails: number
  /** 같은 recordText 가 이어진 횟수 (도구 하나가 오래 돌 때 알리려고) */
  same: number
  seenAt: number
  lastAt: number
}

export type ExeInfo = { path: string | null; why: string | null }

export type UiPrefs = { bandHidden: boolean; expanded: boolean }

declare module 'claude-code' {
  interface PluginState {
    'effort-steps': {
      plan: Plan | null
      marks: Record<string, MarkInfo>
      exe: ExeInfo
      ui: UiPrefs
      /** 타이머가 멈췄는지 (show 세 번 잇달아 실패) */
      paused: boolean
      /** session.start 의 isInteractive. 거짓이면 강제(막기·경고)를 안 한다 */
      interactive: boolean
      /**
       * /steps allow (사람이 터미널에서 친 것). 다음 사람 말(슬래시 아님) 한 번을 살아남는다.
       * 그 말에서 used 를 true 로 두고, 그 다음 사람 말에 지운다.
       * (allow 는 막혀 턴이 끝난 뒤 치므로, 바로 다음 「계속」 에 지우면 쓸모가 없다)
       */
      allow: { at: number; used: boolean } | null
      /** 이번 턴에 경고 줄을 이미 붙였나. 사람 말에 지운다 */
      turn: { warned: boolean; noMarkWarned: boolean }
      /** 안 닫힌 mark 로 이미 toast 한 mark id (mark 하나에 한 번) */
      toasted: string[]
    }
  }
}
