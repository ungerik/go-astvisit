package astvisit

import (
	"fmt"
	"go/ast"
)

func NodeType(node ast.Node) string {
	switch node.(type) {
	case nil:
		return "<nil>"

	case *ast.Comment:
		return "Comment"

	case *ast.CommentGroup:
		return "CommentGroup"

	case *ast.Field:
		return "Field"

	case *ast.FieldList:
		return "FieldList"

	case *ast.BadExpr:
		return "BadExpr"

	case *ast.Ident:
		return "Ident"

	case *ast.BasicLit:
		return "BasicLit"

	case *ast.Ellipsis:
		return "Ellipsis"

	case *ast.FuncLit:
		return "FuncLit"

	case *ast.CompositeLit:
		return "CompositeLit"

	case *ast.ParenExpr:
		return "ParenExpr"

	case *ast.SelectorExpr:
		return "SelectorExpr"

	case *ast.IndexExpr:
		return "IndexExpr"

	case *ast.IndexListExpr:
		return "IndexListExpr"

	case *ast.SliceExpr:
		return "SliceExpr"

	case *ast.TypeAssertExpr:
		return "TypeAssertExpr"

	case *ast.CallExpr:
		return "CallExpr"

	case *ast.StarExpr:
		return "StarExpr"

	case *ast.UnaryExpr:
		return "UnaryExpr"

	case *ast.BinaryExpr:
		return "BinaryExpr"

	case *ast.KeyValueExpr:
		return "KeyValueExpr"

	case *ast.ArrayType:
		return "ArrayType"

	case *ast.StructType:
		return "StructType"

	case *ast.FuncType:
		return "FuncType"

	case *ast.InterfaceType:
		return "InterfaceType"

	case *ast.MapType:
		return "MapType"

	case *ast.ChanType:
		return "ChanType"

	case *ast.BadStmt:
		return "BadStmt"

	case *ast.DeclStmt:
		return "DeclStmt"

	case *ast.EmptyStmt:
		return "EmptyStmt"

	case *ast.LabeledStmt:
		return "LabeledStmt"

	case *ast.ExprStmt:
		return "ExprStmt"

	case *ast.SendStmt:
		return "SendStmt"

	case *ast.IncDecStmt:
		return "IncDecStmt"

	case *ast.AssignStmt:
		return "AssignStmt"

	case *ast.GoStmt:
		return "GoStmt"

	case *ast.DeferStmt:
		return "DeferStmt"

	case *ast.ReturnStmt:
		return "ReturnStmt"

	case *ast.BranchStmt:
		return "BranchStmt"

	case *ast.BlockStmt:
		return "BlockStmt"

	case *ast.IfStmt:
		return "IfStmt"

	case *ast.CaseClause:
		return "CaseClause"

	case *ast.SwitchStmt:
		return "SwitchStmt"

	case *ast.TypeSwitchStmt:
		return "TypeSwitchStmt"

	case *ast.CommClause:
		return "CommClause"

	case *ast.SelectStmt:
		return "SelectStmt"

	case *ast.ForStmt:
		return "ForStmt"

	case *ast.RangeStmt:
		return "RangeStmt"

	case *ast.ImportSpec:
		return "ImportSpec"

	case *ast.ValueSpec:
		return "ValueSpec"

	case *ast.TypeSpec:
		return "TypeSpec"

	case *ast.BadDecl:
		return "BadDecl"

	case *ast.GenDecl:
		return "GenDecl"

	case *ast.FuncDecl:
		return "FuncDecl"

	case *ast.File:
		return "File"

	case *ast.Package:
		return "Package"
	}

	panic(fmt.Sprintf("unknown ast.Node type %T", node))
}
