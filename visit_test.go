package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// Type definitions for test visitors

type funcCounter struct {
	VisitorImpl
	count int
}

func (v *funcCounter) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	v.count++
	return true
}

type orderTracker struct {
	VisitorImpl
	order *[]string
	phase string
}

func (v *orderTracker) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	*v.order = append(*v.order, v.phase+":"+node.Name.Name)
	return true
}

func (v *orderTracker) VisitBasicLit(node *ast.BasicLit, cursor Cursor) bool {
	*v.order = append(*v.order, v.phase+":"+node.Value)
	return true
}

type skipInner struct {
	VisitorImpl
	funcNames []string
	funcLits  int
}

func (v *skipInner) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	v.funcNames = append(v.funcNames, node.Name.Name)
	// Return false to skip visiting children
	return false
}

func (v *skipInner) VisitFuncLit(node *ast.FuncLit, cursor Cursor) bool {
	v.funcLits++
	return true
}

type declPathRecorder struct {
	VisitorImpl
	skipFuncDecls bool
	paths         []Path
}

func (v *declPathRecorder) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	v.paths = append(v.paths, cursor.Path())
	return !v.skipFuncDecls
}

func (v *declPathRecorder) VisitGenDecl(node *ast.GenDecl, cursor Cursor) bool {
	v.paths = append(v.paths, cursor.Path())
	return true
}

type funcVisitor struct {
	VisitorImpl
	found []string
}

func (v *funcVisitor) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	v.found = append(v.found, node.Name.Name)
	return true
}

// Test basic visitor functionality - example from README
func TestVisit_FuncCounter(t *testing.T) {
	src := `package main

func Hello() {
	println("world")
}

func Goodbye() {
	println("bye")
}

func main() {
	Hello()
	Goodbye()
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	counter := &funcCounter{}

	Visit(file, counter, nil)

	expected := 3 // Hello, Goodbye, main
	if counter.count != expected {
		t.Errorf("Found %d functions, want %d", counter.count, expected)
	}
}

// Test pre and post order traversal
func TestVisit_PreAndPost(t *testing.T) {
	src := `package test
func foo() int {
	return 42
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	var preOrder, postOrder []string

	pre := &orderTracker{order: &preOrder, phase: "pre"}
	post := &orderTracker{order: &postOrder, phase: "post"}

	Visit(file, pre, post)

	// Pre should visit parent before children
	if len(preOrder) != 2 {
		t.Errorf("Pre-order visits: %d, want 2", len(preOrder))
	}
	if preOrder[0] != "pre:foo" {
		t.Errorf("First pre-order visit: %s, want pre:foo", preOrder[0])
	}

	// Post should visit children before parent
	if len(postOrder) != 2 {
		t.Errorf("Post-order visits: %d, want 2", len(postOrder))
	}
	if postOrder[1] != "post:foo" {
		t.Errorf("Last post-order visit: %s, want post:foo", postOrder[1])
	}
}

// Test that returning false skips children but continues with siblings
func TestVisit_SkipChildren(t *testing.T) {
	src := `package test
func outer() {
	inner := func() {
		var x = 42
	}
	_ = inner
}

func sibling() {}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	visitor := &skipInner{}

	Visit(file, visitor, nil)

	// The FuncLit "inner" is a child of "outer" and must not be visited
	if visitor.funcLits != 0 {
		t.Errorf("Visited %d function literals within skipped children, want 0", visitor.funcLits)
	}
	// The sibling of the skipped "outer" must still be visited
	if !slices.Equal(visitor.funcNames, []string{"outer", "sibling"}) {
		t.Errorf("Function names: %v, want [outer sibling]", visitor.funcNames)
	}
}

// Skipping the children of a node must not corrupt the paths
// of the nodes visited after it, else tools that select nodes
// by their path silently match the wrong nodes.
func TestVisit_PathAfterSkippedChildren(t *testing.T) {
	src := `package test
func a() {}
func b() {}
var x = 1`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	visitor := &declPathRecorder{skipFuncDecls: true}
	Visit(file, visitor, nil)

	var paths []string
	for _, path := range visitor.paths {
		paths = append(paths, path.String())
	}
	want := []string{"/File.Decls[0]/FuncDecl", "/File.Decls[1]/FuncDecl", "/File.Decls[2]/GenDecl"}
	if !slices.Equal(paths, want) {
		t.Errorf("Paths: %v, want %v", paths, want)
	}
}

// A path returned by Cursor.Path must stay valid after the visitor
// method returned, so that visitors can collect paths for later use.
func TestVisit_PathIsNotChangedByTraversal(t *testing.T) {
	src := `package test
func a() {}
func b() {}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	visitor := &declPathRecorder{}
	Visit(file, visitor, nil)

	if len(visitor.paths) != 2 {
		t.Fatalf("Recorded %d paths, want 2", len(visitor.paths))
	}
	if got := visitor.paths[0].String(); got != "/File.Decls[0]/FuncDecl" {
		t.Errorf("First path: %s, want /File.Decls[0]/FuncDecl", got)
	}
	if visitor.paths[0].Last().Node != file.Decls[0] {
		t.Errorf("First path doesn't end with the first FuncDecl")
	}
	// The root has no parent field and no index as documented for PathItem
	if root := visitor.paths[0][0]; root.ParentField != "" || root.ParentFieldIndex != -1 {
		t.Errorf("Root PathItem: ParentField %q, ParentFieldIndex %d, want \"\" and -1", root.ParentField, root.ParentFieldIndex)
	}
}

// Visitor methods return false to skip children and continue with siblings.
// A post visitor that shares a visitor type with a pre visitor
// that skips children must not abort the traversal.
func TestVisit_PostVisitorFalseDoesNotAbort(t *testing.T) {
	src := `package test
func a() {}
func b() {}
func c() {}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	post := &skipInner{}
	Visit(file, nil, post)

	if !slices.Equal(post.funcNames, []string{"a", "b", "c"}) {
		t.Errorf("Post visited functions: %v, want [a b c]", post.funcNames)
	}
}

// Test visiting all node types doesn't panic
func TestVisit_AllNodeTypes(t *testing.T) {
	src := `package test

import "fmt"

type MyStruct struct {
	field int
}

func (m MyStruct) Method() {}

func testFunc() {
	// Various statements
	var x = 42
	y := "string"
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			fmt.Println(i)
		}
	}

	switch y {
	case "a":
		break
	default:
		fallthrough
	}

	select {
	case <-make(chan int):
	default:
	}

	defer func() {}()
	go func() {}()
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Just use default visitor that visits everything
	visitor := &VisitorImpl{}

	// Should not panic
	Visit(file, visitor, nil)
}

