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
	// 아직 없는 폴더(.effort 는 처음에 없다)는 가장 가까운 있는 조상까지 링크를 푼다.
	// Resolve 도 같은 길로 풀어야 짧은 이름(8.3)·링크 조상이 양쪽에서 같은 꼴이 된다.
	real, err := resolveExisting(filepath.Clean(abs))
	if err != nil {
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
	// 아직 없는 파일도 가장 가까운 있는 조상까지 링크를 풀어 견준다 (security 1).
	// 그냥 EvalSymlinks 만 하면 끝 파일이 없을 때 실패해 문자열 비교로 떨어지고, 중간 폴더가 링크·정션이면 밖으로 나간다.
	real, err := resolveExisting(abs)
	if err != nil {
		return "", fmt.Errorf("%v : %s", err, p)
	}
	abs = filepath.Clean(real)
	if !under(j.root, abs) {
		return "", fmt.Errorf("뿌리 밖 경로입니다 : %s", p)
	}
	if err := noReparseBelow(j.root, abs); err != nil {
		return "", fmt.Errorf("%v : %s", err, p)
	}
	return abs, nil
}

// noReparseBelow 는 뿌리 아래 abs 까지의 있는 조각 가운데 링크·정션이 없는지 본다.
// Go 1.23 부터 EvalSymlinks 는 윈도우 정션(mount point)을 풀지 않는다 — 그대로 두면 정션 중간 폴더로 밖에 닿는다.
// 정션은 Lstat 에 ModeIrregular 로 보인다. 뿌리 자체가 정션인 것은 사람이 고른 것이라 본다.
func noReparseBelow(root, abs string) error {
	for cur := abs; normCase(cur) != normCase(root); {
		if st, err := os.Lstat(cur); err == nil && st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("뿌리 안에 링크·정션이 끼어 있습니다 (%s)", cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return nil
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

// resolveExisting 은 abs(정리된 절대 경로)의 가장 가까운 있는 조상까지 링크를 풀고 남은 조각을 다시 붙인다.
// 있는데 못 푸는 링크(끊긴 링크 따위)가 끼면 오류다 — 그 너머로 쓰면 링크를 따라 밖으로 나간다.
func resolveExisting(abs string) (string, error) {
	cur := abs
	var rest []string
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(append([]string{real}, rest...)...), nil
		}
		if st, lerr := os.Lstat(cur); lerr == nil {
			if st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return "", fmt.Errorf("링크를 못 풉니다 (끊긴 링크 등)")
			}
			// 있는 보통 항목인데 못 풀면(드문 드라이브 등) 예전처럼 그대로 견준다
			return filepath.Join(append([]string{cur}, rest...)...), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = append([]string{filepath.Base(cur)}, rest...)
		cur = parent
	}
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
