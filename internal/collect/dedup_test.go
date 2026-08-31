package collect

import (
	"testing"

	"github.com/mirusona/efforttool/internal/model"
)

func TestDedupLastWins(t *testing.T) {
	d := NewDedup()
	k := usageKey{RequestID: "req_1", MessageID: "msg_1", Model: "m"}
	for _, out := range []int64{10, 40, 90, 150, 220} {
		d.Put(k, model.Usage{Input: 5, Output: out, CacheCreate: 100})
	}
	sum := d.Sum()
	if sum.Output != 220 {
		t.Fatalf("Output = %d, 바란 값 220 (마지막 것만 남아야 한다)", sum.Output)
	}
	if sum.CacheCreate != 100 {
		t.Fatalf("CacheCreate = %d, 바란 값 100", sum.CacheCreate)
	}
	if d.Count() != 1 {
		t.Fatalf("Count = %d", d.Count())
	}
}

func TestDedupNoRequestIDNeverMerges(t *testing.T) {
	d := NewDedup()
	d.Put(usageKey{RequestID: "", MessageID: "uuid-a"}, model.Usage{Output: 10})
	d.Put(usageKey{RequestID: "", MessageID: "uuid-b"}, model.Usage{Output: 20})
	if d.Sum().Output != 30 {
		t.Fatalf("Output = %d, 바란 값 30", d.Sum().Output)
	}
}

func TestDedupRollbackCounted(t *testing.T) {
	d := NewDedup()
	k := usageKey{RequestID: "req_1", MessageID: "msg_1"}
	d.Put(k, model.Usage{Output: 100})
	d.Put(k, model.Usage{Output: 40})
	if d.Rollback != 1 {
		t.Fatalf("Rollback = %d, 바란 값 1", d.Rollback)
	}
	if d.Sum().Output != 40 {
		t.Fatalf("last-wins 가 아니다 : %d", d.Sum().Output)
	}
}

func TestDedupSumByModel(t *testing.T) {
	d := NewDedup()
	d.Put(usageKey{RequestID: "r1", MessageID: "m1", Model: "opus"}, model.Usage{Output: 10})
	d.Put(usageKey{RequestID: "r2", MessageID: "m2", Model: "sonnet"}, model.Usage{Output: 20})
	mu := d.SumByModel()
	if mu["opus"].Output != 10 || mu["sonnet"].Output != 20 {
		t.Fatalf("SumByModel = %+v", mu)
	}
}
