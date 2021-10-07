package astvisit

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

func Visit(root ast.Node, pre, post Visitor) (result ast.Node) {
	path := make(Path, 0, 16)
	preApply := func(c *astutil.Cursor) bool {
		path.push(c.Name(), c.Index(), c.Node())
		// if c.Node() != nil {
		// 	fmt.Println(path)
		// }
		return visitNode(pre, newCursor(c, path))
	}
	postApply := func(c *astutil.Cursor) bool {
		result := visitNode(post, newCursor(c, path))
		path.pop()
		return result
	}
	return astutil.Apply(root, preApply, postApply)
}

func visitNode(visitor Visitor, cursor Cursor) bool {
	if visitor == nil {
		return true
	}

	switch x := cursor.Node().(type) {
	case nil:
		return true

	case *ast.Comment:
		return visitor.VisitComment(x, cursor)

	case *ast.CommentGroup:
		return visitor.VisitCommentGroup(x, cursor)

	case *ast.Field:
		return visitor.VisitField(x, cursor)

	case *ast.FieldList:
		return visitor.VisitFieldList(x, cursor)

	case *ast.BadExpr:
		return visitor.VisitBadExpr(x, cursor)

	case *ast.Ident:
		return visitor.VisitIdent(x, cursor)

	case *ast.BasicLit:
		return visitor.VisitBasicLit(x, cursor)

	case *ast.Ellipsis:
		return visitor.VisitEllipsis(x, cursor)

	case *ast.FuncLit:
		return visitor.VisitFuncLit(x, cursor)

	case *ast.CompositeLit:
		return visitor.VisitCompositeLit(x, cursor)

	case *ast.ParenExpr:
		return visitor.VisitParenExpr(x, cursor)

	case *ast.SelectorExpr:
		return visitor.VisitSelectorExpr(x, cursor)

	case *ast.IndexExpr:
		return visitor.VisitIndexExpr(x, cursor)

	case *ast.SliceExpr:
		return visitor.VisitSliceExpr(x, cursor)

	case *ast.TypeAssertExpr:
		return visitor.VisitTypeAssertExpr(x, cursor)

	case *ast.CallExpr:
		return visitor.VisitCallExpr(x, cursor)

	case *ast.StarExpr:
		return visitor.VisitStarExpr(x, cursor)

	case *ast.UnaryExpr:
		return visitor.VisitUnaryExpr(x, cursor)

	case *ast.BinaryExpr:
		return visitor.VisitBinaryExpr(x, cursor)

	case *ast.KeyValueExpr:
		return visitor.VisitKeyValueExpr(x, cursor)

	case *ast.ArrayType:
		return visitor.VisitArrayType(x, cursor)

	case *ast.StructType:
		return visitor.VisitStructType(x, cursor)

	case *ast.FuncType:
		return visitor.VisitFuncType(x, cursor)

	case *ast.InterfaceType:
		return visitor.VisitInterfaceType(x, cursor)

	case *ast.MapType:
		return visitor.VisitMapType(x, cursor)

	case *ast.ChanType:
		return visitor.VisitChanType(x, cursor)

	case *ast.BadStmt:
		return visitor.VisitBadStmt(x, cursor)

	case *ast.DeclStmt:
		return visitor.VisitDeclStmt(x, cursor)

	case *ast.EmptyStmt:
		return visitor.VisitEmptyStmt(x, cursor)

	case *ast.LabeledStmt:
		return visitor.VisitLabeledStmt(x, cursor)

	case *ast.ExprStmt:
		return visitor.VisitExprStmt(x, cursor)

	case *ast.SendStmt:
		return visitor.VisitSendStmt(x, cursor)

	case *ast.IncDecStmt:
		return visitor.VisitIncDecStmt(x, cursor)

	case *ast.AssignStmt:
		return visitor.VisitAssignStmt(x, cursor)

	case *ast.GoStmt:
		return visitor.VisitGoStmt(x, cursor)

	case *ast.DeferStmt:
		return visitor.VisitDeferStmt(x, cursor)

	case *ast.ReturnStmt:
		return visitor.VisitReturnStmt(x, cursor)

	case *ast.BranchStmt:
		return visitor.VisitBranchStmt(x, cursor)

	case *ast.BlockStmt:
		return visitor.VisitBlockStmt(x, cursor)

	case *ast.IfStmt:
		return visitor.VisitIfStmt(x, cursor)

	case *ast.CaseClause:
		return visitor.VisitCaseClause(x, cursor)

	case *ast.SwitchStmt:
		return visitor.VisitSwitchStmt(x, cursor)

	case *ast.TypeSwitchStmt:
		return visitor.VisitTypeSwitchStmt(x, cursor)

	case *ast.CommClause:
		return visitor.VisitCommClause(x, cursor)

	case *ast.SelectStmt:
		return visitor.VisitSelectStmt(x, cursor)

	case *ast.ForStmt:
		return visitor.VisitForStmt(x, cursor)

	case *ast.RangeStmt:
		return visitor.VisitRangeStmt(x, cursor)

	case *ast.ImportSpec:
		return visitor.VisitImportSpec(x, cursor)

	case *ast.ValueSpec:
		return visitor.VisitValueSpec(x, cursor)

	case *ast.TypeSpec:
		return visitor.VisitTypeSpec(x, cursor)

	case *ast.BadDecl:
		return visitor.VisitBadDecl(x, cursor)

	case *ast.GenDecl:
		return visitor.VisitGenDecl(x, cursor)

	case *ast.FuncDecl:
		return visitor.VisitFuncDecl(x, cursor)

	case *ast.File:
		return visitor.VisitFile(x, cursor)

	case *ast.Package:
		return visitor.VisitPackage(x, cursor)
	}

	panic(fmt.Sprintf("unknown ast.Node type %T", cursor.Node()))
}
