package store

import (
	"os"
	"strings"
	"testing"

	"github.com/mirusona/efforttool/internal/model"
)

func newGroupStore(t *testing.T) *Store {
	t.Helper()
	return New(t.TempDir())
}

func TestGroupsAppendKeepsComments(t *testing.T) {
	s := newGroupStore(t)
	if _, err := s.EnsureGroups(); err != nil {
		t.Fatal(err)
	}
	head, err := os.ReadFile(s.GroupsPath())
	if err != nil {
		t.Fatal(err)
	}
	err = s.AppendGroup(Mark{
		ID: "g20260901-01", Name: "1. 벽시계 고치기",
		Class: model.ClassBuild, Tasks: []string{"7bcf9a45", "31ee27e1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.GroupsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), string(head)) {
		t.Fatal("머리말 주석이 사라졌다")
	}
	marks, err := s.LoadGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 1 || len(marks[0].Tasks) != 2 || marks[0].Class != model.ClassBuild {
		t.Fatalf("읽은 정본 = %+v", marks)
	}
}

func TestGroupsDropKeepsOtherLines(t *testing.T) {
	s := newGroupStore(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.AppendGroup(Mark{ID: "g20260901-01", Name: "첫째", Tasks: []string{"aaaa1111"}}))
	must(s.AppendGroup(Mark{ID: "g20260901-02", Name: "둘째", Tasks: []string{"bbbb2222"}}))
	n, err := s.DropGroup("g20260901-01")
	must(err)
	if n != 2 {
		t.Fatalf("지운 줄 = %d, 바란 값 2", n)
	}
	marks, err := s.LoadGroups()
	must(err)
	if len(marks) != 1 || marks[0].ID != "g20260901-02" || len(marks[0].Tasks) != 1 {
		t.Fatalf("남은 정본 = %+v", marks)
	}
	raw, err := os.ReadFile(s.GroupsPath())
	must(err)
	if !strings.Contains(string(raw), "# effort 소단계 정본") {
		t.Fatal("주석이 사라졌다")
	}
}

func TestGroupsRejectsBadID(t *testing.T) {
	s := newGroupStore(t)
	bad := []Mark{
		{ID: "g1", Name: "너무 짧은 id", Tasks: []string{"aaaa1111"}},
		{ID: "g20260901-01", Name: "경로가 든 작업 id", Tasks: []string{"../../etc"}},
		{ID: "g/20260901", Name: "슬래시", Tasks: []string{"aaaa1111"}},
	}
	for _, m := range bad {
		if err := s.AppendGroup(m); err == nil {
			t.Fatalf("%+v 를 받아들였다", m)
		}
	}
}

// 이름에 든 탭·파이프는 표와 파일 꼴을 깨므로 살균을 지나야 한다.
func TestGroupsSanitizesName(t *testing.T) {
	s := newGroupStore(t)
	if err := s.AppendGroup(Mark{
		ID: "g20260901-01", Name: "가\t나|다\n라", Tasks: []string{"aaaa1111"},
	}); err != nil {
		t.Fatal(err)
	}
	marks, err := s.LoadGroups()
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(marks[0].Name, "\t\n|") {
		t.Fatalf("살균을 안 지났다 : %q", marks[0].Name)
	}
}

func TestGroupsUnknownKindFails(t *testing.T) {
	s := newGroupStore(t)
	body := "# 주석\ngroup\tg20260901-01\t이름\n이상한\tg20260901-01\t뭐냐\n"
	if err := os.WriteFile(s.GroupsPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.LoadGroups()
	if err == nil {
		t.Fatal("모르는 종류를 안 막았다")
	}
	if !strings.Contains(err.Error(), "3번째 줄") {
		t.Fatalf("줄 번호가 없다 : %v", err)
	}
}

func TestGroupsMissingFileIsEmpty(t *testing.T) {
	s := newGroupStore(t)
	marks, err := s.LoadGroups()
	if err != nil || marks != nil {
		t.Fatalf("없는 파일에서 %v · %v", marks, err)
	}
}
