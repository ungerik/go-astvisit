package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
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
