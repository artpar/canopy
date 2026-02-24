package engine

import (
	"fmt"
	"jview/protocol"
	"math"
	"strings"
)

// Evaluator handles FunctionCall evaluation against a DataModel.
type Evaluator struct {
	dm  *DataModel
	FFI *FFIRegistry
}

func NewEvaluator(dm *DataModel) *Evaluator {
	return &Evaluator{dm: dm}
}

type evalFn func(e *Evaluator, args []interface{}) (interface{}, error)

var dispatchMap map[string]evalFn
var lazySet map[string]bool

func init() {
	lazySet = make(map[string]bool)
	for _, f := range protocol.FunctionRegistry {
		if f.Lazy {
			lazySet[f.Name] = true
		}
	}

	dispatchMap = map[string]evalFn{
		"concat":      (*Evaluator).fnConcat,
		"toString":    (*Evaluator).fnToString,
		"toUpperCase": (*Evaluator).fnToUpperCase,
		"toLowerCase": (*Evaluator).fnToLowerCase,
		"trim":        (*Evaluator).fnTrim,
		"substring":   (*Evaluator).fnSubstring,
		"length":      (*Evaluator).fnLength,
		"format":      (*Evaluator).fnFormat,
		"contains":    (*Evaluator).fnContains,
		"add":         (*Evaluator).fnAdd,
		"subtract":    (*Evaluator).fnSubtract,
		"multiply":    (*Evaluator).fnMultiply,
		"divide":      (*Evaluator).fnDivide,
		"calc":        (*Evaluator).fnCalc,
		"toNumber":    (*Evaluator).fnToNumber,
		"negate":      (*Evaluator).fnNegate,
		"if":          (*Evaluator).fnIfLazy,
		"equals":      (*Evaluator).fnEquals,
		"greaterThan": (*Evaluator).fnGreaterThan,
		"not":         (*Evaluator).fnNot,
		"or":          (*Evaluator).fnOrLazy,
		"and":         (*Evaluator).fnAndLazy,
	}

	// Validate: every registry entry has an impl, and vice versa
	regNames := make(map[string]bool)
	for _, f := range protocol.FunctionRegistry {
		regNames[f.Name] = true
		if _, ok := dispatchMap[f.Name]; !ok {
			panic("evaluator: no impl for registered function " + f.Name)
		}
	}
	for name := range dispatchMap {
		if !regNames[name] {
			panic("evaluator: impl for unregistered function " + name)
		}
	}
}

// Eval evaluates a function call, resolving args recursively.
// Args can be: string, float64, bool literals, map with "path" key, or map with "functionCall" key.
func (e *Evaluator) Eval(name string, args []interface{}) (interface{}, error) {
	fn, ok := dispatchMap[name]
	if ok {
		if lazySet[name] {
			return fn(e, args)
		}
		resolved, err := e.resolveArgs(args)
		if err != nil {
			return nil, err
		}
		return fn(e, resolved)
	}

	// Fallthrough to FFI registry for native functions
	if e.FFI != nil && e.FFI.Has(name) {
		resolved, err := e.resolveArgs(args)
		if err != nil {
			return nil, err
		}
		return e.FFI.Call(name, resolved)
	}

	return nil, fmt.Errorf("unknown function: %s", name)
}

// resolveArgs resolves each argument: literals pass through, path refs look up DataModel,
// nested functionCalls recurse.
func (e *Evaluator) resolveArgs(args []interface{}) ([]interface{}, error) {
	resolved := make([]interface{}, len(args))
	for i, arg := range args {
		val, err := e.resolveArg(arg)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		resolved[i] = val
	}
	return resolved, nil
}

func (e *Evaluator) resolveArg(arg interface{}) (interface{}, error) {
	switch v := arg.(type) {
	case string, float64, bool:
		return v, nil
	case map[string]interface{}:
		if path, ok := v["path"].(string); ok {
			val, found := e.dm.Get(path)
			if !found {
				return "", nil
			}
			return val, nil
		}
		if fc, ok := v["functionCall"].(map[string]interface{}); ok {
			name, _ := fc["name"].(string)
			rawArgs, _ := fc["args"].([]interface{})
			return e.Eval(name, rawArgs)
		}
		return nil, fmt.Errorf("unrecognized object arg: %v", v)
	default:
		return arg, nil
	}
}

// PathsInArgs returns all data model paths referenced in the args tree.
func PathsInArgs(args []interface{}) []string {
	var paths []string
	for _, arg := range args {
		pathsInArg(arg, &paths)
	}
	return paths
}

func pathsInArg(arg interface{}, paths *[]string) {
	m, ok := arg.(map[string]interface{})
	if !ok {
		return
	}
	if path, ok := m["path"].(string); ok {
		*paths = append(*paths, path)
	}
	if fc, ok := m["functionCall"].(map[string]interface{}); ok {
		if nestedArgs, ok := fc["args"].([]interface{}); ok {
			for _, a := range nestedArgs {
				pathsInArg(a, paths)
			}
		}
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == math.Trunc(val) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %T to number", v)
	}
}

func toBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return val == "true" || val == "1", nil
	case float64:
		return val != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

// --- Function implementations ---

