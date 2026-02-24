package protocol

// FuncMeta describes a single evaluator function for both dispatch and prompt generation.
type FuncMeta struct {
	Name     string // function name (e.g. "concat")
	Args     string // display signature (e.g. "a, b, ...")
	Desc     string // short description for prompt
	Category string // "string", "math", "logic"
	Lazy     bool   // true = args resolved lazily (if, or, and)
}

// FunctionRegistry is the single source of truth for all evaluator functions.
// Both the engine dispatcher and the LLM system prompt are derived from this list.
var FunctionRegistry = []FuncMeta{
	// String functions
	{Name: "concat", Args: "a, b, ...", Desc: "concatenate values as strings", Category: "string"},
	{Name: "toString", Args: "val", Desc: "convert to string", Category: "string"},
	{Name: "toUpperCase", Args: "s", Desc: "uppercase", Category: "string"},
	{Name: "toLowerCase", Args: "s", Desc: "lowercase", Category: "string"},
	{Name: "trim", Args: "s", Desc: "strip whitespace", Category: "string"},
	{Name: "substring", Args: "s, start, end?", Desc: "extract substring", Category: "string"},
	{Name: "length", Args: "s", Desc: "string length", Category: "string"},
	{Name: "format", Args: "template, arg0, arg1, ...", Desc: "replace {0}, {1}, etc. in template", Category: "string"},
	{Name: "contains", Args: "s, sub", Desc: "true if s contains sub", Category: "string"},

	// Math functions
	{Name: "add", Args: "a, b", Desc: "addition", Category: "math"},
	{Name: "subtract", Args: "a, b", Desc: "subtraction", Category: "math"},
	{Name: "multiply", Args: "a, b", Desc: "multiplication", Category: "math"},
	{Name: "divide", Args: "a, b", Desc: "division", Category: "math"},
	{Name: "calc", Args: "op, left, right", Desc: `op is "+", "-", "*", or "/"`, Category: "math"},
	{Name: "toNumber", Args: "s", Desc: "convert string to number", Category: "math"},
	{Name: "negate", Args: "n", Desc: "negate a number", Category: "math"},

	// Logic functions
	{Name: "if", Args: "condition, trueVal, falseVal", Desc: "conditional (lazy: only evaluates chosen branch)", Category: "logic", Lazy: true},
	{Name: "equals", Args: "a, b", Desc: "string equality", Category: "logic"},
	{Name: "greaterThan", Args: "a, b", Desc: "numeric comparison", Category: "logic"},
	{Name: "not", Args: "val", Desc: "boolean negation", Category: "logic"},
	{Name: "or", Args: "a, b, ...", Desc: "short-circuit OR", Category: "logic", Lazy: true},
	{Name: "and", Args: "a, b, ...", Desc: "short-circuit AND", Category: "logic", Lazy: true},
}
