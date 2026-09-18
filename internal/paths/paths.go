package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Home 은 파생 저장소 자리를 정한다 : --home → EFFORT_HOME → 홈 아래.
func Home(flagHome string) (string, error) {
	if flagHome != "" {
		return filepath.Abs(flagHome)
	}
	if env := os.Getenv("EFFORT_HOME"); env != "" {
		return filepath.Abs(env)
	}
	if runtime.GOOS != "windows" {
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "effort"), nil
		}
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".effort"), nil
}

// ProjectsRoot 는 읽을 뿌리를 정한다 : --projects → ~/.claude/projects.
func ProjectsRoot(flagProjects string) (string, error) {
	if flagProjects != "" {
		return filepath.Abs(flagProjects)
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".claude", "projects"), nil
}

// Slug 는 프로젝트 폴더 경로를 Claude Code 가 쓰는 폴더 이름으로 바꾼다.
// 클로드 코드는 영숫자가 아닌 글자를 모두 `-` 로 바꾼다 — 공백·한글도 마찬가지다.
func Slug(dir string) string {
	s := filepath.Clean(dir)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isAlnum(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}

func isAlnum(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	return r >= '0' && r <= '9'
}
