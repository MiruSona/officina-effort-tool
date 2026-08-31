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
func Slug(dir string) string {
	s := filepath.Clean(dir)
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}
