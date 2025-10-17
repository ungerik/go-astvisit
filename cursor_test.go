package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Type definitions for test visitors

type replacer struct {
	VisitorImpl
	replaced bool
}

func (v *replacer) VisitBasicLit(node *ast.BasicLit, cursor Cursor) bool {
	if node.Value == "42" {
		cursor.Replace(&ast.BasicLit{
			Kind:  token.INT,
			Value: "99",
		})
		v.replaced = true
	}
	return true
}

type deleter struct {
	VisitorImpl
	deleted bool
}

func (v *deleter) VisitAssignStmt(node *ast.AssignStmt, cursor Cursor) bool {
	// Delete the statement that assigns to 'y'
	if len(node.Lhs) > 0 {
		if ident, ok := node.Lhs[0].(*ast.Ident); ok && ident.Name == "y" {
			cursor.Delete()
			v.deleted = true
		}
	}
	return true
}

type inserter struct {
	VisitorImpl
	insertedBefore bool
	insertedAfter  bool
}

func (v *inserter) VisitExprStmt(node *ast.ExprStmt, cursor Cursor) bool {
	// Check if this ExprStmt contains a call to "middle"
	if callExpr, ok := node.X.(*ast.CallExpr); ok {
		if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "middle" {
			// Insert before
			beforeStmt := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.Ident{Name: "before"},
				},
			}
			cursor.InsertBefore(beforeStmt)
			v.insertedBefore = true

			// Insert after
			afterStmt := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.Ident{Name: "after"},
				},
			}
			cursor.InsertAfter(afterStmt)
			v.insertedAfter = true
		}
	}
	return true
}

type pathTracker struct {
	VisitorImpl
	returnStmtPath string
}

func (v *pathTracker) VisitReturnStmt(node *ast.ReturnStmt, cursor Cursor) bool {
	path := cursor.Path()
	v.returnStmtPath = path.String()
	return true
}

type parentChecker struct {
	VisitorImpl
	foundParent bool
	t           *testing.T
}

func (v *parentChecker) VisitBasicLit(node *ast.BasicLit, cursor Cursor) bool {
	if node.Value == "42" {
		parent := cursor.Parent()
		require.NotNil(v.t, parent, "Parent should not be nil")
		_, ok := parent.(*ast.ReturnStmt)
		v.foundParent = ok
	}
	return true
}

type fieldChecker struct {
	VisitorImpl
	foundField bool
}

func (v *fieldChecker) VisitReturnStmt(node *ast.ReturnStmt, cursor Cursor) bool {
	field := cursor.ParentField()
	// ReturnStmt should be in the "List" field of BlockStmt
	v.foundField = field == "List"
	return true
}

type complexReplacer struct {
	VisitorImpl
	replaced bool
}

func (v *complexReplacer) VisitBinaryExpr(node *ast.BinaryExpr, cursor Cursor) bool {
	if node.Op == token.ADD {
		// Create add(a, b) call
		cursor.Replace(&ast.CallExpr{
			Fun: &ast.Ident{Name: "add"},
			Args: []ast.Expr{
				node.X,
				node.Y,
			},
		})
		v.replaced = true
	}
	return true
}

type multiReplacer struct {
	VisitorImpl
	count int
}

func (v *multiReplacer) VisitBasicLit(node *ast.BasicLit, cursor Cursor) bool {
	// Replace all integer literals with 99
	if node.Kind == token.INT {
		cursor.Replace(&ast.BasicLit{
			Kind:  token.INT,
			Value: "99",
		})
		v.count++
	}
	return true
}

// Test Cursor.Replace - replacing nodes during traversal
func TestCursor_Replace(t *testing.T) {
	src := `package test
func foo() int {
	x := 42
	return x
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &replacer{}

	Visit(file, visitor, nil)

	require.True(t, visitor.replaced, "Should have replaced the literal")
}

// Test Cursor.Delete - deleting nodes from slices
func TestCursor_Delete(t *testing.T) {
	src := `package test
func foo() {
	x := 1
	y := 2
	z := 3
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &deleter{}

	Visit(file, visitor, nil)

	require.True(t, visitor.deleted, "Should have deleted the statement")
}

// Test Cursor.InsertBefore and InsertAfter
func TestCursor_InsertBeforeAfter(t *testing.T) {
	src := `package test
func foo() {
	middle()
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &inserter{}

	Visit(file, visitor, nil)

	require.True(t, visitor.insertedBefore, "Should have inserted before")
	require.True(t, visitor.insertedAfter, "Should have inserted after")
}

// Test Cursor.Path - verify path tracking
func TestCursor_Path(t *testing.T) {
	src := `package test
func foo() {
	return 42
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &pathTracker{}

	Visit(file, visitor, nil)

	require.NotEmpty(t, visitor.returnStmtPath, "Should have captured path")
	require.Contains(t, visitor.returnStmtPath, "ReturnStmt", "Path should contain ReturnStmt")
	require.Contains(t, visitor.returnStmtPath, "FuncDecl", "Path should contain FuncDecl")
}

// Test Cursor.Parent - accessing parent node
func TestCursor_Parent(t *testing.T) {
	src := `package test
func foo() int {
	return 42
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &parentChecker{t: t}

	Visit(file, visitor, nil)

	require.True(t, visitor.foundParent, "Should have found ReturnStmt parent")
}

// Test Cursor.ParentField - accessing parent field name
func TestCursor_ParentField(t *testing.T) {
	src := `package test
func foo() int {
	return 42
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &fieldChecker{}

	Visit(file, visitor, nil)

	require.True(t, visitor.foundField, "Should have found 'List' parent field")
}

// Test complex replacement scenario
func TestCursor_ComplexReplacement(t *testing.T) {
	src := `package test
func sum(a, b int) int {
	return a + b
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &complexReplacer{}

	Visit(file, visitor, nil)

	require.True(t, visitor.replaced, "Should have replaced binary expression")
}

// Test multiple replacements in same traversal
func TestCursor_MultipleReplacements(t *testing.T) {
	src := `package test
func foo() {
	x := 1
	y := 2
	z := 3
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	visitor := &multiReplacer{}

	Visit(file, visitor, nil)

	require.Equal(t, 3, visitor.count, "Should have replaced 3 literals")
}
