package astvisit

import (
	"fmt"
	"go/ast"
	"strings"
)

func ExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case nil:
		return ""
	case *ast.BadExpr:
		return ""
	case *ast.Ident:
		return e.Name
	case *ast.Ellipsis:
		return "..." + ExprString(e.Elt)
	case *ast.BasicLit:
		return e.Value
	case *ast.FuncLit:
		return "func" + FuncTypeString(e.Type) + "{ TODO ast.FuncLit.Body }"
	case *ast.CompositeLit:
		var b strings.Builder
		b.WriteString(ExprString(e.Type))
		b.WriteString("{ ")
		for i, elem := range e.Elts {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(ExprString(elem))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.ParenExpr:
		return "(" + ExprString(e.X) + ")"
	case *ast.SelectorExpr:
		return ExprString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		return ExprString(e.X) + "[" + ExprString(e.Index) + "]"
	case *ast.SliceExpr:
		return "TODO ast.SliceExpr"
	case *ast.TypeAssertExpr:
		return ExprString(e.X) + ".(" + ExprString(e.Type) + ")"
	case *ast.CallExpr:
		return "TODO ast.CallExpr"
	case *ast.StarExpr:
		return "*" + ExprString(e.X)
	case *ast.UnaryExpr:
		return ExprString(e.X) + e.Op.String()
	case *ast.BinaryExpr:
		return ExprString(e.X) + " " + e.Op.String() + " " + ExprString(e.Y)
	case *ast.KeyValueExpr:
		return ExprString(e.Key) + ": " + ExprString(e.Value)
	case *ast.ArrayType:
		return "[" + ExprString(e.Len) + "]" + ExprString(e.Elt)
	case *ast.StructType:
		var b strings.Builder
		b.WriteString("struct {")
		b.WriteString("{ ")
		for i, field := range e.Fields.List {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(ExprString(field))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.FuncType:
		return "func" + FuncTypeString(e)
	case *ast.InterfaceType:
		var b strings.Builder
		b.WriteString("interface {")
		b.WriteString("{ ")
		for i, field := range e.Methods.List {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(ExprString(field))
		}
		b.WriteString(" }")
		return b.String()
	case *ast.MapType:
		return "map[" + ExprString(e.Key) + "]" + ExprString(e.Value)
	case *ast.ChanType:
		switch {
		case e.Dir == ast.RECV && e.Begin == e.Arrow:
			return "<-chan " + ExprString(e.Value)
		case e.Dir == ast.RECV && e.Begin < e.Arrow:
			return "chan<- " + ExprString(e.Value)
		case e.Dir == ast.SEND && e.Begin == e.Arrow:
			return "->chan " + ExprString(e.Value)
		case e.Dir == ast.SEND && e.Begin < e.Arrow:
			return "chan-> " + ExprString(e.Value)
		default:
			return "chan " + ExprString(e.Value)
		}
	default:
		panic(fmt.Sprintf("TODO %#v", expr))
	}
}

func FuncTypeString(functype *ast.FuncType) string {
	var b strings.Builder
	b.WriteByte('(')
	for fieldIndex, field := range functype.Params.List {
		if fieldIndex > 0 {
			b.WriteString(", ")
		}
		for i, name := range field.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name.Name)
		}
		b.WriteByte(' ')
		b.WriteString(ExprString(field.Type))
	}
	b.WriteByte(')')
	if len(functype.Results.List) == 0 {
		return b.String()
	}
	b.WriteByte(' ')
	if len(functype.Results.List) == 1 && len(functype.Results.List[0].Names) == 0 {
		b.WriteString(ExprString(functype.Results.List[0].Type))
		return b.String()
	}
	b.WriteByte('(')
	for fieldIndex, field := range functype.Results.List {
		if fieldIndex > 0 {
			b.WriteString(", ")
		}
		for i, name := range field.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name.Name)
		}
		b.WriteByte(' ')
		b.WriteString(ExprString(field.Type))
	}
	b.WriteByte(')')
	return b.String()
}
