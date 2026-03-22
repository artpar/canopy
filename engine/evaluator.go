package engine

import (
	"crypto/rand"
	"fmt"
	"jview/protocol"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// FuncDef stores a user-defined function registered via defineFunction.
type FuncDef struct {
	Name   string
	Params []string
	Body   any
}

// Evaluator handles FunctionCall evaluation against a DataModel.
type Evaluator struct {
	dm          *DataModel
	FFI         *FFIRegistry
	customFuncs map[string]*FuncDef
}

func NewEvaluator(dm *DataModel) *Evaluator {
	return &Evaluator{dm: dm, customFuncs: make(map[string]*FuncDef)}
}

type evalFn func(e *Evaluator, args []any) (any, error)

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
		"append":      (*Evaluator).fnAppend,
		"removeLast":  (*Evaluator).fnRemoveLast,
		"slice":       (*Evaluator).fnSlice,
		"filter":           (*Evaluator).fnFilter,
		"filterContains":   (*Evaluator).fnFilterContains,
		"find":             (*Evaluator).fnFind,
		"getField":         (*Evaluator).fnGetField,
		"substringAfter":      (*Evaluator).fnSubstringAfter,
		"replace":             (*Evaluator).fnReplace,
		"sort":                (*Evaluator).fnSort,
		"remove":              (*Evaluator).fnRemove,
		"updateItem":          (*Evaluator).fnUpdateItem,
		"lessThan":            (*Evaluator).fnLessThan,
		"formatDateRelative":  (*Evaluator).fnFormatDateRelative,
		"now":                 (*Evaluator).fnNow,
		"setField":            (*Evaluator).fnSetField,
		"countWhere":          (*Evaluator).fnCountWhere,
		"insertAt":            (*Evaluator).fnInsertAt,
		"filterContainsAny":   (*Evaluator).fnFilterContainsAny,
		"uuid":                (*Evaluator).fnUUID,
		"appendToTree":        (*Evaluator).fnAppendToTree,
		"removeFromTree":      (*Evaluator).fnRemoveFromTree,
		"shell":               (*Evaluator).fnShell,
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
func (e *Evaluator) Eval(name string, args []any) (any, error) {
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

	// Check custom (user-defined) functions
	if def, ok := e.customFuncs[name]; ok {
		if len(args) != len(def.Params) {
			return nil, fmt.Errorf("%s: expected %d args, got %d", name, len(def.Params), len(args))
		}
		resolved, err := e.resolveArgs(args)
		if err != nil {
			return nil, err
		}
		paramMap := make(map[string]any, len(def.Params))
		for i, p := range def.Params {
			paramMap[p] = resolved[i]
		}
		substituted := substituteParams(deepCopyJSON(def.Body), paramMap)
		return e.resolveArg(substituted)
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
func (e *Evaluator) resolveArgs(args []any) ([]any, error) {
	resolved := make([]any, len(args))
	for i, arg := range args {
		val, err := e.resolveArg(arg)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		resolved[i] = val
	}
	return resolved, nil
}

func (e *Evaluator) resolveArg(arg any) (any, error) {
	switch v := arg.(type) {
	case string, float64, bool:
		return v, nil
	case map[string]any:
		if path, ok := v["path"].(string); ok {
			val, found := e.dm.Get(path)
			if !found {
				return "", nil
			}
			return val, nil
		}
		if fc, ok := v["functionCall"].(map[string]any); ok {
			name, _ := fc["name"].(string)
			rawArgs, _ := fc["args"].([]any)
			return e.Eval(name, rawArgs)
		}
		// Plain object — recursively resolve each value
		resolved := make(map[string]any, len(v))
		for key, val := range v {
			r, err := e.resolveArg(val)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", key, err)
			}
			resolved[key] = r
		}
		return resolved, nil
	case []any:
		resolved := make([]any, len(v))
		for i, val := range v {
			r, err := e.resolveArg(val)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			resolved[i] = r
		}
		return resolved, nil
	default:
		return arg, nil
	}
}

// PathsInArgs returns all data model paths referenced in the args tree.
func PathsInArgs(args []any) []string {
	var paths []string
	for _, arg := range args {
		pathsInArg(arg, &paths)
	}
	return paths
}

func pathsInArg(arg any, paths *[]string) {
	m, ok := arg.(map[string]any)
	if !ok {
		return
	}
	if path, ok := m["path"].(string); ok {
		*paths = append(*paths, path)
	}
	if fc, ok := m["functionCall"].(map[string]any); ok {
		if nestedArgs, ok := fc["args"].([]any); ok {
			for _, a := range nestedArgs {
				pathsInArg(a, paths)
			}
		}
	}
}

func toString(v any) string {
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

func toFloat(v any) (float64, error) {
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

func toBool(v any) (bool, error) {
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

func (e *Evaluator) fnConcat(args []any) (any, error) {
	var b strings.Builder
	for _, a := range args {
		b.WriteString(toString(a))
	}
	return b.String(), nil
}

func (e *Evaluator) fnFormat(args []any) (any, error) {
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

func (e *Evaluator) fnToUpperCase(args []any) (any, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.ToUpper(toString(args[0])), nil
}

func (e *Evaluator) fnToLowerCase(args []any) (any, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.ToLower(toString(args[0])), nil
}

func (e *Evaluator) fnTrim(args []any) (any, error) {
	if len(args) < 1 {
		return "", nil
	}
	return strings.TrimSpace(toString(args[0])), nil
}

func (e *Evaluator) fnSubstring(args []any) (any, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("substring requires at least 2 args")
	}
	s := toString(args[0])
	start, err := toFloat(args[1])
	if err != nil {
		return "", err
	}
	si := max(int(start), 0)
	if si >= len(s) {
		return "", nil
	}
	if len(args) >= 3 {
		end, err := toFloat(args[2])
		if err != nil {
			return "", err
		}
		ei := min(int(end), len(s))
		if ei <= si {
			return "", nil
		}
		return s[si:ei], nil
	}
	return s[si:], nil
}

func (e *Evaluator) fnSubstringAfter(args []any) (any, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("substringAfter requires 2 args (string, delimiter)")
	}
	s := toString(args[0])
	delim := toString(args[1])
	idx := strings.Index(s, delim)
	if idx < 0 {
		return s, nil
	}
	return s[idx+len(delim):], nil
}

func (e *Evaluator) fnLength(args []any) (any, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	// Handle arrays — return element count
	if arr, ok := args[0].([]any); ok {
		return float64(len(arr)), nil
	}
	return float64(len(toString(args[0]))), nil
}

func (e *Evaluator) fnAdd(args []any) (any, error) {
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

func (e *Evaluator) fnSubtract(args []any) (any, error) {
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

func (e *Evaluator) fnMultiply(args []any) (any, error) {
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

func (e *Evaluator) fnDivide(args []any) (any, error) {
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

func (e *Evaluator) fnEquals(args []any) (any, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("equals requires 2 args")
	}
	return toString(args[0]) == toString(args[1]), nil
}

func (e *Evaluator) fnGreaterThan(args []any) (any, error) {
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

func (e *Evaluator) fnNot(args []any) (any, error) {
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
func (e *Evaluator) fnIfLazy(rawArgs []any) (any, error) {
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
func (e *Evaluator) fnOrLazy(rawArgs []any) (any, error) {
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
func (e *Evaluator) fnAndLazy(rawArgs []any) (any, error) {
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

func (e *Evaluator) fnToNumber(args []any) (any, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	f, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	return f, nil
}

func (e *Evaluator) fnToString(args []any) (any, error) {
	if len(args) < 1 {
		return "", nil
	}
	return toString(args[0]), nil
}

func (e *Evaluator) fnCalc(args []any) (any, error) {
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

func (e *Evaluator) fnContains(args []any) (any, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("contains requires 2 args")
	}
	return strings.Contains(toString(args[0]), toString(args[1])), nil
}

func (e *Evaluator) fnNegate(args []any) (any, error) {
	if len(args) < 1 {
		return float64(0), nil
	}
	f, err := toFloat(args[0])
	if err != nil {
		return float64(0), err
	}
	return -f, nil
}

func (e *Evaluator) fnAppend(args []any) (any, error) {
	if len(args) < 2 {
		return []any{}, fmt.Errorf("append requires 2 args (array, element)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{args[1]}, nil
	}
	result := make([]any, len(arr), len(arr)+1)
	copy(result, arr)
	return append(result, args[1]), nil
}

func (e *Evaluator) fnRemoveLast(args []any) (any, error) {
	if len(args) < 1 {
		return []any{}, nil
	}
	arr, ok := args[0].([]any)
	if !ok || len(arr) == 0 {
		return []any{}, nil
	}
	return arr[:len(arr)-1], nil
}

func (e *Evaluator) fnSlice(args []any) (any, error) {
	if len(args) < 2 {
		return []any{}, fmt.Errorf("slice requires at least 2 args (array, start)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	start, err := toFloat(args[1])
	if err != nil {
		return []any{}, err
	}
	si := max(int(start), 0)
	if si >= len(arr) {
		return []any{}, nil
	}
	if len(args) >= 3 {
		end, err := toFloat(args[2])
		if err != nil {
			return []any{}, err
		}
		ei := min(int(end), len(arr))
		if ei <= si {
			return []any{}, nil
		}
		result := make([]any, ei-si)
		copy(result, arr[si:ei])
		return result, nil
	}
	result := make([]any, len(arr)-si)
	copy(result, arr[si:])
	return result, nil
}

func (e *Evaluator) fnFilter(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("filter requires 3 args (array, key, value)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	key := toString(args[1])
	value := toString(args[2])
	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if toString(m[key]) == value {
			result = append(result, item)
		}
	}
	if result == nil {
		return []any{}, nil
	}
	return result, nil
}

func (e *Evaluator) fnFilterContains(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("filterContains requires 3 args (array, key, substring)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	key := toString(args[1])
	sub := strings.ToLower(toString(args[2]))
	if sub == "" {
		// Empty substring matches all
		result := make([]any, len(arr))
		copy(result, arr)
		return result, nil
	}
	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(toString(m[key])), sub) {
			result = append(result, item)
		}
	}
	if result == nil {
		return []any{}, nil
	}
	return result, nil
}

func (e *Evaluator) fnFind(args []any) (any, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("find requires 3 args (array, key, value)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return nil, nil
	}
	key := toString(args[1])
	value := toString(args[2])
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if toString(m[key]) == value {
			return item, nil
		}
	}
	return nil, nil
}

func (e *Evaluator) fnReplace(args []any) (any, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("replace requires 3 args (string, old, new)")
	}
	s := toString(args[0])
	old := toString(args[1])
	newStr := toString(args[2])
	return strings.ReplaceAll(s, old, newStr), nil
}

func (e *Evaluator) fnGetField(args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("getField requires 2 args (object, fieldName)")
	}
	obj, ok := args[0].(map[string]any)
	if !ok {
		return nil, nil
	}
	field := toString(args[1])
	return obj[field], nil
}

func (e *Evaluator) fnSort(args []any) (any, error) {
	if len(args) < 2 {
		return []any{}, fmt.Errorf("sort requires at least 2 args (array, key)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	key := toString(args[1])
	descending := false
	if len(args) >= 3 {
		b, err := toBool(args[2])
		if err == nil {
			descending = b
		}
	}
	result := make([]any, len(arr))
	copy(result, arr)
	sort.SliceStable(result, func(i, j int) bool {
		mi, _ := result[i].(map[string]any)
		mj, _ := result[j].(map[string]any)
		if mi == nil || mj == nil {
			return false
		}
		vi := toString(mi[key])
		vj := toString(mj[key])
		if descending {
			return vi > vj
		}
		return vi < vj
	})
	return result, nil
}

func (e *Evaluator) fnRemove(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("remove requires 3 args (array, key, value)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	key := toString(args[1])
	value := toString(args[2])
	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			result = append(result, item)
			continue
		}
		if toString(m[key]) != value {
			result = append(result, item)
		}
	}
	if result == nil {
		return []any{}, nil
	}
	return result, nil
}

func (e *Evaluator) fnUpdateItem(args []any) (any, error) {
	if len(args) < 5 {
		return []any{}, fmt.Errorf("updateItem requires 5 args (array, idKey, idValue, field, value)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	idKey := toString(args[1])
	idValue := toString(args[2])
	field := toString(args[3])
	newValue := args[4]

	result := make([]any, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			result[i] = item
			continue
		}
		if toString(m[idKey]) == idValue {
			clone := make(map[string]any, len(m))
			for k, v := range m {
				clone[k] = v
			}
			clone[field] = newValue
			result[i] = clone
		} else {
			result[i] = item
		}
	}
	return result, nil
}

func (e *Evaluator) fnLessThan(args []any) (any, error) {
	if len(args) < 2 {
		return false, fmt.Errorf("lessThan requires 2 args")
	}
	a, err := toFloat(args[0])
	if err != nil {
		return false, err
	}
	b, err := toFloat(args[1])
	if err != nil {
		return false, err
	}
	return a < b, nil
}

func (e *Evaluator) fnFormatDateRelative(args []any) (any, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("formatDateRelative requires 1 arg (isoDate)")
	}
	dateStr := toString(args[0])
	if dateStr == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", dateStr)
		if err != nil {
			return dateStr, nil
		}
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	tLocal := t.In(now.Location())
	tDay := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day(), 0, 0, 0, 0, now.Location())

	if tDay.Equal(today) {
		return "Today at " + tLocal.Format("3:04 PM"), nil
	}
	if tDay.Equal(yesterday) {
		return "Yesterday", nil
	}
	if tLocal.Year() == now.Year() {
		return tLocal.Format("Jan 2"), nil
	}
	return tLocal.Format("1/2/06"), nil
}

func (e *Evaluator) fnNow(args []any) (any, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

func (e *Evaluator) fnUUID(args []any) (any, error) {
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		return "", fmt.Errorf("uuid: %w", err)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

func (e *Evaluator) fnSetField(args []any) (any, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("setField requires 3 args (object, key, value)")
	}
	obj, ok := args[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("setField: first arg must be an object")
	}
	key := toString(args[1])
	clone := make(map[string]any, len(obj))
	for k, v := range obj {
		clone[k] = v
	}
	clone[key] = args[2]
	return clone, nil
}

func (e *Evaluator) fnCountWhere(args []any) (any, error) {
	if len(args) < 3 {
		return float64(0), fmt.Errorf("countWhere requires 3 args (array, key, value)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return float64(0), nil
	}
	key := toString(args[1])
	value := toString(args[2])
	count := 0
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if toString(m[key]) == value {
			count++
		}
	}
	return float64(count), nil
}

func (e *Evaluator) fnInsertAt(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("insertAt requires 3 args (array, index, item)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{args[2]}, nil
	}
	idx, err := toFloat(args[1])
	if err != nil {
		return arr, err
	}
	i := int(idx)
	if i < 0 {
		i = 0
	}
	if i > len(arr) {
		i = len(arr)
	}
	result := make([]any, len(arr)+1)
	copy(result, arr[:i])
	result[i] = args[2]
	copy(result[i+1:], arr[i:])
	return result, nil
}

func (e *Evaluator) fnFilterContainsAny(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("filterContainsAny requires 3 args (array, keys, substring)")
	}
	arr, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	// Keys can be a []any of strings or a single string
	var keys []string
	switch k := args[1].(type) {
	case []any:
		for _, v := range k {
			keys = append(keys, toString(v))
		}
	case string:
		keys = []string{k}
	default:
		keys = []string{toString(k)}
	}
	sub := strings.ToLower(toString(args[2]))
	if sub == "" {
		result := make([]any, len(arr))
		copy(result, arr)
		return result, nil
	}
	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range keys {
			if strings.Contains(strings.ToLower(toString(m[key])), sub) {
				result = append(result, item)
				break
			}
		}
	}
	if result == nil {
		return []any{}, nil
	}
	return result, nil
}

func (e *Evaluator) fnAppendToTree(args []any) (any, error) {
	if len(args) < 3 {
		return []any{}, fmt.Errorf("appendToTree requires 3 args (tree, parentId, item)")
	}
	tree, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	parentID := toString(args[1])
	item := args[2]

	// Deep copy the tree to avoid mutation
	result := make([]any, len(tree))
	for i, n := range tree {
		result[i] = deepCopyJSON(n)
	}

	// If parentId is empty, append to root
	if parentID == "" {
		return append(result, item), nil
	}

	// Find parent and append child
	if appendToNode(result, parentID, item) {
		return result, nil
	}
	// Parent not found — append to root as fallback
	return append(result, item), nil
}

// appendToNode recursively searches for a node by ID and appends item to its children.
func appendToNode(tree []any, parentID string, item any) bool {
	for i, node := range tree {
		m, ok := node.(map[string]any)
		if !ok {
			continue
		}
		if toString(m["id"]) == parentID {
			children, _ := m["children"].([]any)
			clone := make(map[string]any, len(m))
			for k, v := range m {
				clone[k] = v
			}
			clone["children"] = append(children, item)
			tree[i] = clone
			return true
		}
		// Recurse into children
		if children, ok := m["children"].([]any); ok && len(children) > 0 {
			if appendToNode(children, parentID, item) {
				return true
			}
		}
	}
	return false
}

func (e *Evaluator) fnRemoveFromTree(args []any) (any, error) {
	if len(args) < 2 {
		return []any{}, fmt.Errorf("removeFromTree requires 2 args (tree, id)")
	}
	tree, ok := args[0].([]any)
	if !ok {
		return []any{}, nil
	}
	targetID := toString(args[1])
	return removeFromNode(tree, targetID), nil
}

func (e *Evaluator) fnShell(args []any) (any, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("shell requires 1 arg (command)")
	}
	cmd := toString(args[0])
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("error: %s\n%s", err, strings.TrimSpace(string(out))), nil
	}
	return strings.TrimSpace(string(out)), nil
}

// removeFromNode recursively removes a node by ID from the tree.
func removeFromNode(tree []any, targetID string) []any {
	var result []any
	for _, node := range tree {
		m, ok := node.(map[string]any)
		if !ok {
			result = append(result, node)
			continue
		}
		if toString(m["id"]) == targetID {
			continue // skip this node
		}
		// Recurse into children
		if children, ok := m["children"].([]any); ok && len(children) > 0 {
			clone := make(map[string]any, len(m))
			for k, v := range m {
				clone[k] = v
			}
			clone["children"] = removeFromNode(children, targetID)
			result = append(result, clone)
		} else {
			result = append(result, node)
		}
	}
	if result == nil {
		return []any{}
	}
	return result
}