func (e *Evaluator) fnConcat(args []interface{}) (interface{}, error) {
	var b strings.Builder
	for _, a := range args {
		b.WriteString(toString(a))
	}
	return b.String(), nil
}

func (e *Evaluator) fnFormat(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("format requires at least 1 arg")
	}
	tmpl := toString(args[0])
	for i := 1; i < len(args); i++ {
		placeholder := fmt.Sprintf("{%d}", i-1)
		tmpl = strings.ReplaceAll(tmpl, placeholder, toString(args[i]))
	}
	return tmpl, nil
}

func (e *Evaluator) fnToUpperCase(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.ToUpper(toString(args[0])), nil
}

func (e *Evaluator) fnToLowerCase(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.ToLower(toString(args[0])), nil
}

func (e *Evaluator) fnTrim(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.TrimSpace(toString(args[0])), nil
}

func (e *Evaluator) fnSubstring(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("substring requires at least 2 args")
	}
	s := toString(args[0])
	start, err := toFloat(args[1])
	if err != nil {
		return "", err
	}
	si := int(start)
	if si < 0 {
		si = 0
	}
	if si >= len(s) {
		return "", nil
	}
	if len(args) >= 3 {
		end, err := toFloat(args[2])
		if err != nil {
			return "", err
		}
		ei := int(end)
		if ei > len(s) {
			ei = len(s)
		}
		if ei <= si {
			return "", nil
		}
		return s[si:ei], nil
	}
	return s[si:], nil
}

func (e *Evaluator) fnLength(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	return float64(len(toString(args[0]))), nil
}

func (e *Evaluator) fnAdd(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return float64(0), fmt.Errorf("add requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return float64(0), err
	}
	return a + b, nil
}

func (e *Evaluator) fnSubtract(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return float64(0), fmt.Errorf("subtract requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return float64(0), err
	}
	return a - b, nil
}

func (e *Evaluator) fnMultiply(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return float64(0), fmt.Errorf("multiply requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return float64(0), err
	}
	return a * b, nil
}

func (e *Evaluator) fnDivide(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return float64(0), fmt.Errorf("divide requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return float64(0), err
	}
	if b == 0 {
		return float64(0), fmt.Errorf("division by zero")
	}
	return a / b, nil
}

func (e *Evaluator) fnEquals(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("equals requires 2 args")
	}
	return toString(args[0]) == toString(args[1]), nil
}

func (e *Evaluator) fnGreaterThan(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("greaterThan requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return false, err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return false, err
	}
	return a > b, nil
}

func (e *Evaluator) fnNot(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return true, nil
	}
	b, err := toBool(args[0])
	if err != nil {
		return false, err
	}
	return !b, nil
}

// fnIfLazy resolves args lazily: only evaluates the chosen branch.
func (e *Evaluator) fnIfLazy(rawArgs []interface{}) (interface{}, error) {
	if len(rawArgs) < 3 {
		return nil, fmt.Errorf("if requires 3 args (condition, trueVal, falseVal)")
	}
	condVal, err := e.resolveArg(rawArgs[0])
	if err != nil {
		return nil, err
	}
	cond, err := toBool(condVal)
	if err != nil {
		return nil, err
	}
	if cond {
		return e.resolveArg(rawArgs[1])
	}
	return e.resolveArg(rawArgs[2])
}

// fnOrLazy short-circuits: returns true on first truthy arg.
func (e *Evaluator) fnOrLazy(rawArgs []interface{}) (interface{}, error) {
	for _, a := range rawArgs {
		val, err := e.resolveArg(a)
		if err != nil {
			return false, err
		}
		b, err := toBool(val)
		if err != nil {
			return false, err
		}
		if b {
			return true, nil
		}
	}
	return false, nil
}

// fnAndLazy short-circuits: returns false on first falsy arg.
func (e *Evaluator) fnAndLazy(rawArgs []interface{}) (interface{}, error) {
	if len(rawArgs) == 0 {
		return true, nil
	}
	for _, a := range rawArgs {
		val, err := e.resolveArg(a)
		if err != nil {
			return false, err
		}
		b, err := toBool(val)
		if err != nil {
			return false, err
		}
		if !b {
			return false, nil
		}
	}
	return true, nil
}

func (e *Evaluator) fnToNumber(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	f, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	return f, nil
}

func (e *Evaluator) fnToString(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return "", nil
	}
	return toString(args[0]), nil
}

func (e *Evaluator) fnCalc(args []interface{}) (interface{}, error) {
	if len(args) < 3 {
		return float64(0), fmt.Errorf("calc requires 3 args (operator, left, right)")
	}
	op := toString(args[0])
	left, err := toFloat(args[1])
	if err != nil {
		return float64(0), err
	}
	right, err := toFloat(args[2])
	if err != nil {
		return float64(0), err
	}
	switch op {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/":
		if right == 0 {
			return float64(0), fmt.Errorf("division by zero")
		}
		return left / right, nil
	default:
		return float64(0), fmt.Errorf("unknown operator: %s", op)
	}
}

func (e *Evaluator) fnContains(args []interface{}) (interface{}, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("contains requires 2 args")
	}
	return strings.Contains(toString(args[0]), toString(args[1])), nil
}

func (e *Evaluator) fnNegate(args []interface{}) (interface{}, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	f, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	return -f, nil
}
