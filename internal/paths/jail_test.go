package paths

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestJailRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	j, err := NewJail(root)
	if err != nil {
		t.Fatal(err)
	}
	bad := []string{
		filepath.Join("..", "밖"),
		filepath.Join("a", "..", "..", "밖"),
		filepath.Join(t.TempDir(), "다른뿌리"),
	}
	for _, p := range bad {
		if _, err := j.Resolve(p); err == nil {
			t.Fatalf("뿌리 밖인데 통과했다 : %s", p)
		}
	}
	if _, err := j.Resolve("안쪽/파일.txt"); err != nil {
		t.Fatalf("뿌리 안인데 막혔다 : %v", err)
	}
}

func TestJailRejectsSymlinkedMiddleDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "중간")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("링크를 못 만든다 (권한) : %v", err)
	}
	j, err := NewJail(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join("중간", "파일.txt")); err == nil {
		t.Fatal("중간 폴더가 링크인데 통과했다 (문자열 비교로는 못 막는다)")
	}
}

func TestJailOpenReadRejectsDir(t *testing.T) {
	root := t.TempDir()
	j, _ := NewJail(root)
	if _, err := j.OpenRead("."); err == nil {
		t.Fatal("폴더를 열었다")
	}
}

func TestOverlapDetected(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "안")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	a, _ := NewJail(root)
	b, _ := NewJail(inner)
	if !Overlap(a, b) {
		t.Fatal("겹치는데 못 잡았다")
	}
	c, _ := NewJail(t.TempDir())
	if Overlap(a, c) {
		t.Fatal("안 겹치는데 겹친다고 했다")
	}
}

func TestSlugMatchesClaudeFolderName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("윈도우 경로 꼴 시험")
	}
	// Claude Code 가 만드는 폴더 이름과 같은 꼴인지만 본다. 실제 저장소 경로를 쓰지 않는다.
	got := Slug(`D:\Work\Sample_Proj\my.app`)
	want := "D--Work-Sample-Proj-my-app"
	if got != want {
		t.Fatalf("Slug = %q, 바란 값 %q", got, want)
	}
}

// 윈도우 정션(mklink /J)은 권한 없이 만들 수 있다. 중간 폴더가 정션이고 끝 파일이 아직 없어도 막혀야 한다.
func TestJailRejectsJunctionMiddleDirForNewFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("윈도우 정션 시험")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "중간")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("정션을 못 만든다 : %v %s", err, out)
	}
	j, err := NewJail(root)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := j.Resolve(filepath.Join("중간", "새파일.txt")); err == nil {
		t.Fatalf("중간 폴더가 정션인데 아직 없는 파일이 통과했다 : %s", p)
	}
	if _, err := j.Resolve(filepath.Join("진짜폴더", "새파일.txt")); err != nil {
		t.Fatalf("뿌리 안 아직 없는 파일이 막혔다 : %v", err)
	}
}
