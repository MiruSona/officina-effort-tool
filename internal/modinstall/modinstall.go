// Package modinstall 은 exe 에 담긴 effort-steps mod 를 Claude Code 폴더에 풀고, 견주고, 걷어 낸다.
//
// 뿌리(claude-dir)는 늘 인자로 받는다. 시험은 임시 폴더를 뿌리로 쓴다.
// 쓰는 자리는 <claude-dir>/skills/effort-steps 와 <claude-dir>/mod-backups 둘뿐이고, 각각 paths.Jail 로 묶는다.
// 지우는 코드는 없다 — 덮어쓰기·걷어 내기는 늘 mod-backups 아래 .bak-<시각> 으로 옮기기다.
package modinstall

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/paths"
)

// Name 은 mod 이름이다. 바꾸면 mcp__effort-steps__plan 같은 도구 이름이 다 깨진다.
const Name = "effort-steps"

const (
	pluginRel  = ".claude-plugin/plugin.json"
	stampFmt   = "20060102-150405"
	maxSameSec = 100
)

// Installer 는 한 claude-dir 에 대한 설치기다.
type Installer struct {
	claudeDir string
	src       fs.FS
	// target 은 설치 자리 그대로의 경로다. 링크를 풀지 않는다 — 링크면 Lstat 로 알아보고 멈춘다.
	// (Jail 은 뿌리의 링크를 풀어 버리므로 옮기기·Lstat 에는 이 값을 쓴다.)
	target  string
	skills  *paths.Jail
	backups *paths.Jail

	// Now 는 백업 이름에 쓰는 시계다. 시험만 바꾼다.
	Now func() time.Time
	// rename 은 옮기기다. 시험이 실패를 꾸밀 때만 바꾼다.
	rename func(oldPath, newPath string) error
}

// New 는 claudeDir 아래에 src("effort-steps/..." 꼴 경로)를 풀 설치기를 만든다.
func New(claudeDir string, src fs.FS) (*Installer, error) {
	abs, err := filepath.Abs(claudeDir)
	if err != nil {
		return nil, err
	}
	// 뿌리 자체가 링크(짧은 이름 · /var→/private/var 따위)면 먼저 푼다. 안 풀면 Jail 이 푼 경로와 어긋난다.
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	target := filepath.Join(abs, "skills", Name)
	skills, err := paths.NewJail(target)
	if err != nil {
		return nil, err
	}
	backups, err := paths.NewJail(filepath.Join(abs, "mod-backups"))
	if err != nil {
		return nil, err
	}
	if paths.Overlap(skills, backups) {
		return nil, fmt.Errorf("설치 자리와 백업 자리가 겹칩니다 (%s / %s)", skills.Root(), backups.Root())
	}
	return &Installer{
		claudeDir: abs,
		src:       src,
		target:    target,
		skills:    skills,
		backups:   backups,
		Now:       time.Now,
		rename:    os.Rename,
	}, nil
}

// InstallDir 는 설치본 폴더다 (<claude-dir>/skills/effort-steps).
func (in *Installer) InstallDir() string { return in.target }

// InstallResult 는 install 한 판의 결과다.
type InstallResult struct {
	Backup    string // 옛 설치본을 옮긴 자리. 옛 것이 없었으면 빈 값
	Installed string
	Version   string // 담긴 plugin.json 의 version
}

// Install 은 담긴 파일을 풀어 skills/effort-steps 로 바꿔 끼운다.
// effortPath 는 plugin.json userConfig.effortPath.default 에 넣을 exe 경로다.
//
// 차례 : ① mod-backups/effort-steps.new-<시각> 에 다 쓴다 ② 옛 설치본을 .bak-<시각> 으로 옮긴다
// ③ 새 폴더를 skills/effort-steps 로 옮긴다. ③ 이 실패하면 ② 를 되돌린다.
// 중간에 죽어도 skills 아래에는 반쪽 설치본이 안 남는다.
func (in *Installer) Install(effortPath string) (*InstallResult, error) {
	version, err := EmbeddedVersion(in.src)
	if err != nil {
		return nil, err
	}
	stamp := in.Now().Format(stampFmt)

	newDir, err := in.freeBackupName("new", stamp)
	if err != nil {
		return nil, err
	}
	if err := in.writeTree(newDir, effortPath); err != nil {
		return nil, fmt.Errorf("새 판을 못 풀었습니다 (%s) : %w", newDir, err)
	}

	backup, err := in.moveAside(stamp)
	if err != nil {
		return nil, err
	}

	target := in.target
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, in.undoMove(backup, fmt.Errorf("skills 폴더를 못 만들었습니다 : %w", err))
	}
	if err := in.rename(newDir, target); err != nil {
		return nil, in.undoMove(backup, fmt.Errorf("새 판을 설치 자리로 못 옮겼습니다 (%s → %s) : %w", newDir, target, err))
	}
	return &InstallResult{Backup: backup, Installed: target, Version: version}, nil
}

