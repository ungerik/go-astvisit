package astvisit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
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

// Test Cursor.Replace - replacing nodes during traversal.
// Visit must return the unchanged root when only descendants are replaced,
// because callers continue with the returned root.
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

	result := Visit(file, visitor, nil)

	require.Same(t, file, result, "Visit must return the unchanged root")
	require.True(t, visitor.replaced, "Should have replaced the literal")
	require.Equal(t, "package test\n\nfunc foo() int {\n\tx := 99\n\treturn x\n}\n", formatFile(t, fset, file))
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
	body := file.Decls[0].(*ast.FuncDecl).Body
	require.Len(t, body.List, 2, "y := 2 should have been deleted")
	require.Equal(t, "x", body.List[0].(*ast.AssignStmt).Lhs[0].(*ast.Ident).Name)
	require.Equal(t, "z", body.List[1].(*ast.AssignStmt).Lhs[0].(*ast.Ident).Name)
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
	require.Equal(t, "package test\n\nfunc foo() {\n\tbefore()\n\tmiddle()\n\tafter()\n}\n", formatFile(t, fset, file))
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

	require.Equal(t, "/File.Decls[0]/FuncDecl.Body/BlockStmt.List[0]/ReturnStmt", visitor.returnStmtPath)
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
	require.Equal(t, "package test\n\nfunc sum(a, b int) int {\n\treturn add(a, b)\n}\n", formatFile(t, fset, file))
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
	require.Equal(t, "package test\n\nfunc foo() {\n\tx := 99\n\ty := 99\n\tz := 99\n}\n", formatFile(t, fset, file))
}

// formatFile returns the formatted source of file to verify
// that cursor methods changed the AST and not only visitor state.
func formatFile(t *testing.T, fset *token.FileSet, file *ast.File) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, format.Node(&buf, fset, file))
	return buf.String()
}

// fileVisitor calls visit for every *ast.File
type fileVisitor struct {
	VisitorImpl
	visit func(*ast.File, Cursor) bool
}

func (v *fileVisitor) VisitFile(node *ast.File, cursor Cursor) bool {
	return v.visit(node, cursor)
}

// Cursor.Parent is documented to return nil for the root node,
// which visitors use to detect the root. The internal parent node
// that astutil.Apply wraps the root in must not leak out.
func TestCursor_RootHasNoParent(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package test\n", 0)
	require.NoError(t, err)

	called := false
	Visit(file, &fileVisitor{visit: func(node *ast.File, cursor Cursor) bool {
		called = true
		require.Nil(t, cursor.Parent())
		require.Equal(t, "", cursor.ParentField())
		require.Equal(t, -1, cursor.ParentFieldIndex())
		return false
	}}, nil)
	require.True(t, called)
}

// Replacing the root node must work for an *ast.File like for any other node
// and Visit must return the replacement, instead of panicking because
// astutil.Apply expects every *ast.File to be part of an *ast.Package.
func TestCursor_ReplaceRoot(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package test\n", 0)
	require.NoError(t, err)

	newFile := &ast.File{Name: ast.NewIdent("replaced")}
	result := Visit(file, &fileVisitor{visit: func(node *ast.File, cursor Cursor) bool {
		cursor.Replace(newFile)
		return false
	}}, nil)
	require.Same(t, newFile, result)

	newIdent := ast.NewIdent("y")
	result = Visit(ast.NewIdent("x"), &identReplacer{replacement: newIdent}, nil)
	require.Same(t, newIdent, result)
}

type identReplacer struct {
	VisitorImpl
	replacement *ast.Ident
}

func (v *identReplacer) VisitIdent(node *ast.Ident, cursor Cursor) bool {
	cursor.Replace(v.replacement)
	return false
}

// The root node is not part of a slice, so Delete must panic
// with the documented reason also for an *ast.File root,
// and not with a type assertion error from astutil.
func TestCursor_DeleteRootPanics(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package test\n", 0)
	require.NoError(t, err)

	require.PanicsWithValue(t, "Delete node not contained in slice", func() {
		Visit(file, &fileVisitor{visit: func(node *ast.File, cursor Cursor) bool {
			cursor.Delete()
			return false
		}}, nil)
	})
}

// exampleEditor makes the edits of the complete example
// of docs/howto-modify-ast-with-cursor.md
type exampleEditor struct {
	VisitorImpl
}

// Replace the magic number 42 with the constant name limit
func (v *exampleEditor) VisitBasicLit(node *ast.BasicLit, cursor Cursor) bool {
	if node.Kind == token.INT && node.Value == "42" {
		cursor.Replace(ast.NewIdent("limit"))
	}
	return true
}

// Delete debugLog(...) statements
func (v *exampleEditor) VisitExprStmt(node *ast.ExprStmt, cursor Cursor) bool {
	call, ok := node.X.(*ast.CallExpr)
	if !ok || ExprString(call.Fun) != "debugLog" {
		return true
	}
	if cursor.ParentFieldIndex() < 0 {
		return true // Not in a statement list, Delete would panic
	}
	cursor.Delete()
	return false
}

// Log every return statement of a statement list
func (v *exampleEditor) VisitReturnStmt(node *ast.ReturnStmt, cursor Cursor) bool {
	if cursor.ParentFieldIndex() < 0 {
		return true
	}
	fmt.Println(cursor.Path())
	cursor.InsertBefore(&ast.ExprStmt{X: &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("log"), Sel: ast.NewIdent("Println")},
		Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"return"`}},
	}})
	return true
}

// Leave function literals alone
func (v *exampleEditor) VisitFuncLit(node *ast.FuncLit, cursor Cursor) bool {
	return false
}

// The complete example of docs/howto-modify-ast-with-cursor.md
// with its documented output, so that a change of the Cursor or Path
// behavior breaks this example instead of silently breaking the docs.
func ExampleCursor() {
	const src = `package p

import "log"

func compute(x int) int {
	debugLog("compute", x)
	if x > 42 {
		return 42
	}
	handler := func() int {
		return 42
	}
	return handler() + x
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	Visit(file, &exampleEditor{}, nil)
	err = format.Node(os.Stdout, fset, file)
	if err != nil {
		panic(err)
	}

	// Output:
	// /File.Decls[1]/FuncDecl.Body/BlockStmt.List[0]/IfStmt.Body/BlockStmt.List[0]/ReturnStmt
	// /File.Decls[1]/FuncDecl.Body/BlockStmt.List[2]/ReturnStmt
	// package p
	//
	// import "log"
	//
	// func compute(x int) int {
	//
	//	if x > limit {
	//		log.Println("return")
	//		return limit
	//	}
	//	handler := func() int {
	//		return 42
	//	}
	//	log.Println("return")
	//	return handler() + x
	// }
}
