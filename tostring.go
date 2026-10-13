package astvisit

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"
)

// ExprString returns the Go source of an expression on a single line,
// for example to compare expressions or to generate code.
// The result parses back to the same expression.
//
// The expression is formatted like gofmt formats it on a single line,
// except that binary operators always have spaces around them
// and that struct and interface types with several fields or methods
// are written on one line separated by semicolons.
// A function literal is formatted by gofmt, which keeps a short body
// on the line and writes a longer one with a statement per line.
// Raw string literals keep their newlines and comments are not included.
// An expression type without a case, like one that go/ast adds later,
// is written as "(ast: <type>)" like types.ExprString does.
//
// Examples:
//   - *ast.Ident → "foo"
//   - *ast.SelectorExpr → "pkg.Type"
//   - *ast.ArrayType → "[]int"
//   - *ast.MapType → "map[string]int"
//   - *ast.IndexListExpr → "Generic[T, U]"
//   - *ast.CallExpr → "foo(1, 2)" or "foo(args...)"
//   - *ast.SliceExpr → "arr[1:5]" or "arr[1:5:10]"
//   - *ast.CompositeLit → "[]T{{a, b}, {c, d}}"
//   - *ast.StructType → "struct{ A int; B string }"
//   - *ast.InterfaceType → "interface{}" or "interface{ String() string }"
//   - *ast.FuncLit → "func(x int) int { return x * 2 }"
func ExprString(expr ast.Expr) string {
	return exprString(expr, "")
}

// ExprStringWithExportedNameQualifyer converts an ast.Expr to string,
// adding a qualifier prefix to exported names.
//
// Example:
//
//	// If expr is an exported Ident "Foo" and qualifyer is "pkg"
//	ExprStringWithExportedNameQualifyer(expr, "pkg") // Returns: "pkg.Foo"
//
// Exported names in nested types like func, struct, and interface types
// are qualified too, but not the names of fields, parameters, and methods,
// and no names in function literals.
//
// Without type information the keys of composite literals are qualified
// like any other expression, which is correct for map literals,
// but for struct literals produces invalid Go like "pkg.Foo{pkg.Field: 1}".
func ExprStringWithExportedNameQualifyer(expr ast.Expr, qualifyer string) string {
	return exprString(expr, qualifyer)
}

func exprString(expr ast.Expr, qualifyer string) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case *ast.BadExpr:
		return ""
	case *ast.Ident:
		if e == nil {
			return ""
		}
		if qualifyer != "" && e.IsExported() {
			return qualifyer + "." + e.Name
		}
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X, "") + "." + e.Sel.Name
	case *ast.Ellipsis:
		return "..." + exprString(e.Elt, qualifyer)
	case *ast.BasicLit:
		return e.Value
	case *ast.FuncLit:
		// A function body can't be written on a single line in general,
		// gofmt decides which body fits on the line
		if e.Body != nil {
			if s, err := formatSingleLine(e); err == nil {
				return s
			}
		}
		return fmt.Sprintf("(ast: %T)", expr)
	case *ast.CompositeLit:
		var b strings.Builder
		b.WriteString(exprString(e.Type, qualifyer))
		b.WriteByte('{')
		for i, elem := range e.Elts {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(elem, qualifyer))
		}
		b.WriteByte('}')
		return b.String()
	case *ast.ParenExpr:
		return "(" + exprString(e.X, qualifyer) + ")"
	case *ast.IndexExpr:
		return exprString(e.X, qualifyer) + "[" + exprString(e.Index, qualifyer) + "]"
	case *ast.IndexListExpr:
		var b strings.Builder
		b.WriteString(exprString(e.X, qualifyer))
		b.WriteByte('[')
		for i, index := range e.Indices {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(index, qualifyer))
		}
		b.WriteByte(']')
		return b.String()
	case *ast.SliceExpr:
		var b strings.Builder
		b.WriteString(exprString(e.X, qualifyer))
		b.WriteByte('[')
		if e.Low != nil {
			b.WriteString(exprString(e.Low, qualifyer))
		}
		b.WriteByte(':')
		if e.High != nil {
			b.WriteString(exprString(e.High, qualifyer))
		}
		if e.Slice3 {
			b.WriteByte(':')
			if e.Max != nil {
				b.WriteString(exprString(e.Max, qualifyer))
			}
		}
		b.WriteByte(']')
		return b.String()
	case *ast.TypeAssertExpr:
		if e.Type == nil {
			// Type switch guard
			return exprString(e.X, qualifyer) + ".(type)"
		}
		return exprString(e.X, qualifyer) + ".(" + exprString(e.Type, qualifyer) + ")"
	case *ast.CallExpr:
		var b strings.Builder
		b.WriteString(exprString(e.Fun, qualifyer))
		b.WriteByte('(')
		for i, arg := range e.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(arg, qualifyer))
		}
		if e.Ellipsis.IsValid() {
			b.WriteString("...")
		}
		b.WriteByte(')')
		return b.String()
	case *ast.StarExpr:
		return "*" + exprString(e.X, qualifyer)
	case *ast.UnaryExpr:
		if e.OpPos == e.Pos() {
			return e.Op.String() + exprString(e.X, qualifyer)
		}
		return exprString(e.X, qualifyer) + e.Op.String()
	case *ast.BinaryExpr:
		return exprString(e.X, qualifyer) + " " + e.Op.String() + " " + exprString(e.Y, qualifyer)
	case *ast.KeyValueExpr:
		return exprString(e.Key, qualifyer) + ": " + exprString(e.Value, qualifyer)
	case *ast.ArrayType:
		return "[" + exprString(e.Len, qualifyer) + "]" + exprString(e.Elt, qualifyer)
	case *ast.StructType:
		if e.Fields.NumFields() == 0 {
			return "struct{}"
		}
		var b strings.Builder
		b.WriteString("struct{ ")
		for i, field := range e.Fields.List {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(fieldString(field, qualifyer))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.FuncType:
		return "func" + funcTypeString(e, qualifyer)
	case *ast.InterfaceType:
		if e.Methods.NumFields() == 0 {
			return "interface{}"
		}
		var b strings.Builder
		b.WriteString("interface{ ")
		for i, method := range e.Methods.List {
			if i > 0 {
				b.WriteString("; ")
			}
			if funcType, ok := method.Type.(*ast.FuncType); ok && len(method.Names) > 0 {
				b.WriteString(method.Names[0].Name)
				b.WriteString(funcTypeString(funcType, qualifyer))
			} else {
				// Embedded type or type constraint
				b.WriteString(exprString(method.Type, qualifyer))
			}
		}
		b.WriteString(" }")
		return b.String()
	case *ast.MapType:
		return "map[" + exprString(e.Key, qualifyer) + "]" + exprString(e.Value, qualifyer)
	case *ast.ChanType:
		switch e.Dir {
		case ast.RECV:
			return "<-chan " + exprString(e.Value, qualifyer)
		case ast.SEND:
			return "chan<- " + exprString(e.Value, qualifyer)
		default:
			// Without parentheses chan <-chan T would parse as chan<- chan T
			if value, ok := e.Value.(*ast.ChanType); ok && value.Dir == ast.RECV {
				return "chan (" + exprString(e.Value, qualifyer) + ")"
			}
			return "chan " + exprString(e.Value, qualifyer)
		}
	default:
		// Like types.ExprString, go/printer would panic for an unknown type
		return fmt.Sprintf("(ast: %T)", expr)
	}
}