// Uninstall 은 설치본을 mod-backups/effort-steps.bak-<시각> 으로 옮긴다.
// 설치본이 없으면 빈 값과 nil 을 돌려준다.
func (in *Installer) Uninstall() (string, error) {
	return in.moveAside(in.Now().Format(stampFmt))
}

// moveAside 는 설치본이 있으면 .bak-<시각> 으로 옮기고 그 자리를 돌려준다.
func (in *Installer) moveAside(stamp string) (string, error) {
	present, err := in.installedDir()
	if err != nil || !present {
		return "", err
	}
	bak, err := in.freeBackupName("bak", stamp)
	if err != nil {
		return "", err
	}
	if err := in.rename(in.target, bak); err != nil {
		return "", fmt.Errorf("옛 설치본을 백업으로 못 옮겼습니다 (%s → %s) : %w", in.target, bak, err)
	}
	return bak, nil
}

// undoMove 는 옮겨 둔 옛 설치본을 제자리로 되돌린다. 되돌리기도 실패하면 둘 다 알린다.
func (in *Installer) undoMove(backup string, cause error) error {
	if backup == "" {
		return cause
	}
	if err := in.rename(backup, in.target); err != nil {
		return fmt.Errorf("%w — 옛 설치본도 못 되돌렸습니다. %s 를 %s 로 손으로 옮기세요 : %v",
			cause, backup, in.target, err)
	}
	return fmt.Errorf("%w — 옛 설치본은 제자리로 되돌렸습니다", cause)
}

// installedDir 는 설치본 폴더가 있는지다. 링크·파일이면 손대지 않고 오류로 멈춘다.
func (in *Installer) installedDir() (bool, error) {
	st, err := os.Lstat(in.target)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("설치 자리가 링크라 손대지 않습니다 : %s", in.target)
	}
	if !st.IsDir() {
		return false, fmt.Errorf("설치 자리가 폴더가 아닙니다 : %s", in.target)
	}
	return true, nil
}

// freeBackupName 은 mod-backups 아래 아직 없는 effort-steps.<kind>-<시각> 자리를 고른다.
// 같은 초에 두 번 치면 뒤에 -2, -3 을 붙인다.
func (in *Installer) freeBackupName(kind, stamp string) (string, error) {
	if err := os.MkdirAll(in.backups.Root(), 0o755); err != nil {
		return "", fmt.Errorf("백업 폴더를 못 만들었습니다 : %w", err)
	}
	base := Name + "." + kind + "-" + stamp
	for i := 1; i <= maxSameSec; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		p, err := in.backups.Resolve(name)
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			return p, nil
		}
	}
	return "", fmt.Errorf("백업 이름이 %d개 넘게 겹칩니다 : %s", maxSameSec, base)
}

// writeTree 는 담긴 파일을 dir 아래에 쓴다. plugin.json 은 effortPath 기본값만 바꿔 쓴다.
func (in *Installer) writeTree(dir, effortPath string) error {
	files, err := embeddedFiles(in.src)
	if err != nil {
		return err
	}
	for _, rel := range files {
		raw, err := fs.ReadFile(in.src, path.Join(Name, rel))
		if err != nil {
			return err
		}
		if rel == pluginRel {
			if raw, err = withEffortPath(raw, effortPath); err != nil {
				return err
			}
		}
		dst, err := in.backups.Resolve(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// embeddedFiles 는 담긴 파일의 effort-steps 아래 경로("/" 꼴)를 차례대로 준다.
func embeddedFiles(src fs.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(src, Name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, p[len(Name)+1:])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("담긴 mod 파일을 못 읽었습니다 : %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("담긴 mod 파일이 없습니다")
	}
	return out, nil
}

// DevModOverlaps 는 <claude-dir>/dev-mods/*/effort-steps 가 있으면 그 경로들을 준다.
// 겹치면 다음 시작 때 설치본이 「name already taken by a session-only plugin」 으로 거절된다.
func (in *Installer) DevModOverlaps() ([]string, error) {
	root := filepath.Join(in.claudeDir, "dev-mods")
	ents, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name(), Name)
		if _, err := os.Lstat(p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}
