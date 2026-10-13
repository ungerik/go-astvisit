# How to modify an AST with Cursor

Replace, delete and insert nodes while `astvisit.Visit` walks a syntax tree,
then print the modified tree as Go source.

Use this when you build or transform code in memory. If you edit existing
files and want to keep their comments and layout exactly, use
`NodeReplacements` instead, see
[How to rewrite Go source files](howto-rewrite-source-files.md) and
[why](explanation-text-replacements.md).

## Prerequisites

- `go get github.com/ungerik/go-astvisit`
- A parsed file. Parse with `parser.ParseComments` so comments survive printing:

  ```go
  fset := token.NewFileSet()
  file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
  if err != nil {
  	return err
  }
  ```

- A visitor type that embeds `astvisit.VisitorImpl`. The embedded struct
  implements every `Visit*` method of `astvisit.Visitor` with `return true`, so
  you only write the methods for node types you care about.

## Steps

1. **Replace a node** with `cursor.Replace`. The new node is not visited.

   ```go
   // Replace the magic number 42 with the constant name limit
   func (v *editor) VisitBasicLit(node *ast.BasicLit, cursor astvisit.Cursor) bool {
   	if node.Kind == token.INT && node.Value == "42" {
   		cursor.Replace(ast.NewIdent("limit"))
   	}
   	return true
   }
   ```

2. **Delete a node** from the slice that contains it with `cursor.Delete`.
   Check `ParentFieldIndex()` first: it is negative when the node is a single
   field (like `IfStmt.Cond`), and `Delete` panics there.

   ```go
   // Delete debugLog(...) statements
   func (v *editor) VisitExprStmt(node *ast.ExprStmt, cursor astvisit.Cursor) bool {
   	call, ok := node.X.(*ast.CallExpr)
   	if !ok || astvisit.ExprString(call.Fun) != "debugLog" {
   		return true
   	}
   	if cursor.ParentFieldIndex() < 0 {
   		return true // Not in a statement list, Delete would panic
   	}
   	cursor.Delete()
   	return false
   }
   ```

3. **Insert a node** next to the current one with `cursor.InsertBefore` or
   `cursor.InsertAfter`. The same slice rule applies. Inserted nodes are not
   visited. `InsertBefore` increases the current node's `ParentFieldIndex()`.

   ```go
   // Log every return statement of a statement list
   func (v *editor) VisitReturnStmt(node *ast.ReturnStmt, cursor astvisit.Cursor) bool {
   	if cursor.ParentFieldIndex() < 0 {
   		return true
   	}
   	cursor.InsertBefore(&ast.ExprStmt{X: &ast.CallExpr{
   		Fun:  &ast.SelectorExpr{X: ast.NewIdent("log"), Sel: ast.NewIdent("Println")},
   		Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"return"`}},
   	}})
   	return true
   }
   ```

   Build inserted nodes by hand like this rather than with
   `astvisit.ParseStatements`. Parsed nodes carry positions from another file,
   and the printer turns that position gap into blank lines (see
   [Troubleshooting](#troubleshooting)).

4. **Skip a subtree** by returning `false`. Siblings are still visited.

   ```go
   // Leave function literals alone
   func (v *editor) VisitFuncLit(node *ast.FuncLit, cursor astvisit.Cursor) bool {
   	return false
   }
   ```

5. **Check where you are** before you edit. `cursor.Parent()` is the parent
   node, `cursor.ParentField()` the name of the parent's struct field that
   holds the node, and `cursor.Path()` the whole path from the root:

   ```go
   fmt.Println(cursor.Path())
   // /File.Decls[1]/FuncDecl.Body/BlockStmt.List[2]/ReturnStmt
   ```

   `Path()` returns a copy, so you can keep it after the visitor method
   returns. For the root node passed to `Visit`, `Parent()` is `nil` and
   `ParentField()` is empty. When the root is an `*ast.Package`,
   `ParentField()` of a file is its file name.

   Replacing the root node works like replacing any other node: `Visit`
   returns the replacement, so use its result
   (`file = astvisit.Visit(file, v, nil).(*ast.File)`) if your visitor can
   replace the root.

6. **Run the visitor and print the result.**

   ```go
   astvisit.Visit(file, &editor{}, nil)
   err = format.Node(os.Stdout, fset, file)
   ```

### Edit bottom-up with a post visitor

`Visit(root, pre, post)` takes a second visitor that runs after a node's
children have been visited. Use it when an edit depends on the result of edits
further down. This pair deletes `debugLog` calls first, then deletes `if`
statements that became empty:

```go
// removeDebugLog deletes debugLog(...) statements
type removeDebugLog struct{ astvisit.VisitorImpl }

func (removeDebugLog) VisitExprStmt(node *ast.ExprStmt, cursor astvisit.Cursor) bool {
	call, ok := node.X.(*ast.CallExpr)
	if ok && astvisit.ExprString(call.Fun) == "debugLog" && cursor.ParentFieldIndex() >= 0 {
		cursor.Delete()
		return false
	}
	return true
}

