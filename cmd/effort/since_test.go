package main

import (
	"testing"
	"time"

	"github.com/mirusona/officina-effort-tool/internal/model"
)

// --since 는 사람이 사는 시간대의 날짜다. UTC 로 읽으면 KST 새벽 작업이 통째로 빠진다.
func TestSinceUsesLocalDay(t *testing.T) {
	start := time.Date(2026, 9, 18, 0, 30, 0, 0, time.Local)
	tasks := []model.Task{{PromptID: "abcd1234", Class: model.ClassBuild, Start: start}}
	got, _, err := filterTasks(tasks, "", "2026-09-18", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("그날 새벽 작업이 빠졌다 (남은 건수 %d)", len(got))
	}
	// 하루 앞 날짜로 자르면 남고, 다음 날로 자르면 빠져야 한다.
	got, _, err = filterTasks(tasks, "", "2026-09-19", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("다음 날로 잘랐는데 남았다 (남은 건수 %d)", len(got))
	}
}
