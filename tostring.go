package astvisit

import (
	"fmt"
	"go/ast"
	"strings"
)

// ExprString converts an ast.Expr to its string representation.
// This is useful for debugging or displaying type information.
//
// Examples:
//   - *ast.Ident{Name: "foo"} → "foo"
//   - *ast.SelectorExpr → "pkg.Type"
//   - *ast.ArrayType → "[]int"
//   - *ast.MapType → "map[string]int"
//   - *ast.IndexListExpr → "Generic[T, U]" (Go 1.18+)
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
		return "func" + FuncTypeString(e.Type) + "{ TODO ast.FuncLit.Body }"
	case *ast.CompositeLit:
		if len(e.Elts) == 0 {
			return exprString(e.Type, qualifyer) + "{}"
		}
		var b strings.Builder
		b.WriteString(exprString(e.Type, qualifyer))
		b.WriteString("{ ")
		for i, elem := range e.Elts {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(exprString(elem, qualifyer))
		}
		b.WriteString(" }")
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
		return "TODO ast.SliceExpr"
	case *ast.TypeAssertExpr:
		return exprString(e.X, qualifyer) + ".(" + exprString(e.Type, qualifyer) + ")"
	case *ast.CallExpr:
		return "TODO ast.CallExpr"
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
		b.WriteString("struct { ")
		for i, field := range e.Fields.List {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(FieldString(field))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.FuncType:
		return "func" + FuncTypeString(e)
	case *ast.InterfaceType:
		if e.Methods.NumFields() == 0 {
			return "any" // "interface{}"
		}
		var b strings.Builder
		b.WriteString("interface {")
		for i, method := range e.Methods.List {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(FieldString(method))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.MapType:
		return "map[" + exprString(e.Key, qualifyer) + "]" + exprString(e.Value, qualifyer)
	case *ast.ChanType:
		switch {
		case e.Dir == ast.RECV && e.Begin == e.Arrow:
			return "<-chan " + exprString(e.Value, qualifyer)
		case e.Dir == ast.RECV && e.Begin < e.Arrow:
			return "chan<- " + exprString(e.Value, qualifyer)
		case e.Dir == ast.SEND && e.Begin == e.Arrow:
			return "->chan " + exprString(e.Value, qualifyer)
		case e.Dir == ast.SEND && e.Begin < e.Arrow:
			return "chan-> " + exprString(e.Value, qualifyer)
		default:
			return "chan " + exprString(e.Value, qualifyer)
		}
	default:
		return fmt.Sprintf("TODO %T", expr)
	}
}

func FieldString(field *ast.Field) string {
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
	b.WriteString(ExprString(field.Type))
	if field.Tag != nil {
		b.WriteByte(' ')
		b.WriteString(ExprString(field.Tag))
	}
	return b.String()
}

func FuncTypeString(funcType *ast.FuncType) string {
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
		b.WriteString(ExprString(field.Type))
	}
	b.WriteByte(')')
	if funcType.Results == nil || len(funcType.Results.List) == 0 {
		return b.String()
	}
	b.WriteByte(' ')
	if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
		b.WriteString(ExprString(funcType.Results.List[0].Type))
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
		b.WriteString(ExprString(field.Type))
	}
	b.WriteByte(')')
	return b.String()
}

// TypeExprNameQualifyers sets all type name qualifyers (package names) at the massed map
// that are used in any ast.SelectorExpr recursively found
// within the passed type expression.
func TypeExprNameQualifyers(expr ast.Expr, qualifyers map[string]struct{}) {
	switch e := expr.(type) {
	case *ast.Ident:
		// Unqualified name
	case *ast.SelectorExpr:
		qualifyers[ExprString(e.X)] = struct{}{}
	case *ast.IndexListExpr:
		TypeExprNameQualifyers(e.X, qualifyers)
		for _, index := range e.Indices {
			TypeExprNameQualifyers(index, qualifyers)
		}
	case *ast.StarExpr:
		TypeExprNameQualifyers(e.X, qualifyers)
	case *ast.Ellipsis:
		TypeExprNameQualifyers(e.Elt, qualifyers)
	case *ast.ArrayType:
		TypeExprNameQualifyers(e.Elt, qualifyers)
	case *ast.StructType:
		for _, f := range e.Fields.List {
			TypeExprNameQualifyers(f.Type, qualifyers)
		}
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			TypeExprNameQualifyers(elt, qualifyers)
		}
	case *ast.MapType:
		TypeExprNameQualifyers(e.Key, qualifyers)
		TypeExprNameQualifyers(e.Value, qualifyers)
	case *ast.ChanType:
		TypeExprNameQualifyers(e.Value, qualifyers)
	case *ast.FuncType:
		for _, p := range e.Params.List {
			TypeExprNameQualifyers(p.Type, qualifyers)
		}
		if e.Results != nil {
			for _, r := range e.Results.List {
				TypeExprNameQualifyers(r.Type, qualifyers)
			}
		}
	default:
		panic(fmt.Sprintf("UNSUPPORTED: %#v", expr))
	}
}
