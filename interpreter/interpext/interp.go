package interpext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-task/task/v3/interpreter"
	_ "github.com/go-task/task/v3/interpreter/exprext"
	"github.com/go-task/task/v3/taskfile"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/shell"
	"mvdan.cc/sh/v3/syntax"
)

// RunCommandOptions is the options for the RunCommand func
type RunCommandOptions struct {
	Command     string
	Dir         string
	Env         []string
	IgnoreError bool
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// ErrNilOptions is returned when a nil options is given
var ErrNilOptions = errors.New("interpext: nil options given")

type Interp struct{}

var _ interpreter.Interpreter = (*Interp)(nil)

func (i Interp) CreateOpts(tf *taskfile.Taskfile, cmd *taskfile.Cmd, t *taskfile.Task, stdin io.Reader, stdout, stderr io.Writer) any {
	return &RunCommandOptions{
		Command: cmd.Cmd,
		Dir:     t.Dir,
		Env:     getEnviron(t),
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}
}

func (i Interp) EvalExpr(ctx context.Context, opts any) error {
	panic("implement me")
}

// RunCommand runs a shell command
func RunCommand(ctx context.Context, opts *RunCommandOptions) error {
	if opts == nil {
		return ErrNilOptions
	}

	environ := opts.Env
	if len(environ) == 0 {
		environ = os.Environ()
	}

	// os.StartProcess(opts.Command)
	r, err := interp.New(
		// interp.Params(params...),
		interp.Env(expand.ListEnviron(environ...)),
		interp.ExecHandler(interp.DefaultExecHandler(15*time.Second)),
		interp.OpenHandler(openHandler),
		interp.StdIO(opts.Stdin, opts.Stdout, opts.Stderr),
		dirOption(opts.Dir),
	)
	if err != nil {
		return err
	}

	parser := syntax.NewParser()

	// Run the user-defined command
	p, err := parser.Parse(strings.NewReader(opts.Command), "")
	if err != nil {
		return err
	}
	return r.Run(ctx, p)
}

// IsExitError returns true the given error is an exit status error
func IsExitError(err error) bool {
	if _, ok := interp.IsExitStatus(err); ok {
		return true
	}
	return false
}

// Expand is a helper to mvdan.cc/shell.Fields that returns the first field
// if available.
func Expand(s string) (string, error) {
	s = filepath.ToSlash(s)
	s = strings.ReplaceAll(s, " ", `\ `)
	fields, err := shell.Fields(s, nil)
	if err != nil {
		return "", err
	}
	if len(fields) > 0 {
		return fields[0], nil
	}
	return "", nil
}

func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}

func dirOption(path string) interp.RunnerOption {
	return func(r *interp.Runner) error {
		err := interp.Dir(path)(r)
		if err == nil {
			return nil
		}

		// If the specified directory doesn't exist, it will be created later.
		// Therefore, even if `interp.Dir` method returns an error, the
		// directory path should be set only when the directory cannot be found.
		if absPath, _ := filepath.Abs(path); absPath != "" {
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				r.Dir = absPath
				return nil
			}
		}

		return err
	}
}

func getEnviron(t *taskfile.Task) []string {
	if t.Env == nil {
		return nil
	}

	environ := os.Environ()

	for k, v := range t.Env.ToCacheMap() {
		str, isString := v.(string)
		if !isString {
			continue
		}

		if _, alreadySet := os.LookupEnv(k); alreadySet {
			continue
		}

		environ = append(environ, fmt.Sprintf("%s=%s", k, str))
	}

	return environ
}
