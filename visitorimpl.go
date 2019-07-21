package astvisit

import "go/ast"

type VisitorImpl struct{}

func (VisitorImpl) VisitComment(comment *ast.Comment, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitCommentGroup(commentGroup *ast.CommentGroup, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitField(field *ast.Field, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitFieldList(fieldList *ast.FieldList, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBadExpr(badExpr *ast.BadExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitIdent(ident *ast.Ident, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBasicLit(basicLit *ast.BasicLit, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitEllipsis(ellipsis *ast.Ellipsis, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitFuncLit(funcLit *ast.FuncLit, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitCompositeLit(compositeLit *ast.CompositeLit, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitParenExpr(parenExpr *ast.ParenExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitSelectorExpr(selectorExpr *ast.SelectorExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitIndexExpr(indexExpr *ast.IndexExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitSliceExpr(sliceExpr *ast.SliceExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitTypeAssertExpr(typeAssertExpr *ast.TypeAssertExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitCallExpr(callExpr *ast.CallExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitStarExpr(starExpr *ast.StarExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitUnaryExpr(unaryExpr *ast.UnaryExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBinaryExpr(binaryExpr *ast.BinaryExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitKeyValueExpr(keyValueExpr *ast.KeyValueExpr, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitArrayType(arrayType *ast.ArrayType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitStructType(structType *ast.StructType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitFuncType(funcType *ast.FuncType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitInterfaceType(interfaceType *ast.InterfaceType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitMapType(mapType *ast.MapType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitChanType(chanType *ast.ChanType, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBadStmt(badStmt *ast.BadStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitDeclStmt(declStmt *ast.DeclStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitEmptyStmt(emptyStmt *ast.EmptyStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitLabeledStmt(labeledStmt *ast.LabeledStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitExprStmt(exprStmt *ast.ExprStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitSendStmt(sendStmt *ast.SendStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitIncDecStmt(incDecStmt *ast.IncDecStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitAssignStmt(assignStmt *ast.AssignStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitGoStmt(goStmt *ast.GoStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitDeferStmt(deferStmt *ast.DeferStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitReturnStmt(returnStmt *ast.ReturnStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBranchStmt(branchStmt *ast.BranchStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBlockStmt(blockStmt *ast.BlockStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitIfStmt(ifStmt *ast.IfStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitCaseClause(caseClause *ast.CaseClause, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitSwitchStmt(switchStmt *ast.SwitchStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitTypeSwitchStmt(typeSwitchStmt *ast.TypeSwitchStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitCommClause(commClause *ast.CommClause, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitSelectStmt(selectStmt *ast.SelectStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitForStmt(forStmt *ast.ForStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitRangeStmt(rangeStmt *ast.RangeStmt, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitImportSpec(importSpec *ast.ImportSpec, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitValueSpec(valueSpec *ast.ValueSpec, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitTypeSpec(typeSpec *ast.TypeSpec, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitBadDecl(badDecl *ast.BadDecl, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitGenDecl(genDecl *ast.GenDecl, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitFuncDecl(funcDecl *ast.FuncDecl, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitFile(file *ast.File, cursor Cursor) bool {
	return true
}

func (VisitorImpl) VisitPackage(pkg *ast.Package, cursor Cursor) bool {
	return true
}