// removeEmptyIf deletes if statements that are empty
// after their children have been visited
type removeEmptyIf struct{ astvisit.VisitorImpl }

func (removeEmptyIf) VisitIfStmt(node *ast.IfStmt, cursor astvisit.Cursor) bool {
	if len(node.Body.List) == 0 && node.Else == nil && cursor.ParentFieldIndex() >= 0 {
		cursor.Delete()
	}
	return true
}

// ...
astvisit.Visit(file, removeDebugLog{}, removeEmptyIf{})
```

The return value of a post visitor is ignored. The post visitor is not called
for a node whose pre visitor returned `false`.

## Complete example

```go
package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"github.com/ungerik/go-astvisit"
)

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

type editor struct {
	astvisit.VisitorImpl
}

// Replace the magic number 42 with the constant name limit
func (v *editor) VisitBasicLit(node *ast.BasicLit, cursor astvisit.Cursor) bool {
	if node.Kind == token.INT && node.Value == "42" {
		cursor.Replace(ast.NewIdent("limit"))
	}
	return true
}

// Delete debugLog(...) statements
func (v *editor) VisitExprStmt(node *ast.ExprStmt, cursor astvisit.Cursor) bool {
	call, ok := node.X.(*ast.CallExpr)
	if !ok || astvisit.ExprString(call.Fun) != "debugLog" {
		return true
	}
	if cursor.ParentFieldIndex() < 0 {
		return true // Not in a statement list, Delete would panic
	}
	cursor.Delete()
	return false
}

// Log every return statement of a statement list
func (v *editor) VisitReturnStmt(node *ast.ReturnStmt, cursor astvisit.Cursor) bool {
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
func (v *editor) VisitFuncLit(node *ast.FuncLit, cursor astvisit.Cursor) bool {
	return false
}

func main() {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	astvisit.Visit(file, &editor{}, nil)
	err = format.Node(os.Stdout, fset, file)
	if err != nil {
		panic(err)
	}
}
```

Output:

```
/File.Decls[1]/FuncDecl.Body/BlockStmt.List[0]/IfStmt.Body/BlockStmt.List[0]/ReturnStmt
/File.Decls[1]/FuncDecl.Body/BlockStmt.List[2]/ReturnStmt
package p

import "log"

func compute(x int) int {

	if x > limit {
		log.Println("return")
		return limit
	}
	handler := func() int {
		return 42
	}
	log.Println("return")
	return handler() + x
}
```

The paths show the indices after the deletion: the `if` moved from `List[1]`
to `List[0]`. The `42` inside the function literal is unchanged because
`VisitFuncLit` returned `false`. The blank line after `{` is where the deleted
statement was, see [Troubleshooting](#troubleshooting).

## Verification

- Check the error of `format.Node`. It fails for some malformed trees, but a
  `nil` error doesn't prove the output is valid Go.
- Parse the printed output again with `parser.ParseFile` and check that error.
  That proves the result is syntactically valid Go.
- Compile it, for example with `go build` or `go vet` on the written file, to
  catch type errors like a missing import.

## Troubleshooting

**`panic: Delete node not contained in slice`** (or `InsertAfter …`,
`InsertBefore …`). The current node is a single field or the root node, not a
slice element. Check `cursor.ParentFieldIndex() >= 0` before calling these
methods.

**`panic: attempt to replace *ast.File with non-*ast.File`.** Inside an
`*ast.Package`, a file can only be replaced with another `*ast.File`. To remove
a file from the package, call `cursor.Delete()`.

**`VisitComment` is not called for some comments.** `Visit` only walks
comments that are attached to nodes, like `FuncDecl.Doc` or `Field.Comment`.
Free-floating comments in `ast.File.Comments` are not walked. Range over
`file.Comments` yourself to see all of them.

**My replacement or inserted node isn't visited.** `Visit` is built on
`astutil.Apply`, which doesn't walk replaced or inserted nodes. Call
`astvisit.Visit` on the new node yourself if you need it visited.

**The printed output has extra blank lines.** The printer lays out code by the
positions stored in the nodes. A deleted node leaves a position gap, and nodes
from `ParseStatements` or `ParseDeclarations` carry positions from another
file. Build inserted nodes by hand (they have no position), run `gofmt`
afterwards if the blank lines bother you, or switch to `NodeReplacements` to
leave the untouched source byte for byte as it was.

**Comments end up in the wrong place after an edit.** Comments live in
`ast.File.Comments` and are placed by position, not attached to nodes. Deleting
or moving nodes doesn't move their comments. Use `NodeReplacements` for edits
in files where comments matter.

## Related

- [`Cursor` reference](https://pkg.go.dev/github.com/ungerik/go-astvisit#Cursor)
  and [`Visit` reference](https://pkg.go.dev/github.com/ungerik/go-astvisit#Visit)
- [How to rewrite Go source files](howto-rewrite-source-files.md)
- [Why NodeReplacements edits source text instead of reprinting the AST](explanation-text-replacements.md)
- [Tutorial: write your first codemod](tutorial-write-a-codemod.md)
