// Package mod 은 effort-steps mod 의 돌 때 쓰는 파일을 exe 안에 담는다.
// `effort mod install` 이 이것을 ~/.claude/skills/effort-steps 로 푼다.
//
// 담는 것은 plugin.json · hooks · types 셋뿐이다. tests · tsconfig 는 저장소에만 둔다.
// 점으로 시작하는 폴더(.claude-plugin)는 폴더째 적으면 빠지므로 파일 이름으로 적는다.
// 그래서 엔진이 개발 때 까는 .claude-plugin/types/ 도 안 담긴다.
package mod

import "embed"

// Files 는 "effort-steps/..." 꼴 경로로 담긴 mod 파일이다.
//
//go:embed effort-steps/.claude-plugin/plugin.json effort-steps/hooks effort-steps/types
var Files embed.FS
