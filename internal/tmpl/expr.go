package tmpl

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/vm"
	"github.com/expr-lang/expr/vm/runtime"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// arithmetic makes / and ** (^) fail where expr would give NaN or ±Inf, a division by zero above all,
// so that a comparison with it does not silently hold or fail.
// LIMIT: +, - and * past ±1.8e308 still give ±Inf, and whole numbers past ±9.2e18 wrap around. Check
// their results too if a template needs such numbers.
var arithmetic = []expr.Option{
	finite("_divide", "/", runtime.Divide), expr.Operator("/", "_divide"),
	// expr matches an operator as written, and ^ is ** written otherwise.
	finite("_power", "**", runtime.Exponent), expr.Operator("**", "_power"), expr.Operator("^", "_power"),
}

var errNotFinite = errors.New("is not a finite number")

func finite(name, op string, f func(a, b any) float64) expr.Option {
	return expr.Function(name, func(p ...any) (any, error) {
		r := f(p[0], p[1])
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return nil, fmt.Errorf("%v %s %v %w", p[0], op, p[1], errNotFinite)
		}
		return r, nil
	}, new(func(a, b any) float64))
}

// compile compiles the expression src for the variables of env, a struct whose fields are tagged
// with their names.
func compile(src string, env any, opts ...expr.Option) (*vm.Program, error) {
	p, err := expr.Compile(src, slices.Concat([]expr.Option{expr.Env(env)}, arithmetic, opts)...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src, err)
	}
	return p, nil
}

// compileWritten compiles src, an expression as YAML read it at at in file, for env with opts. rest is
// the text of the YAML comment after it on its last line, or "". When src fails and src with the comment put
// back compiles, a # in the expression began the comment, and the mistake says to quote it.
func compileWritten(
	file string, at yamlpos.Pos, src, rest string, env any, opts ...expr.Option,
) (*vm.Program, *yamlpos.Error) {
	p, err := compile(src, env, opts...)
	if err == nil {
		return p, nil
	}
	var detail any = brief(err)
	// LIMIT: the hint needs the expression with its comment put back to compile, so one cut short that
	// has another mistake too gets none. Compare where the two fail if that hides a cut.
	if rest != "" {
		if _, e := compile(src+" #"+rest, env, opts...); e == nil {
			detail = yamlpos.HintFirst("yaml_hint_expr_comment", brief(err))
		}
	}
	return nil, yamlpos.New(file, at, "tmpl_expr", src, detail)
}

// brief words an error of expr on one line, without the snippet it draws under the message.
func brief(err error) string {
	if fe := (*file.Error)(nil); errors.As(err, &fe) {
		return fe.Message
	}
	return err.Error()
}
