package exprext

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/go-task/task/v3"
	"github.com/go-task/task/v3/internal/execext"
	"github.com/go-task/task/v3/interpreter"
	"github.com/go-task/task/v3/taskfile"
	"mvdan.cc/sh/v3/shell"
	"mvdan.cc/sh/v3/syntax"
)

func init() {
	interpreter.Register("expr", func() interpreter.Interpreter { return &ExprInterpreter{} })
}

type ExprOptions struct {
	Cmd         *taskfile.Cmd
	Task        *taskfile.Task
	Expr        string
	Env         map[string]any
	Result      any
	IgnoreError bool
	runOpts     *execext.RunCommandOptions
}

type ExprInterpreter struct{}

var _ interpreter.Interpreter = (*ExprInterpreter)(nil)

func (i ExprInterpreter) CreateOpts(tf *taskfile.Taskfile, cmd *taskfile.Cmd, t *taskfile.Task, stdin io.Reader, stdout, stderr io.Writer) any {
	opts := task.ShInterpreter{}.CreateOpts(tf, cmd, t, stdin, stdout, stderr).(*execext.RunCommandOptions)
	var e map[string]any = make(map[string]any)
	for _, k := range t.Env.Keys() {
		e[k] = t.Env.ToCacheMap()[k]
		opts.Env = append(opts.Env, fmt.Sprintf("%s=%s", k, t.Env.ToCacheMap()[k]))
	}
	return &ExprOptions{
		Cmd:     cmd,
		Task:    t,
		Expr:    cmd.Cmd,
		Env:     e,
		runOpts: opts,
	}
}

func (i ExprInterpreter) EvalExpr(ctx context.Context, opts any) error {
	return EvalExpr(ctx, opts.(*ExprOptions))
}

func EvalExpr(ctx context.Context, opts *ExprOptions) error {
	f := opts.Env["HEIMDALL_FILE"].(string)
	ns, err := shell.Fields(f, nil)
	if err != nil {
		return err
	}

	for i := range ns {
		ns[i], err = syntax.Quote(ns[i], syntax.LangAuto)
		if err != nil {
			return err
		}
	}

	opts.runOpts.Command = fmt.Sprintf("heimdall eval --verbose -E expr -e '%s' %v", opts.runOpts.Command, strings.Join(ns, " "))

	b := strings.Builder{}
	opts.runOpts.Stdout, opts.runOpts.Stderr = &b, &b

	err = task.ShInterpreter{}.EvalExpr(ctx, opts.runOpts)
	if len(os.Getenv("HEIMDALL_QUIET")) > 0 || opts.Env["HEIMDALL_QUIET"] != nil {
		return err
	}

	for _, l := range strings.Split(b.String(), "\n") {
		if l == "false" || l == "true" || l == "" {
			// print nothing
		} else if exprOutputTmpl == nil {
			fmt.Println(strings.TrimRight(l, "\r\n"))
		} else {
			data := map[string]any{"line": l, "output": b.String()}
			if tmplErr := exprOutputTmpl.Execute(os.Stdout, data); tmplErr != nil {
				fmt.Fprintln(os.Stderr, tmplErr)
			}
		}
	}
	return err
}

var exprOutputTmpl *template.Template

func init() {
	tmpl := os.Getenv("EXPR_OUTPUT_TEMPLATE")
	if tmpl != "" {
		exprOutputTmpl = template.Must(template.New("exprOutputTemplate").Parse(tmpl))
	}
}
