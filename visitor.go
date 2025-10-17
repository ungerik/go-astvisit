package astvisit

import (
	"go/ast"
)

// Visitor defines the interface for visiting AST nodes in a type-safe manner.
// Each method corresponds to a specific ast.Node type and is called when
// that node type is encountered during traversal.
//
// Methods return bool to control traversal:
//   - true: Continue visiting child nodes
//   - false: Skip visiting child nodes (but continue with siblings)
//
// To implement a visitor, embed VisitorImpl to get default implementations
// for all methods, then override only the methods you need:
//
//	type MyVisitor struct {
//	    astvisit.VisitorImpl
//	}
//
//	func (v *MyVisitor) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
//	    // Handle function declarations
//	    return true
//	}
type Visitor interface {
	VisitComment(*ast.Comment, Cursor) bool
	VisitCommentGroup(*ast.CommentGroup, Cursor) bool
	VisitField(*ast.Field, Cursor) bool
	VisitFieldList(*ast.FieldList, Cursor) bool
	VisitBadExpr(*ast.BadExpr, Cursor) bool
	VisitIdent(*ast.Ident, Cursor) bool
	VisitBasicLit(*ast.BasicLit, Cursor) bool
	VisitEllipsis(*ast.Ellipsis, Cursor) bool
	VisitFuncLit(*ast.FuncLit, Cursor) bool
	VisitCompositeLit(*ast.CompositeLit, Cursor) bool
	VisitParenExpr(*ast.ParenExpr, Cursor) bool
	VisitSelectorExpr(*ast.SelectorExpr, Cursor) bool
	VisitIndexExpr(*ast.IndexExpr, Cursor) bool
	VisitIndexListExpr(*ast.IndexListExpr, Cursor) bool
	VisitSliceExpr(*ast.SliceExpr, Cursor) bool
	VisitTypeAssertExpr(*ast.TypeAssertExpr, Cursor) bool
	VisitCallExpr(*ast.CallExpr, Cursor) bool
	VisitStarExpr(*ast.StarExpr, Cursor) bool
	VisitUnaryExpr(*ast.UnaryExpr, Cursor) bool
	VisitBinaryExpr(*ast.BinaryExpr, Cursor) bool
	VisitKeyValueExpr(*ast.KeyValueExpr, Cursor) bool
	VisitArrayType(*ast.ArrayType, Cursor) bool
	VisitStructType(*ast.StructType, Cursor) bool
	VisitFuncType(*ast.FuncType, Cursor) bool
	VisitInterfaceType(*ast.InterfaceType, Cursor) bool
	VisitMapType(*ast.MapType, Cursor) bool
	VisitChanType(*ast.ChanType, Cursor) bool
	VisitBadStmt(*ast.BadStmt, Cursor) bool
	VisitDeclStmt(*ast.DeclStmt, Cursor) bool
	VisitEmptyStmt(*ast.EmptyStmt, Cursor) bool
	VisitLabeledStmt(*ast.LabeledStmt, Cursor) bool
	VisitExprStmt(*ast.ExprStmt, Cursor) bool
	VisitSendStmt(*ast.SendStmt, Cursor) bool
	VisitIncDecStmt(*ast.IncDecStmt, Cursor) bool
	VisitAssignStmt(*ast.AssignStmt, Cursor) bool
	VisitGoStmt(*ast.GoStmt, Cursor) bool
	VisitDeferStmt(*ast.DeferStmt, Cursor) bool
	VisitReturnStmt(*ast.ReturnStmt, Cursor) bool
	VisitBranchStmt(*ast.BranchStmt, Cursor) bool
	VisitBlockStmt(*ast.BlockStmt, Cursor) bool
	VisitIfStmt(*ast.IfStmt, Cursor) bool
	VisitCaseClause(*ast.CaseClause, Cursor) bool
	VisitSwitchStmt(*ast.SwitchStmt, Cursor) bool
	VisitTypeSwitchStmt(*ast.TypeSwitchStmt, Cursor) bool
	VisitCommClause(*ast.CommClause, Cursor) bool
	VisitSelectStmt(*ast.SelectStmt, Cursor) bool
	VisitForStmt(*ast.ForStmt, Cursor) bool
	VisitRangeStmt(*ast.RangeStmt, Cursor) bool
	VisitImportSpec(*ast.ImportSpec, Cursor) bool
	VisitValueSpec(*ast.ValueSpec, Cursor) bool
	VisitTypeSpec(*ast.TypeSpec, Cursor) bool
	VisitBadDecl(*ast.BadDecl, Cursor) bool
	VisitGenDecl(*ast.GenDecl, Cursor) bool
	VisitFuncDecl(*ast.FuncDecl, Cursor) bool
	VisitFile(*ast.File, Cursor) bool
	VisitPackage(*ast.Package, Cursor) bool
}