// directiveCursor is a Cursor at a node of a type without a Visitor method
type directiveCursor struct {
	Cursor
}

func (directiveCursor) Node() ast.Node { return &ast.Directive{} }

// astutil.Apply panics when it descends into a node type it doesn't support,
// like *ast.Directive with golang.org/x/tools v0.52, so visitNode must return
// false for a node type without Visitor method to skip its children.
// The trade-off: children of a future node type that astutil.Apply supports
// are not visited until this package has a Visitor method for it.
func TestVisitNode_UnknownNodeTypeIsNotTraversed(t *testing.T) {
	if visitNode(&VisitorImpl{}, directiveCursor{}) {
		t.Error("visitNode returned true for a node type without Visitor method, want false to skip its children")
	}
}

// Test README example: finding function declarations
func TestVisit_READMEExample(t *testing.T) {
	src := `package main

func Hello() {
	println("world")
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	visitor := &funcVisitor{}

	Visit(file, visitor, nil)

	if len(visitor.found) != 1 {
		t.Errorf("Found %d functions, want 1", len(visitor.found))
	}
	if visitor.found[0] != "Hello" {
		t.Errorf("Function name: %s, want Hello", visitor.found[0])
	}
}

// The post visitor is documented to skip nodes whose pre visitor returned false.
// Visit pops the path of such nodes itself, so the paths of later nodes
// that the post visitor sees must stay correct.
func TestVisit_PostNotCalledAfterPreSkip(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "test.go", "package test\nfunc a() {}\nvar x = 1", 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	pre := &declPathRecorder{skipFuncDecls: true}
	post := &declPathRecorder{}
	Visit(file, pre, post)

	var paths []string
	for _, path := range post.paths {
		paths = append(paths, path.String())
	}
	if want := []string{"/File.Decls[1]/GenDecl"}; !slices.Equal(paths, want) {
		t.Errorf("Post paths: %v, want %v", paths, want)
	}
}

// A node type without a Visitor method, like *ast.Directive which astutil.Apply
// doesn't support, must not make Visit panic in the middle of a rewrite.
func TestVisit_UnknownNodeTypeDoesNotPanic(t *testing.T) {
	visitor := &funcVisitor{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Visit panicked: %v", r)
		}
	}()
	result := Visit(&ast.Directive{}, visitor, visitor)
	if _, ok := result.(*ast.Directive); !ok {
		t.Errorf("Visit returned %T, want the unchanged *ast.Directive root", result)
	}
}
