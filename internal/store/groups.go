package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/model"
	"github.com/mirusona/officina-effort-tool/internal/secret"
)

// GroupsHead 는 groups.txt 를 처음 만들 때 넣는 머리말이다.
const GroupsHead = `# effort 소단계 정본. 탭으로 나눈다. # 은 주석. 손으로 고쳐도 된다.
# group  <묶음id>  <이름>          [<분류>]
# task   <묶음id>  <작업id 또는 앞자리>
#
# 여기 적은 것이 자동 묶기보다 먼저다. 캐시가 아니라 정본이라 scan --rebuild 로도 안 날아간다.
`

// 묶음 id·작업 id 로 쓸 수 있는 글자와 길이.
const (
	minIDLen = 4
	maxIDLen = 64
)

// Mark 는 사람이 표시한 소단계 하나다.
type Mark struct {
	ID    string
	Name  string
	Class model.Class // 비어 있으면 묶음 안 작업으로 정한다
	Tasks []string    // promptId 전체 또는 앞자리
}

func (s *Store) GroupsPath() string { return filepath.Join(s.home, "groups.txt") }

// EnsureGroups 는 groups.txt 가 없을 때만 머리말만 든 빈 파일을 만든다. 있으면 절대 안 덮는다.
func (s *Store) EnsureGroups() (created bool, err error) {
	p := s.GroupsPath()
	if _, err := os.Stat(p); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(s.home, 0o755); err != nil {
		return false, err
	}
	if err := writeAtomic(p, []byte(GroupsHead)); err != nil {
		return false, err
	}
	return true, nil
}

// LoadGroups 는 정본을 읽는다. 모르는 종류·꼴은 줄 번호와 함께 실패한다.
// 파일이 없으면 빈 목록이다 (아직 아무도 소단계를 표시 안 한 것뿐이다).
func (s *Store) LoadGroups() ([]Mark, error) {
	f, err := os.Open(s.GroupsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var order []string
	byID := map[string]*Mark{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := applyGroupLine(splitTabs(line), n, &order, byID); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]Mark, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func applyGroupLine(f []string, n int, order *[]string, byID map[string]*Mark) error {
	if len(f) == 0 {
		return nil
	}
	switch f[0] {
	case "group":
		return addGroupLine(f, n, order, byID)
	case "task":
		return addTaskLine(f, n, byID)
	}
	return fmt.Errorf("groups.txt %d번째 줄: 모르는 종류 %q (group·task 만 됩니다)", n, f[0])
}

func addGroupLine(f []string, n int, order *[]string, byID map[string]*Mark) error {
	if len(f) < 3 {
		return fmt.Errorf("groups.txt %d번째 줄: group 은 묶음id·이름 두 칸이 필요합니다", n)
	}
	if !okID(f[1]) {
		return fmt.Errorf("groups.txt %d번째 줄: 묶음 id 꼴이 아닙니다 (%s)", n, idRule())
	}
	if byID[f[1]] != nil {
		return fmt.Errorf("groups.txt %d번째 줄: 묶음 id 가 겹칩니다 %q", n, f[1])
	}
	m := &Mark{ID: f[1], Name: secret.Sanitize(f[2])}
	if len(f) >= 4 && f[3] != "" {
		if !model.IsClass(f[3]) {
			return fmt.Errorf("groups.txt %d번째 줄: 모르는 분류 %q", n, f[3])
		}
		m.Class = model.Class(f[3])
	}
	byID[m.ID] = m
	*order = append(*order, m.ID)
	return nil
}

func addTaskLine(f []string, n int, byID map[string]*Mark) error {
	if len(f) < 3 {
		return fmt.Errorf("groups.txt %d번째 줄: task 는 묶음id·작업id 두 칸이 필요합니다", n)
	}
	m := byID[f[1]]
	if m == nil {
		return fmt.Errorf("groups.txt %d번째 줄: 앞에 없는 묶음 id 입니다 %q", n, f[1])
	}
	if !okID(f[2]) {
		return fmt.Errorf("groups.txt %d번째 줄: 작업 id 꼴이 아닙니다 (%s)", n, idRule())
	}
	m.Tasks = append(m.Tasks, f[2])
	return nil
}

// AppendGroup 은 끝에 덧붙이기만 한다. 기존 줄은 읽지도 고치지도 않는다.
func (s *Store) AppendGroup(m Mark) error {
	if !okID(m.ID) {
		return fmt.Errorf("묶음 id 꼴이 아닙니다 : %q (%s)", m.ID, idRule())
	}
	for _, id := range m.Tasks {
		if !okID(id) {
			return fmt.Errorf("작업 id 꼴이 아닙니다 : %q (%s)", id, idRule())
		}
	}
	// 이름이 공백뿐이면 줄이 2칸으로 써져 이후 모든 명령이 실패한다. (탭·줄바꿈은 Sanitize 가 공백으로 바꾼다)
	name := secret.Sanitize(m.Name)
	if name == "" {
		return fmt.Errorf("묶음 이름이 비었습니다 : 한 글자 이상 적어 주세요")
	}
	if _, err := s.EnsureGroups(); err != nil {
		return err
	}
	old, err := os.ReadFile(s.GroupsPath())
	if err != nil {
		return err
	}
	body := "group\t" + m.ID + "\t" + name
	if m.Class != "" {
		body += "\t" + string(m.Class)
	}
	body += "\n"
	for _, id := range m.Tasks {
		body += "task\t" + m.ID + "\t" + id + "\n"
	}
	if len(old) > 0 && old[len(old)-1] != '\n' {
		body = "\n" + body
	}
	return writeAtomic(s.GroupsPath(), append(old, []byte(body)...))
}

// DropGroup 은 그 묶음 줄만 빼고 나머지를 글자 그대로 다시 쓴다.
// 줄 걸러내기라 주석과 사람이 쓴 줄 차례가 그대로 남는다.
func (s *Store) DropGroup(id string) (int, error) {
	if !okID(id) {
		return 0, fmt.Errorf("묶음 id 꼴이 아닙니다 : %q (%s)", id, idRule())
	}
	raw, err := os.ReadFile(s.GroupsPath())
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	lines := strings.SplitAfter(string(raw), "\n")
	var keep strings.Builder
	dropped := 0
	for _, line := range lines {
		if belongsToGroup(line, id) {
			dropped++
			continue
		}
		keep.WriteString(line)
	}
	if dropped == 0 {
		return 0, nil
	}
	return dropped, writeAtomic(s.GroupsPath(), []byte(keep.String()))
}

// belongsToGroup 은 그 줄이 이 묶음의 group·task 줄인지다. 주석과 빈 줄은 늘 거짓이다.
func belongsToGroup(line, id string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	f := splitTabs(trimmed)
	if len(f) < 2 {
		return false
	}
	if f[0] != "group" && f[0] != "task" {
		return false
	}
	return f[1] == id
}

// okID 는 묶음·작업 id 로 쓸 수 있는 값인지다.
func okID(s string) bool {
	if len(s) < minIDLen || len(s) > maxIDLen {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}

func idRule() string {
	return fmt.Sprintf("영문·숫자·- 로 %d~%d자", minIDLen, maxIDLen)
}

func splitTabs(line string) []string {
	parts := strings.Split(line, "\t")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
