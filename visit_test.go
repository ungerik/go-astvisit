package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
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
}

func (v *skipInner) VisitFuncDecl(node *ast.FuncDecl, cursor Cursor) bool {
	v.funcNames = append(v.funcNames, node.Name.Name)
	// Return false to skip visiting children
	return false
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

// Test that returning false skips children
func TestVisit_SkipChildren(t *testing.T) {
	src := `package test
func outer() {
	inner := func() {
		var x = 42
	}
	_ = inner
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	visitor := &skipInner{}

	Visit(file, visitor, nil)

	funcNames := visitor.funcNames

	// Should only see "outer", not "inner" because we skip children
	if len(funcNames) != 1 {
		t.Errorf("Found %d functions, want 1", len(funcNames))
	}
	if funcNames[0] != "outer" {
		t.Errorf("Function name: %s, want outer", funcNames[0])
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
