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
		if e == nil {
			return ""
		}
		return e.Name
	case *ast.Ellipsis:
		return "..." + ExprString(e.Elt)
	case *ast.BasicLit:
		return e.Value
	case *ast.FuncLit:
		return "func" + FuncTypeString(e.Type) + "{ TODO ast.FuncLit.Body }"
	case *ast.CompositeLit:
		if len(e.Elts) == 0 {
			return ExprString(e.Type) + "{}"
		}
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
		if e.OpPos == e.Pos() {
			return e.Op.String() + ExprString(e.X)
		}
		return ExprString(e.X) + e.Op.String()
	case *ast.BinaryExpr:
		return ExprString(e.X) + " " + e.Op.String() + " " + ExprString(e.Y)
	case *ast.KeyValueExpr:
		return ExprString(e.Key) + ": " + ExprString(e.Value)
	case *ast.ArrayType:
		return "[" + ExprString(e.Len) + "]" + ExprString(e.Elt)
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
			return "interface{}"
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
