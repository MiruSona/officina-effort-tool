package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mirusona/officina-effort-tool/internal/paths"
	"github.com/mirusona/officina-effort-tool/internal/store"
)

// Version 은 이 툴의 판이다.
const Version = "0.1.0"

// 종료 코드.
const (
	exitOK      = 0
	exitUsage   = 1
	exitNoData  = 2
	exitRead    = 3
	exitWrite   = 4
	exitCorrupt = 5
)

// codedError 는 종료 코드를 달고 다니는 오류다.
type codedError struct {
	code int
	msg  string
}

func (e *codedError) Error() string { return e.msg }

func fail(code int, format string, a ...any) error {
	return &codedError{code: code, msg: fmt.Sprintf(format, a...)}
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printHelp("")
		return exitUsage
	}
	cmd := args[0]
	rest := args[1:]
	var err error
	switch cmd {
	case "scan":
		err = cmdScan(rest)
	case "stats":
		err = cmdStats(rest)
	case "estimate":
		err = cmdEstimate(rest)
	case "show":
		err = cmdShow(rest)
	case "list":
		err = cmdList(rest)
	case "group":
		err = cmdGroup(rest)
	case "actual":
		err = cmdActual(rest)
	case "rules":
		err = cmdRules(rest)
	case "version", "--version", "-v":
		fmt.Println("effort " + Version)
		return exitOK
	case "help", "--help", "-h":
		topic := ""
		if len(rest) > 0 {
			topic = rest[0]
		}
		printHelp(topic)
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "모르는 명령 : %s\n", cmd)
		printHelp("")
		return exitUsage
	}
	if err == nil {
		return exitOK
	}
	var ce *codedError
	if errors.As(err, &ce) {
		fmt.Fprintln(os.Stderr, ce.msg)
		return ce.code
	}
	if errors.Is(err, flag.ErrHelp) {
		return exitUsage
	}
	if errors.Is(err, store.ErrNoCache) {
		fmt.Fprintln(os.Stderr, "캐시가 없습니다. 먼저 `effort scan` 을 돌리세요.")
		return exitNoData
	}
	fmt.Fprintln(os.Stderr, err.Error())
	return exitRead
}

// openStore 는 파생 저장소를 연다. 두 뿌리가 겹치면 바로 실패한다.
func openStore(home, projects string) (*store.Store, string, error) {
	h, err := paths.Home(home)
	if err != nil {
		return nil, "", fail(exitWrite, "저장소 자리를 못 정했습니다 : %v", err)
	}
	p, err := paths.ProjectsRoot(projects)
	if err != nil {
		return nil, "", fail(exitRead, "읽을 뿌리를 못 정했습니다 : %v", err)
	}
	hj, err := paths.NewJail(h)
	if err != nil {
		return nil, "", fail(exitWrite, "%v", err)
	}
	pj, err := paths.NewJail(p)
	if err != nil {
		return nil, "", fail(exitRead, "%v", err)
	}
	if paths.Overlap(hj, pj) {
		return nil, "", fail(exitUsage, "캐시 자리와 원본 자리가 겹칩니다 (%s / %s)", h, p)
	}
	return store.New(h), p, nil
}

// newFlags 는 모르는 옵션이 오면 바로 실패하는 플래그 묶음을 만든다.
func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return fail(exitUsage, "옵션이 잘못됐습니다 (%s)", fs.Name())
	}
	// Go 의 flag 는 첫 위치 인자에서 읽기를 멈춘다. 뒤에 남은 옵션은 조용히 무시되므로 막는다.
	for _, a := range fs.Args() {
		if len(a) > 1 && a[0] == '-' {
			return fail(exitUsage, "옵션은 명령 바로 뒤에 둡니다 : effort %s %s <인자…>", fs.Name(), a)
		}
	}
	return nil
}
