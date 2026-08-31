package estimate

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mirusona/efforttool/internal/model"
)

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
