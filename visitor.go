package astvisit

import (
	"go/ast"
)

type Visitor interface {
	VisitComment(comment *ast.Comment, cursor Cursor) bool
	VisitCommentGroup(commentGroup *ast.CommentGroup, cursor Cursor) bool
	VisitField(field *ast.Field, cursor Cursor) bool
	VisitFieldList(fieldList *ast.FieldList, cursor Cursor) bool
	VisitBadExpr(badExpr *ast.BadExpr, cursor Cursor) bool
	VisitIdent(ident *ast.Ident, cursor Cursor) bool
	VisitBasicLit(basicLit *ast.BasicLit, cursor Cursor) bool
	VisitEllipsis(ellipsis *ast.Ellipsis, cursor Cursor) bool
	VisitFuncLit(funcLit *ast.FuncLit, cursor Cursor) bool
	VisitCompositeLit(compositeLit *ast.CompositeLit, cursor Cursor) bool
	VisitParenExpr(parenExpr *ast.ParenExpr, cursor Cursor) bool
	VisitSelectorExpr(selectorExpr *ast.SelectorExpr, cursor Cursor) bool
	VisitIndexExpr(indexExpr *ast.IndexExpr, cursor Cursor) bool
	VisitSliceExpr(sliceExpr *ast.SliceExpr, cursor Cursor) bool
	VisitTypeAssertExpr(typeAssertExpr *ast.TypeAssertExpr, cursor Cursor) bool
	VisitCallExpr(callExpr *ast.CallExpr, cursor Cursor) bool
	VisitStarExpr(starExpr *ast.StarExpr, cursor Cursor) bool
	VisitUnaryExpr(unaryExpr *ast.UnaryExpr, cursor Cursor) bool
	VisitBinaryExpr(binaryExpr *ast.BinaryExpr, cursor Cursor) bool
	VisitKeyValueExpr(keyValueExpr *ast.KeyValueExpr, cursor Cursor) bool
	VisitArrayType(arrayType *ast.ArrayType, cursor Cursor) bool
	VisitStructType(structType *ast.StructType, cursor Cursor) bool
	VisitFuncType(funcType *ast.FuncType, cursor Cursor) bool
	VisitInterfaceType(interfaceType *ast.InterfaceType, cursor Cursor) bool
	VisitMapType(mapType *ast.MapType, cursor Cursor) bool
	VisitChanType(chanType *ast.ChanType, cursor Cursor) bool
	VisitBadStmt(badStmt *ast.BadStmt, cursor Cursor) bool
	VisitDeclStmt(declStmt *ast.DeclStmt, cursor Cursor) bool
	VisitEmptyStmt(emptyStmt *ast.EmptyStmt, cursor Cursor) bool
	VisitLabeledStmt(labeledStmt *ast.LabeledStmt, cursor Cursor) bool
	VisitExprStmt(exprStmt *ast.ExprStmt, cursor Cursor) bool
	VisitSendStmt(sendStmt *ast.SendStmt, cursor Cursor) bool
	VisitIncDecStmt(incDecStmt *ast.IncDecStmt, cursor Cursor) bool
	VisitAssignStmt(assignStmt *ast.AssignStmt, cursor Cursor) bool
	VisitGoStmt(goStmt *ast.GoStmt, cursor Cursor) bool
	VisitDeferStmt(deferStmt *ast.DeferStmt, cursor Cursor) bool
	VisitReturnStmt(returnStmt *ast.ReturnStmt, cursor Cursor) bool
	VisitBranchStmt(branchStmt *ast.BranchStmt, cursor Cursor) bool
	VisitBlockStmt(blockStmt *ast.BlockStmt, cursor Cursor) bool
	VisitIfStmt(ifStmt *ast.IfStmt, cursor Cursor) bool
	VisitCaseClause(caseClause *ast.CaseClause, cursor Cursor) bool
	VisitSwitchStmt(switchStmt *ast.SwitchStmt, cursor Cursor) bool
	VisitTypeSwitchStmt(typeSwitchStmt *ast.TypeSwitchStmt, cursor Cursor) bool
	VisitCommClause(commClause *ast.CommClause, cursor Cursor) bool
	VisitSelectStmt(selectStmt *ast.SelectStmt, cursor Cursor) bool
	VisitForStmt(forStmt *ast.ForStmt, cursor Cursor) bool
	VisitRangeStmt(rangeStmt *ast.RangeStmt, cursor Cursor) bool
	VisitImportSpec(importSpec *ast.ImportSpec, cursor Cursor) bool
	VisitValueSpec(valueSpec *ast.ValueSpec, cursor Cursor) bool
	VisitTypeSpec(typeSpec *ast.TypeSpec, cursor Cursor) bool
	VisitBadDecl(badDecl *ast.BadDecl, cursor Cursor) bool
	VisitGenDecl(genDecl *ast.GenDecl, cursor Cursor) bool
	VisitFuncDecl(funcDecl *ast.FuncDecl, cursor Cursor) bool
	VisitFile(file *ast.File, cursor Cursor) bool
	VisitPackage(pkg *ast.Package, cursor Cursor) bool
}
