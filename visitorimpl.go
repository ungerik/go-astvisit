package astvisit

import "go/ast"

type VisitorImpl struct{}

func (VisitorImpl) VisitComment(*ast.Comment, Cursor) bool               { return true }
func (VisitorImpl) VisitCommentGroup(*ast.CommentGroup, Cursor) bool     { return true }
func (VisitorImpl) VisitField(*ast.Field, Cursor) bool                   { return true }
func (VisitorImpl) VisitFieldList(*ast.FieldList, Cursor) bool           { return true }
func (VisitorImpl) VisitBadExpr(*ast.BadExpr, Cursor) bool               { return true }
func (VisitorImpl) VisitIdent(*ast.Ident, Cursor) bool                   { return true }
func (VisitorImpl) VisitBasicLit(*ast.BasicLit, Cursor) bool             { return true }
func (VisitorImpl) VisitEllipsis(*ast.Ellipsis, Cursor) bool             { return true }
func (VisitorImpl) VisitFuncLit(*ast.FuncLit, Cursor) bool               { return true }
func (VisitorImpl) VisitCompositeLit(*ast.CompositeLit, Cursor) bool     { return true }
func (VisitorImpl) VisitParenExpr(*ast.ParenExpr, Cursor) bool           { return true }
func (VisitorImpl) VisitSelectorExpr(*ast.SelectorExpr, Cursor) bool     { return true }
func (VisitorImpl) VisitIndexExpr(*ast.IndexExpr, Cursor) bool           { return true }
func (VisitorImpl) VisitIndexListExpr(*ast.IndexListExpr, Cursor) bool   { return true }
func (VisitorImpl) VisitSliceExpr(*ast.SliceExpr, Cursor) bool           { return true }
func (VisitorImpl) VisitTypeAssertExpr(*ast.TypeAssertExpr, Cursor) bool { return true }
func (VisitorImpl) VisitCallExpr(*ast.CallExpr, Cursor) bool             { return true }
func (VisitorImpl) VisitStarExpr(*ast.StarExpr, Cursor) bool             { return true }
func (VisitorImpl) VisitUnaryExpr(*ast.UnaryExpr, Cursor) bool           { return true }
func (VisitorImpl) VisitBinaryExpr(*ast.BinaryExpr, Cursor) bool         { return true }
func (VisitorImpl) VisitKeyValueExpr(*ast.KeyValueExpr, Cursor) bool     { return true }
func (VisitorImpl) VisitArrayType(*ast.ArrayType, Cursor) bool           { return true }
func (VisitorImpl) VisitStructType(*ast.StructType, Cursor) bool         { return true }
func (VisitorImpl) VisitFuncType(*ast.FuncType, Cursor) bool             { return true }
func (VisitorImpl) VisitInterfaceType(*ast.InterfaceType, Cursor) bool   { return true }
func (VisitorImpl) VisitMapType(*ast.MapType, Cursor) bool               { return true }
func (VisitorImpl) VisitChanType(*ast.ChanType, Cursor) bool             { return true }
func (VisitorImpl) VisitBadStmt(*ast.BadStmt, Cursor) bool               { return true }
func (VisitorImpl) VisitDeclStmt(*ast.DeclStmt, Cursor) bool             { return true }
func (VisitorImpl) VisitEmptyStmt(*ast.EmptyStmt, Cursor) bool           { return true }
func (VisitorImpl) VisitLabeledStmt(*ast.LabeledStmt, Cursor) bool       { return true }
func (VisitorImpl) VisitExprStmt(*ast.ExprStmt, Cursor) bool             { return true }
func (VisitorImpl) VisitSendStmt(*ast.SendStmt, Cursor) bool             { return true }
func (VisitorImpl) VisitIncDecStmt(*ast.IncDecStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitAssignStmt(*ast.AssignStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitGoStmt(*ast.GoStmt, Cursor) bool                 { return true }
func (VisitorImpl) VisitDeferStmt(*ast.DeferStmt, Cursor) bool           { return true }
func (VisitorImpl) VisitReturnStmt(*ast.ReturnStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitBranchStmt(*ast.BranchStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitBlockStmt(*ast.BlockStmt, Cursor) bool           { return true }
func (VisitorImpl) VisitIfStmt(*ast.IfStmt, Cursor) bool                 { return true }
func (VisitorImpl) VisitCaseClause(*ast.CaseClause, Cursor) bool         { return true }
func (VisitorImpl) VisitSwitchStmt(*ast.SwitchStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitTypeSwitchStmt(*ast.TypeSwitchStmt, Cursor) bool { return true }
func (VisitorImpl) VisitCommClause(*ast.CommClause, Cursor) bool         { return true }
func (VisitorImpl) VisitSelectStmt(*ast.SelectStmt, Cursor) bool         { return true }
func (VisitorImpl) VisitForStmt(*ast.ForStmt, Cursor) bool               { return true }
func (VisitorImpl) VisitRangeStmt(*ast.RangeStmt, Cursor) bool           { return true }
func (VisitorImpl) VisitImportSpec(*ast.ImportSpec, Cursor) bool         { return true }
func (VisitorImpl) VisitValueSpec(*ast.ValueSpec, Cursor) bool           { return true }
func (VisitorImpl) VisitTypeSpec(*ast.TypeSpec, Cursor) bool             { return true }
func (VisitorImpl) VisitBadDecl(*ast.BadDecl, Cursor) bool               { return true }
func (VisitorImpl) VisitGenDecl(*ast.GenDecl, Cursor) bool               { return true }
func (VisitorImpl) VisitFuncDecl(*ast.FuncDecl, Cursor) bool             { return true }
func (VisitorImpl) VisitFile(*ast.File, Cursor) bool                     { return true }
func (VisitorImpl) VisitPackage(*ast.Package, Cursor) bool               { return true }
