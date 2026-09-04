package main

import "testing"

func TestPctOf(t *testing.T) {
	// 1..10 이면 p50 은 다섯째 자리(5), p20 은 셋째(3), p95 는 열째(10)다.
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := []struct {
		p    int
		want float64
	}{{0, 1}, {20, 3}, {50, 6}, {80, 8}, {95, 10}, {100, 10}}
	for _, c := range cases {
		if got := pctOf(xs, c.p); got != c.want {
			t.Fatalf("p%d = %v, 바란 값 %v", c.p, got, c.want)
		}
	}
	if pctOf(nil, 50) != 0 {
		t.Fatal("빈 목록인데 0 이 아니다")
	}
	if pctOf([]float64{7}, 50) != 7 {
		t.Fatal("한 건짜리 목록이 틀렸다")
	}
}

// --measure 는 읽기 전용이다. 표본이 모자라면 값을 지어내지 않고 막는다.
func TestRulesMeasureNeedsSamples(t *testing.T) {
	home := t.TempDir()
	if code, _ := capture(t, "rules", "--home", home, "--check"); code != exitOK {
		t.Fatal("빈 저장소에서 rules --check 가 실패했다")
	}
	code, out := capture(t, "rules", "--home", home, "--measure")
	if code == exitOK {
		t.Fatalf("표본이 없는데 성공했다 : %s", out)
	}
}
