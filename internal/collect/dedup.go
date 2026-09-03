package collect

import "github.com/mirusona/officina-effort-tool/internal/model"

// usageKey 는 같은 API 호출을 알아보는 열쇠다.
type usageKey struct {
	RequestID string
	MessageID string
	Model     string
}

// Dedup 은 스트리밍 스냅샷 중복을 걷어낸다 (mem 20260831-4dcdb496).
type Dedup struct {
	last     map[usageKey]model.Usage
	max      map[usageKey]int64
	Rollback int
}

func NewDedup() *Dedup {
	return &Dedup{
		last: make(map[usageKey]model.Usage),
		max:  make(map[usageKey]int64),
	}
}

// Put 은 한 줄을 넣는다. 같은 열쇠면 마지막 값이 이긴다 (output 은 단조 증가).
func (d *Dedup) Put(k usageKey, u model.Usage) {
	if prevMax, ok := d.max[k]; ok && u.Output < prevMax {
		d.Rollback++
	}
	if u.Output > d.max[k] {
		d.max[k] = u.Output
	}
	d.last[k] = u
}

func (d *Dedup) Sum() model.Usage {
	var out model.Usage
	for _, u := range d.last {
		out.Add(u)
	}
	return out
}

// SumByModel 은 모델 이름별로 나눠 더한다.
func (d *Dedup) SumByModel() model.ModelUsage {
	out := model.ModelUsage{}
	for k, u := range d.last {
		out.Add(k.Model, u)
	}
	return out
}

// Count 는 중복을 걷어낸 뒤 남은 호출 수다.
func (d *Dedup) Count() int { return len(d.last) }
