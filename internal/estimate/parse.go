package estimate

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mirusona/officina-effort-tool/internal/classify"
	"github.com/mirusona/officina-effort-tool/internal/model"
)

// CheckItems 는 규칙을 읽은 뒤 크기 이름을 검사한다.
// 파서는 규칙을 모르므로 여기서 본다. 모르는 크기를 M 으로 삼키면 조용한 오답이 된다.
func CheckItems(items []Item, r *classify.Rules) error {
	for _, it := range items {
		if r.HasSize(it.Size) {
			continue
		}
		return fmt.Errorf("모르는 크기 %q 입니다 (쓸 수 있는 크기 : %s)",
			it.Size, strings.Join(r.SizeNames(), " · "))
	}
	return nil
}

// ParseArgs 는 `[이름:]분류[:크기]` 꼴 인자를 읽는다.
func ParseArgs(args []string) ([]Item, error) {
	var out []Item
	for _, a := range args {
		it, err := parseArg(a)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func parseArg(a string) (Item, error) {
	f := strings.Split(a, ":")
	switch len(f) {
	case 1:
		if !model.IsClass(f[0]) {
			return Item{}, fmt.Errorf("모르는 분류 %q", f[0])
		}
		return Item{Name: f[0], Class: model.Class(f[0]), Size: "M"}, nil
	case 2:
		if model.IsClass(f[0]) {
			return Item{Name: f[0], Class: model.Class(f[0]), Size: strings.ToUpper(f[1])}, nil
		}
		if !model.IsClass(f[1]) {
			return Item{}, fmt.Errorf("모르는 분류 %q", f[1])
		}
		return Item{Name: f[0], Class: model.Class(f[1]), Size: "M"}, nil
	case 3:
		if !model.IsClass(f[1]) {
			return Item{}, fmt.Errorf("모르는 분류 %q", f[1])
		}
		return Item{Name: f[0], Class: model.Class(f[1]), Size: strings.ToUpper(f[2])}, nil
	}
	return Item{}, fmt.Errorf("인자 꼴이 아닙니다 : %q ([이름:]분류[:크기])", a)
}

// ParseTable 은 공수기록.md 와 같은 파이프 표를 읽는다.
func ParseTable(r io.Reader) ([]Item, error) {
	var out []Item
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := tableCells(line)
		if len(cells) < 2 || isSeparator(cells) {
			continue
		}
		if !model.IsClass(cells[1]) {
			continue // 머리 줄
		}
		it := Item{Name: cells[0], Class: model.Class(cells[1]), Size: "M"}
		if len(cells) >= 3 && cells[2] != "" {
			it.Size = strings.ToUpper(cells[2])
		}
		if len(cells) >= 4 && cells[3] != "" {
			v, err := strconv.ParseFloat(cells[3], 64)
			if err != nil {
				return nil, fmt.Errorf("%d번째 줄: 사람눈금이 숫자가 아닙니다 (%s)", n, cells[3])
			}
			// 0 을 넣으면 「안 적었다」와 구별이 안 돼 안내 푸터가 뜬다. 빈 칸으로 두게 막는다.
			if v <= 0 {
				return nil, fmt.Errorf("%d번째 줄: 사람눈금은 1분 이상입니다 (%s). 없으면 칸을 비웁니다", n, cells[3])
			}
			it.Human = v
		}
		out = append(out, it)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func tableCells(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(strings.Trim(strings.TrimSpace(p), "*")))
	}
	return out
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if c == "" {
			continue
		}
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
