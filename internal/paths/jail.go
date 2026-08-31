package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Jail 은 한 뿌리 아래로만 닿게 막는다 (security 1).
type Jail struct {
	root string
}

// NewJail 은 뿌리를 정규화해 감옥을 만든다.
func NewJail(root string) (*Jail, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// 아직 없는 폴더는 그대로 쓴다 (.effort 는 처음에 없다).
		real = abs
	}
	return &Jail{root: filepath.Clean(real)}, nil
}

func (j *Jail) Root() string { return j.root }

// Resolve 는 뿌리 안 경로인지 보고 정규화된 경로를 준다.
func (j *Jail) Resolve(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("경로에 못 쓰는 글자가 있습니다")
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(j.root, p)
	}
	abs = filepath.Clean(abs)
	real, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = filepath.Clean(real)
	}
	if !under(j.root, abs) {
		return "", fmt.Errorf("뿌리 밖 경로입니다 : %s", p)
	}
	return abs, nil
}

// OpenRead 는 뿌리 안의 보통 파일만 읽기로 연다.
func (j *Jail) OpenRead(p string) (*os.File, error) {
	abs, err := j.Resolve(p)
	if err != nil {
		return nil, err
	}
	st, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("링크는 안 엽니다 : %s", p)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("보통 파일이 아닙니다 : %s", p)
	}
	return os.Open(abs)
}

func under(root, p string) bool {
	r := normCase(root)
	x := normCase(p)
	if r == x {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(x, r)
}

func normCase(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}

// Overlap 은 두 뿌리가 겹치는지 본다. 겹치면 캐시를 원본에 쓰는 사고가 난다.
func Overlap(a, b *Jail) bool {
	return under(a.root, b.root) || under(b.root, a.root)
}
