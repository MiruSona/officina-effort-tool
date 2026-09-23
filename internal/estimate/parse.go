package estimate

import (
	"bufio"
	"errors"
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

// ArgError 는 분류 자리에 분류가 아닌 낱말이 온 인자다.
// 파서는 규칙을 모르므로 목록·안내는 규칙을 읽은 뒤 ExplainArgError 가 붙인다.
type ArgError struct {
	Arg     string // 받은 인자 통째
	Bad     string // 분류 자리에 온 낱말
	SizeTok string // 뒤따른 칸 (크기일 수 있다). 없으면 빈 글
	// Lead 는 두 칸 꼴에서 둘 다 분류가 아닐 때의 앞 칸이다. 이때 Bad 는 뒤 칸이다.
	// 뒤 칸이 크기면(`검증:M`) 앞 칸이 분류 자리였고, 아니면(`리뷰스킬:검증`) 「이름:분류」 뜻이다.
	// 크기인지는 규칙을 알아야 하므로 ExplainArgError 가 가른다.
	Lead string
}

func (e *ArgError) Error() string {
	return fmt.Sprintf("모르는 분류 %q 입니다 (인자 %q · 꼴 [이름:]분류[:크기])", e.Bad, e.Arg)
}

// ExplainArgError 는 ArgError 에 쓸 수 있는 분류 목록과 올바른 보기를 붙인다.
// 목록은 규칙 word 표에서 읽는다 — 사람이 rules.txt 를 고치면 안내도 따라간다.
// ArgError 가 아니거나 규칙이 없으면 그대로 돌려준다.
func ExplainArgError(err error, r *classify.Rules) error {
	var ae *ArgError
	if !errors.As(err, &ae) || r == nil {
		return err
	}
	bad, size := ae.Bad, ae.SizeTok
	if ae.Lead != "" && r.HasSize(ae.Bad) {
		bad, size = ae.Lead, ae.Bad
	}
	msg := fmt.Sprintf("모르는 분류 %q 입니다.", bad)
	if size != "" && r.HasSize(size) {
		msg += fmt.Sprintf(" 크기(%s)는 맞습니다. 분류 자리가 틀렸습니다.", strings.ToUpper(size))
	}
	msg += fmt.Sprintf(" 쓸 수 있는 분류 : %s. 보기 : `조사:M` · `이름:조사:M`",
		strings.Join(workClassNames(r), " · "))
	return errors.New(msg)
}

// workClassNames 는 word 표에 나오는 분류 가운데 예상할 수 있는 「일」 칸을 표 차례로 준다.
// word 표에 일 칸이 하나도 없으면 전체 일 칸으로 대신한다 — 빈 목록 안내는 쓸모가 없다.
func workClassNames(r *classify.Rules) []string {
	if out := wordWorkClasses(r); len(out) > 0 {
		return out
	}
	out := make([]string, 0, len(model.WorkClasses))
	for _, c := range model.WorkClasses {
		out = append(out, string(c))
	}
	return out
}

func wordWorkClasses(r *classify.Rules) []string {
	seen := map[model.Class]bool{}
	for _, c := range r.Words {
		seen[c] = true
	}
	var out []string
	for _, c := range model.AllClasses {
		if seen[c] && c.IsWork() {
			out = append(out, string(c))
		}
	}
	return out
}

func parseArg(a string) (Item, error) {
	f := strings.Split(a, ":")
	switch len(f) {
	case 1:
		if !model.IsClass(f[0]) {
			return Item{}, &ArgError{Arg: a, Bad: f[0]}
		}
		return Item{Name: f[0], Class: model.Class(f[0]), Size: "M"}, nil
	case 2:
		if model.IsClass(f[0]) {
			return Item{Name: f[0], Class: model.Class(f[0]), Size: strings.ToUpper(f[1])}, nil
		}
		if !model.IsClass(f[1]) {
			// 둘 다 분류가 아니면 어느 칸을 찍을지는 뒤 칸이 크기인지로 가른다 (ArgError.Lead).
			return Item{}, &ArgError{Arg: a, Bad: f[1], Lead: f[0]}
		}
		return Item{Name: f[0], Class: model.Class(f[1]), Size: "M"}, nil
	case 3:
		if !model.IsClass(f[1]) {
			return Item{}, &ArgError{Arg: a, Bad: f[1], SizeTok: f[2]}
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
