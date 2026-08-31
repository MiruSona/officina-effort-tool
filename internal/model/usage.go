package model

// Usage 는 한 번의 API 호출이 쓴 토큰이다.
type Usage struct {
	Input       int64 `json:"in"`
	Output      int64 `json:"out"`
	CacheRead   int64 `json:"cr"`
	CacheCreate int64 `json:"cc"`
	Thinking    int64 `json:"th"`
}

// Total 은 실제로 오간 토큰 전부다. Thinking 은 Output 안쪽 값이라 안 더한다.
func (u Usage) Total() int64 {
	return u.Input + u.Output + u.CacheRead + u.CacheCreate
}

// Billable 은 cache read 를 뺀 값이다 (cache read 는 값이 싸다).
func (u Usage) Billable() int64 {
	return u.Input + u.Output + u.CacheCreate
}

func (u *Usage) Add(o Usage) {
	u.Input += o.Input
	u.Output += o.Output
	u.CacheRead += o.CacheRead
	u.CacheCreate += o.CacheCreate
	u.Thinking += o.Thinking
}

// ModelUsage 는 모델 이름별 토큰이다. 열쇠는 Normalize 를 지난 이름이다.
type ModelUsage map[string]Usage

func (m ModelUsage) Add(name string, u Usage) {
	if m == nil {
		return
	}
	cur := m[name]
	cur.Add(u)
	m[name] = cur
}

func (m ModelUsage) Merge(o ModelUsage) {
	for name, u := range o {
		m.Add(name, u)
	}
}

func (m ModelUsage) Sum() Usage {
	var out Usage
	for _, u := range m {
		out.Add(u)
	}
	return out
}
