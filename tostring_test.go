package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"testing"
)

func TestExprString_SliceExpr(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "simple slice",
			code:     `package test; var _ = arr[1:5]`,
			expected: "arr[1:5]",
		},
		{
			name:     "slice with low only",
			code:     `package test; var _ = arr[2:]`,
			expected: "arr[2:]",
		},
		{
			name:     "slice with high only",
			code:     `package test; var _ = arr[:10]`,
			expected: "arr[:10]",
		},
		{
			name:     "full slice",
			code:     `package test; var _ = arr[:]`,
			expected: "arr[:]",
		},
		{
			name:     "three-index slice",
			code:     `package test; var _ = arr[1:5:10]`,
			expected: "arr[1:5:10]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Get the expression from the var declaration
			decl := file.Decls[0].(*ast.GenDecl)
			spec := decl.Specs[0].(*ast.ValueSpec)
			expr := spec.Values[0]

			result := ExprString(expr)
			if result != tt.expected {
				t.Errorf("ExprString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExprString_CallExpr(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "simple function call",
			code:     `package test; var _ = foo()`,
			expected: "foo()",
		},
		{
			name:     "function call with args",
			code:     `package test; var _ = foo(1, "bar")`,
			expected: `foo(1, "bar")`,
		},
		{
			name:     "function call with ellipsis",
			code:     `package test; var _ = foo(args...)`,
			expected: "foo(args...)",
		},
		{
			name:     "method call",
			code:     `package test; var _ = obj.Method(x, y)`,
			expected: "obj.Method(x, y)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Get the expression from the var declaration
			decl := file.Decls[0].(*ast.GenDecl)
			spec := decl.Specs[0].(*ast.ValueSpec)
			expr := spec.Values[0]

			result := ExprString(expr)
			if result != tt.expected {
				t.Errorf("ExprString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExprString_FuncLit(t *testing.T) {
	code := `package test; var _ = func(x int) string { return "" }`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Get the expression from the var declaration
	decl := file.Decls[0].(*ast.GenDecl)
	spec := decl.Specs[0].(*ast.ValueSpec)
	expr := spec.Values[0]

	result := ExprString(expr)
	expected := "func(x int) string { ... }"
	if result != expected {
		t.Errorf("ExprString() = %q, want %q", result, expected)
	}
}

// ExprString is used to generate code, so the result must be valid Go
// that parses back to the same expression.
func TestExprString_ValidGoSyntax(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{name: "bidirectional channel", expr: `chan int`, expected: "chan int"},
		{name: "send-only channel", expr: `chan<- int`, expected: "chan<- int"},
		{name: "receive-only channel", expr: `<-chan int`, expected: "<-chan int"},
		{name: "channel of receive-only channel", expr: `chan<- <-chan int`, expected: "chan<- <-chan int"},
		{name: "interface with method", expr: `interface{ String() string }`, expected: "interface { String() string }"},
		{name: "interface with embedded type and method", expr: `interface{ io.Reader; Close() error }`, expected: "interface { io.Reader; Close() error }"},
		{name: "type constraint", expr: `interface{ ~int | ~string }`, expected: "interface { ~int | ~string }"},
		{name: "slice literal", expr: `[]int{1, 2, 3}`, expected: "[]int{ 1, 2, 3 }"},
		{name: "map literal", expr: `map[string]int{"a": 1, "b": 2}`, expected: `map[string]int{ "a": 1, "b": 2 }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}
			result := ExprString(expr)
			if result != tt.expected {
				t.Errorf("ExprString() = %q, want %q", result, tt.expected)
			}
			reparsed, err := parser.ParseExpr(result)
			if err != nil {
				t.Fatalf("ExprString() result %q is not valid Go: %v", result, err)
			}
			if again := ExprString(reparsed); again != result {
				t.Errorf("ExprString() of reparsed result = %q, want %q", again, result)
			}
		})
	}
}

// Channel types built in code have no positions,
// the direction must be taken from ChanType.Dir.
func TestExprString_SynthesizedChanType(t *testing.T) {
	for dir, expected := range map[ast.ChanDir]string{
		ast.SEND:            "chan<- int",
		ast.RECV:            "<-chan int",
		ast.SEND | ast.RECV: "chan int",
	} {
		result := ExprString(&ast.ChanType{Dir: dir, Value: ast.NewIdent("int")})
		if result != expected {
			t.Errorf("ExprString() = %q, want %q", result, expected)
		}
	}
}

// A synthesized bidirectional channel of receive-only channels has no ParenExpr.
// Without parentheses its string would parse as a send-only channel of channels.
func TestExprString_SynthesizedNestedChanType(t *testing.T) {
	expr := &ast.ChanType{Dir: ast.SEND | ast.RECV, Value: &ast.ChanType{Dir: ast.RECV, Value: ast.NewIdent("int")}}
	result := ExprString(expr)
	if result != "chan (<-chan int)" {
		t.Errorf("ExprString() = %q, want %q", result, "chan (<-chan int)")
	}
	reparsed, err := parser.ParseExpr(result)
	if err != nil {
		t.Fatalf("Failed to parse %q: %v", result, err)
	}
	if dir := reparsed.(*ast.ChanType).Dir; dir != ast.SEND|ast.RECV {
		t.Errorf("Reparsed channel direction = %v, want bidirectional", dir)
	}
}

func TestExprString_TypeSwitchGuard(t *testing.T) {
	code := `package test; func f(x any) { switch x.(type) {} }`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	typeSwitch := file.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.TypeSwitchStmt)
	expr := typeSwitch.Assign.(*ast.ExprStmt).X

	result := ExprString(expr)
	if result != "x.(type)" {
		t.Errorf("ExprString() = %q, want %q", result, "x.(type)")
	}
}

// Code generated in another package must refer to the exported types
// of the original package with a qualifier, also within nested types.
// Names of fields, parameters, and methods must not be qualified.
func TestExprStringWithExportedNameQualifyer_NestedTypes(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{expr: `[]Foo`, expected: "[]pkg.Foo"},
		{expr: `func(f Foo) Bar`, expected: "func(f pkg.Foo) pkg.Bar"},
		{expr: `func(Foo) (b Bar, err error)`, expected: "func(pkg.Foo) (b pkg.Bar, err error)"},
		{expr: `struct{ Field Foo }`, expected: "struct { Field pkg.Foo }"},
		{expr: `struct{ Foo }`, expected: "struct { pkg.Foo }"},
		{expr: `interface{ Method(Foo) Bar }`, expected: "interface { Method(pkg.Foo) pkg.Bar }"},
		{expr: `other.Foo`, expected: "other.Foo"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}
			result := ExprStringWithExportedNameQualifyer(expr, "pkg")
			if result != tt.expected {
				t.Errorf("ExprStringWithExportedNameQualifyer() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TypeExprNameQualifyers is used to find the imports needed by a type,
// it must handle every type expression without panicking.
func TestTypeExprNameQualifyers(t *testing.T) {
	tests := []struct {
		expr     string
		expected []string
	}{
		{expr: `interface{}`, expected: nil},
		{expr: `map[string]interface{ io.Reader }`, expected: []string{"io"}},
		{expr: `interface{ Method(a.A) b.B }`, expected: []string{"a", "b"}},
		{expr: `interface{ ~int | big.Int }`, expected: []string{"big"}},
		{expr: `list.List[elem.T]`, expected: []string{"elem", "list"}},
		{expr: `maps.Map[key.K, value.V]`, expected: []string{"key", "maps", "value"}},
		{expr: `(*pkg.T)`, expected: []string{"pkg"}},
		{expr: `[size.N]int`, expected: []string{"size"}},
		{expr: `func(a.A, ...b.B) (c.C, error)`, expected: []string{"a", "b", "c"}},
		{expr: `chan<- *x.Y`, expected: []string{"x"}},
		{expr: `[]pkg.T{{Field: 1}, other.Value}`, expected: []string{"other", "pkg"}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}
			qualifyers := make(map[string]struct{})
			TypeExprNameQualifyers(expr, qualifyers)
			result := slices.Sorted(maps.Keys(qualifyers))
			if !slices.Equal(result, tt.expected) {
				t.Errorf("TypeExprNameQualifyers() = %v, want %v", result, tt.expected)
			}
		})
	}
}
