// effort-steps 가 스스로 등록하는 도구 셋의 이름을 엔진 타입에 알린다.
// 엔진이 깔아 주는 .claude-plugin/types/claude-code-mcp 는 세션에 붙은 다른 서버 도구만 적고
// 이 mod 의 도구는 빠진다. 그 목록이 하나라도 있으면 거기 없는 mcp__ 이름은 tool.call 매처·$.tool.call 에서 타입 오류다.
// 입력은 느슨하게 둔다 — 처리기(planTool·stepTool)가 모양을 직접 검사한다. 동작에는 영향 없음 (타입만).
export {}
declare module 'claude-code' {
  interface McpToolInputs {
    'mcp__effort-steps__plan': { [argument: string]: unknown }
    'mcp__effort-steps__step': { [argument: string]: unknown }
    'mcp__effort-steps__table': { [argument: string]: unknown }
  }
}
