package interpreter

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/go-task/task/v3/taskfile"
)

var intByName = make(map[string]func() Interpreter)

type Interpreter interface {
	CreateOpts(tf *taskfile.Taskfile, cmd *taskfile.Cmd, t *taskfile.Task, stdin io.Reader, stdout, stderr io.Writer) any
	EvalExpr(ctx context.Context, opts any) error
}

func Register(name string, fac func() Interpreter) {
	intByName[name] = fac
}

func GetInstance(name string) (Interpreter, error) {
	n, _, _ := strings.Cut(name, " ")
	if fac, ok := intByName[n]; ok {
		return fac(), nil
	}
	return nil, errors.New("no such interpreter")
}
