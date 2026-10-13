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

// exprValue readies a value of a data file for expr: goccy/go-yaml reads a whole number above 0 as
// uint64, which expr negates by wrapping around (-3 / 2 is 9.2e18), so one that fits becomes an int.
func exprValue(v any) any {
	switch v := v.(type) {
	case uint64:
		if v <= math.MaxInt64 {
			return int(v)
		}
	case map[string]any:
		return exprValues(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = exprValue(item)
		}
		return out
	}
	return v
}

// exprValues readies the values of a data file for expr, as exprValue does.
func exprValues(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = exprValue(v)
	}
	return out
}

// brief words an error of expr on one line, without the snippet it draws under the message.
func brief(err error) string {
	if fe := (*file.Error)(nil); errors.As(err, &fe) {
		return fe.Message
	}
	return err.Error()
}
