package engine

import (
	"math"
	"testing"
)

func newTestEvaluator() (*Evaluator, *DataModel) {
	dm := NewDataModel()
	return NewEvaluator(dm), dm
}

func TestEvalConcat(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("concat", []interface{}{"hello", " ", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("concat = %q, want 'hello world'", result)
	}
}

func TestEvalConcatWithPath(t *testing.T) {
	eval, dm := newTestEvaluator()
	dm.Set("/name", "Alice")
	result, err := eval.Eval("concat", []interface{}{
		"Hello, ",
		map[string]interface{}{"path": "/name"},
		"!",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello, Alice!" {
		t.Errorf("concat = %q, want 'Hello, Alice!'", result)
	}
}

func TestEvalFormat(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("format", []interface{}{"Hello, {0}! You are {1}.", "Alice", "great"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello, Alice! You are great." {
		t.Errorf("format = %q", result)
	}
}

func TestEvalToUpperCase(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("toUpperCase", []interface{}{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "HELLO" {
		t.Errorf("toUpperCase = %q", result)
	}
}

func TestEvalToLowerCase(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("toLowerCase", []interface{}{"HELLO"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("toLowerCase = %q", result)
	}
}

func TestEvalTrim(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("trim", []interface{}{"  hello  "})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("trim = %q", result)
	}
}

func TestEvalSubstring(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("substring", []interface{}{"hello world", float64(0), float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("substring = %q, want 'hello'", result)
	}
}

func TestEvalSubstringNoEnd(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("substring", []interface{}{"hello world", float64(6)})
	if err != nil {
		t.Fatal(err)
	}
	if result != "world" {
		t.Errorf("substring = %q, want 'world'", result)
	}
}

func TestEvalLength(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("length", []interface{}{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(5) {
		t.Errorf("length = %v, want 5", result)
	}
}

func TestEvalAdd(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("add", []interface{}{float64(2), float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(5) {
		t.Errorf("add = %v, want 5", result)
	}
}

func TestEvalSubtract(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("subtract", []interface{}{float64(10), float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(7) {
		t.Errorf("subtract = %v, want 7", result)
	}
}

func TestEvalMultiply(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("multiply", []interface{}{float64(4), float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(20) {
		t.Errorf("multiply = %v, want 20", result)
	}
}

func TestEvalDivide(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("divide", []interface{}{float64(10), float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(5) {
		t.Errorf("divide = %v, want 5", result)
	}
}

func TestEvalDivideByZero(t *testing.T) {
	eval, _ := newTestEvaluator()
	_, err := eval.Eval("divide", []interface{}{float64(10), float64(0)})
	if err == nil {
		t.Error("expected division by zero error")
	}
}

func TestEvalEquals(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("equals", []interface{}{"hello", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("equals = %v, want true", result)
	}

	result, err = eval.Eval("equals", []interface{}{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("equals = %v, want false", result)
	}
}

func TestEvalGreaterThan(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("greaterThan", []interface{}{float64(5), float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("greaterThan(5,3) = %v, want true", result)
	}

	result, err = eval.Eval("greaterThan", []interface{}{float64(2), float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("greaterThan(2,3) = %v, want false", result)
	}
}

func TestEvalNot(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("not", []interface{}{true})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("not(true) = %v, want false", result)
	}

	result, err = eval.Eval("not", []interface{}{false})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("not(false) = %v, want true", result)
	}
}

func TestEvalNestedFunctionCall(t *testing.T) {
	eval, _ := newTestEvaluator()
	// concat(toUpperCase("hello"), " ", "world")
	result, err := eval.Eval("concat", []interface{}{
		map[string]interface{}{
			"functionCall": map[string]interface{}{
				"name": "toUpperCase",
				"args": []interface{}{"hello"},
			},
		},
		" ",
		"world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "HELLO world" {
		t.Errorf("nested = %q, want 'HELLO world'", result)
	}
}

func TestEvalNestedWithPath(t *testing.T) {
	eval, dm := newTestEvaluator()
	dm.Set("/name", "alice")
	// concat(toUpperCase({path: "/name"}), "!")
	result, err := eval.Eval("concat", []interface{}{
		map[string]interface{}{
			"functionCall": map[string]interface{}{
				"name": "toUpperCase",
				"args": []interface{}{
					map[string]interface{}{"path": "/name"},
				},
			},
		},
		"!",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "ALICE!" {
		t.Errorf("nested = %q, want 'ALICE!'", result)
	}
}

func TestEvalUnknownFunction(t *testing.T) {
	eval, _ := newTestEvaluator()
	_, err := eval.Eval("bogusFunc", []interface{}{"a"})
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestEvalMissingPath(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("concat", []interface{}{
		"Hello, ",
		map[string]interface{}{"path": "/nonexistent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Missing path resolves to empty string
	if result != "Hello, " {
		t.Errorf("missing path = %q, want 'Hello, '", result)
	}
}

func TestPathsInArgs(t *testing.T) {
	args := []interface{}{
		"literal",
		map[string]interface{}{"path": "/name"},
		map[string]interface{}{
			"functionCall": map[string]interface{}{
				"name": "toUpperCase",
				"args": []interface{}{
					map[string]interface{}{"path": "/title"},
				},
			},
		},
	}
	paths := PathsInArgs(args)
	if len(paths) != 2 {
		t.Fatalf("paths = %d, want 2", len(paths))
	}
	if paths[0] != "/name" || paths[1] != "/title" {
		t.Errorf("paths = %v, want [/name, /title]", paths)
	}
}

func TestEvalIf(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("if", []interface{}{true, "yes", "no"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "yes" {
		t.Errorf("if(true) = %v, want 'yes'", result)
	}
	result, err = eval.Eval("if", []interface{}{false, "yes", "no"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "no" {
		t.Errorf("if(false) = %v, want 'no'", result)
	}
}

func TestEvalOr(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("or", []interface{}{false, false, true})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("or(false,false,true) = %v, want true", result)
	}
	result, err = eval.Eval("or", []interface{}{false, false})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("or(false,false) = %v, want false", result)
	}
}

func TestEvalAnd(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("and", []interface{}{true, true, true})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("and(true,true,true) = %v, want true", result)
	}
	result, err = eval.Eval("and", []interface{}{true, false})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("and(true,false) = %v, want false", result)
	}
}

func TestEvalIfLazy(t *testing.T) {
	eval, _ := newTestEvaluator()
	// When condition is true, falseVal is NOT evaluated (contains error-producing calc("",0,0))
	brokenExpr := map[string]interface{}{
		"functionCall": map[string]interface{}{
			"name": "calc",
			"args": []interface{}{"", float64(0), float64(0)},
		},
	}
	result, err := eval.Eval("if", []interface{}{true, "yes", brokenExpr})
	if err != nil {
		t.Fatalf("if(true, 'yes', broken) should not error, got: %v", err)
	}
	if result != "yes" {
		t.Errorf("if(true, 'yes', broken) = %v, want 'yes'", result)
	}

	// When condition is false, trueVal is NOT evaluated
	result, err = eval.Eval("if", []interface{}{false, brokenExpr, "no"})
	if err != nil {
		t.Fatalf("if(false, broken, 'no') should not error, got: %v", err)
	}
	if result != "no" {
		t.Errorf("if(false, broken, 'no') = %v, want 'no'", result)
	}
}

func TestEvalOrLazy(t *testing.T) {
	eval, _ := newTestEvaluator()
	// First arg is true → short-circuit, never evaluate broken second arg
	brokenExpr := map[string]interface{}{
		"functionCall": map[string]interface{}{
			"name": "calc",
			"args": []interface{}{"", float64(0), float64(0)},
		},
	}
	result, err := eval.Eval("or", []interface{}{true, brokenExpr})
	if err != nil {
		t.Fatalf("or(true, broken) should not error, got: %v", err)
	}
	if result != true {
		t.Errorf("or(true, broken) = %v, want true", result)
	}
}

func TestEvalAndLazy(t *testing.T) {
	eval, _ := newTestEvaluator()
	// First arg is false → short-circuit, never evaluate broken second arg
	brokenExpr := map[string]interface{}{
		"functionCall": map[string]interface{}{
			"name": "calc",
			"args": []interface{}{"", float64(0), float64(0)},
		},
	}
	result, err := eval.Eval("and", []interface{}{false, brokenExpr})
	if err != nil {
		t.Fatalf("and(false, broken) should not error, got: %v", err)
	}
	if result != false {
		t.Errorf("and(false, broken) = %v, want false", result)
	}
}

func TestEvalToNumber(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("toNumber", []interface{}{"42"})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(42) {
		t.Errorf("toNumber('42') = %v, want 42", result)
	}
}

func TestEvalToString(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("toString", []interface{}{float64(42)})
	if err != nil {
		t.Fatal(err)
	}
	if result != "42" {
		t.Errorf("toString(42) = %v, want '42'", result)
	}
}

func TestEvalCalc(t *testing.T) {
	eval, _ := newTestEvaluator()
	cases := []struct {
		op   string
		a, b float64
		want float64
	}{
		{"+", 2, 3, 5},
		{"-", 10, 3, 7},
		{"*", 4, 5, 20},
		{"/", 10, 2, 5},
	}
	for _, tc := range cases {
		result, err := eval.Eval("calc", []interface{}{tc.op, tc.a, tc.b})
		if err != nil {
			t.Fatalf("calc(%s, %v, %v): %v", tc.op, tc.a, tc.b, err)
		}
		if result != tc.want {
			t.Errorf("calc(%s, %v, %v) = %v, want %v", tc.op, tc.a, tc.b, result, tc.want)
		}
	}
}

func TestEvalCalcDivZero(t *testing.T) {
	eval, _ := newTestEvaluator()
	_, err := eval.Eval("calc", []interface{}{"/", float64(1), float64(0)})
	if err == nil {
		t.Error("expected division by zero error")
	}
}

func TestEvalContains(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("contains", []interface{}{"hello world", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Errorf("contains('hello world','world') = %v, want true", result)
	}
	result, err = eval.Eval("contains", []interface{}{"hello", "xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Errorf("contains('hello','xyz') = %v, want false", result)
	}
}

func TestEvalNegate(t *testing.T) {
	eval, _ := newTestEvaluator()
	result, err := eval.Eval("negate", []interface{}{float64(42)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(-42) {
		t.Errorf("negate(42) = %v, want -42", result)
	}
	result, err = eval.Eval("negate", []interface{}{float64(-5)})
	if err != nil {
		t.Fatal(err)
	}
	if result != float64(5) {
		t.Errorf("negate(-5) = %v, want 5", result)
	}
}

func TestEvalAddWithPath(t *testing.T) {
	eval, dm := newTestEvaluator()
	dm.Set("/x", float64(10))
	result, err := eval.Eval("add", []interface{}{
		map[string]interface{}{"path": "/x"},
		float64(5),
	})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := result.(float64)
	if !ok || math.Abs(f-15) > 0.001 {
		t.Errorf("add = %v, want 15", result)
	}
}