// formatSingleLine returns node formatted by gofmt
// as if all of its source was written on a single line.
// The FileSet that the positions of node are from is unknown,
// a FileSet with a single line file that covers the positions replaces it.
// Without line numbers gofmt would put every statement on a line of its own.
func formatSingleLine(node ast.Node) (string, error) {
	fset := token.NewFileSet()
	fset.AddFile("", fset.Base(), int(node.End()))
	var b strings.Builder
	err := format.Node(&b, fset, node)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// FieldString returns the names, type, and tag of a struct field,
// parameter, or result, for example "x, y int" or "Name string `json:\"name\"`".
func FieldString(field *ast.Field) string {
	return fieldString(field, "")
}

func fieldString(field *ast.Field, qualifyer string) string {
	var b strings.Builder
	if len(field.Names) > 0 {
		for i, name := range field.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(ExprString(name))
		}
		b.WriteByte(' ')
	}
	b.WriteString(exprString(field.Type, qualifyer))
	if field.Tag != nil {
		b.WriteByte(' ')
		b.WriteString(ExprString(field.Tag))
	}
	return b.String()
}

// FuncTypeString returns the parameters and results of a function type
// without the func keyword, for example "(x int, y string) error".
func FuncTypeString(funcType *ast.FuncType) string {
	return funcTypeString(funcType, "")
}

func funcTypeString(funcType *ast.FuncType, qualifyer string) string {
	var b strings.Builder
	b.WriteByte('(')
	for fieldIndex, field := range funcType.Params.List {
		if fieldIndex > 0 {
			b.WriteString(", ")
		}
		if len(field.Names) > 0 {
			for i, name := range field.Names {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(name.Name)
			}
			b.WriteByte(' ')
		}
		b.WriteString(exprString(field.Type, qualifyer))
	}
	b.WriteByte(')')
	if funcType.Results == nil || len(funcType.Results.List) == 0 {
		return b.String()
	}
	b.WriteByte(' ')
	if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
		b.WriteString(exprString(funcType.Results.List[0].Type, qualifyer))
		return b.String()
	}
	b.WriteByte('(')
	for fieldIndex, field := range funcType.Results.List {
		if fieldIndex > 0 {
			b.WriteString(", ")
		}
		if len(field.Names) > 0 {
			for i, name := range field.Names {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(name.Name)
			}
			b.WriteByte(' ')
		}
		b.WriteString(exprString(field.Type, qualifyer))
	}
	b.WriteByte(')')
	return b.String()
}

// TypeExprNameQualifyers sets all type name qualifyers (package names) at the passed map
// that are used in any ast.SelectorExpr recursively found
// within the passed type expression.
func TypeExprNameQualifyers(expr ast.Expr, qualifyers map[string]struct{}) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifyers[ExprString(sel.X)] = struct{}{}
		return false
	})
}
