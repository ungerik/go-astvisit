# go-astvisit

A type-safe visitor pattern implementation for traversing and modifying Go Abstract Syntax Trees (AST) with minimal reflection.

## Features

- **Type-safe visitor pattern**: Visit each AST node type with dedicated methods
- **Full AST coverage**: Supports all Go AST node types
- **Cursor-based navigation**: Track position and parent relationships while traversing
- **Node modifications**: Replace, delete, insert nodes during traversal
- **Code rewriting**: Bulk source file transformations with imports management
- **AST utilities**: Parse, format, and convert AST nodes to strings

## Installation

```bash
go get github.com/ungerik/go-astvisit
```

## Documentation

- [Tutorial: write your first codemod](docs/tutorial-write-a-codemod.md)
- How-to guides:
  [modify an AST with Cursor](docs/howto-modify-ast-with-cursor.md),
  [rewrite Go source files](docs/howto-rewrite-source-files.md),
  [check in CI that no file needs rewriting](docs/howto-check-generated-code-in-ci.md)
- Explanation:
  [why NodeReplacements edits source text](docs/explanation-text-replacements.md),
  [how the Rewrite functions choose files and when they write](docs/explanation-file-selection-writes.md)
- API reference: [pkg.go.dev](https://pkg.go.dev/github.com/ungerik/go-astvisit)

All pages are listed in [docs/README.md](docs/README.md).

## Quick Start

### Basic Visitor Example

```go
package main

import (
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"

    "github.com/ungerik/go-astvisit"
)

// Define a visitor that finds all function declarations
type funcVisitor struct {
    astvisit.VisitorImpl // Embed default implementation
}

func (v *funcVisitor) VisitFuncDecl(node *ast.FuncDecl, cursor astvisit.Cursor) bool {
    fmt.Printf("Found function: %s at %s\n", node.Name.Name, cursor.ParentField())
    return true // Continue visiting child nodes
}

func main() {
    src := `package main
    func Hello() {
        println("world")
    }`

    fset := token.NewFileSet()
    file, _ := parser.ParseFile(fset, "example.go", src, 0)

    visitor := &funcVisitor{}
    astvisit.Visit(file, visitor, nil)
}
```

### Modifying AST Example

```go
// Rename all identifiers named "old" to "new"
type renameVisitor struct {
    astvisit.VisitorImpl
}

func (v *renameVisitor) VisitIdent(node *ast.Ident, cursor astvisit.Cursor) bool {
    if node.Name == "old" {
        node.Name = "new"
    }
    return true
}

// Apply the visitor
astvisit.Visit(file, &renameVisitor{}, nil)
```

### Rewriting Source Files

```go
// Rewrite all .go files in a directory tree
err := astvisit.Rewrite("./...", os.Stdout, nil,
    func(fset *token.FileSet, pkg *ast.Package, file *ast.File,
         filePath string, verboseOut io.Writer) ([]byte, error) {

        // Your transformation logic here
        // Return modified source or nil for no changes
        return nil, nil
    })
```

Rewrite skips test files, files excluded from the build for every platform
(like `//go:build ignore` generator files), and directories that the go command
ignores for `./...` patterns (`testdata`, `vendor`, names starting with `.` or `_`,
nested modules, and directories matching an `ignore` directive of `go.mod`)
plus `node_modules`. A directory that is passed explicitly as path is never skipped,
and an `ignore` directive that matches it doesn't apply to its subdirectories either.
A file that is passed explicitly as path is rewritten even if it is
excluded from the build, like the go command does for files named on the command line.
The files of a package are written only after the callback succeeded for all of them,
and only if their content changed. CRLF line endings of a file are kept.

`RewriteWithReplacements` applies `NodeReplacements` and `Imports` returned per file
to the parsed source and formats the result. A file is left unchanged if the result
only differs in formatting from the source. With `debug` set, the replacements are
written with conflict-style markers to `resultOut`, which must not be nil,
and no files are changed.

#### Package-Level Rewriting

`RewritePackages` calls the callback once per package with all parsed files,
their sources and the `token.FileSet`, and writes the returned files
only if the callback succeeded:

```go
err := astvisit.RewritePackages("./...", os.Stdout, nil,
    func(pkg *astvisit.ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
        rewritten := make(map[string][]byte)
        for _, filePath := range pkg.FilePaths {
            // Use pkg.Files[filePath], pkg.Sources[filePath], pkg.Fset
            // and set rewritten[filePath] for changed files
        }
        return rewritten, nil
    })
```

#### Dry-Run for CI

`RewriteDryRun` returns the files that would be changed without writing them.
Per file callbacks can be converted with `RewriteFileFunc.PackageFunc`
and `FileReplacementsFunc.PackageFunc`:

```go
changedFiles, err := astvisit.RewriteDryRun("./...", nil,
    astvisit.FileReplacementsFunc(myReplacementsFunc).PackageFunc())
if len(changedFiles) > 0 {
    // Generated code is outdated
}
```

## Core Concepts

### Visitor Interface

The `Visitor` interface defines a method for each AST node type:

```go
type Visitor interface {
    VisitIdent(*ast.Ident, Cursor) bool
    VisitFuncDecl(*ast.FuncDecl, Cursor) bool
    VisitIndexListExpr(*ast.IndexListExpr, Cursor) bool // Go 1.18+ generics
    // ... methods for all AST node types
}
```

Each method returns `bool`:
- `true`: Continue visiting child nodes
- `false`: Skip child nodes

Nodes of a type without a `Visitor` method are skipped by `Visit` together with their children.
Currently that is only `*ast.Directive` (added in Go 1.26), which is never part of a parsed file.

### Cursor Interface

The `Cursor` provides context about the current position in the AST:

```go
type Cursor interface {
    Node() ast.Node              // Current node
    Parent() ast.Node            // Parent node
    Path() Path                  // Full path from root
    ParentField() string         // Field name in parent struct
    ParentFieldIndex() int       // Index if in a slice (-1 otherwise)

    Replace(ast.Node)            // Replace current node
    Delete()                     // Delete current node from slice
    InsertBefore(ast.Node)       // Insert before current node
    InsertAfter(ast.Node)        // Insert after current node
}
```

### VisitorImpl Helper

Embed `VisitorImpl` in your visitor to get default implementations:

```go
type MyVisitor struct {
    astvisit.VisitorImpl  // Provides default implementation for all methods
}

// Only override the methods you need
func (v *MyVisitor) VisitFuncDecl(node *ast.FuncDecl, cursor astvisit.Cursor) bool {
    // Your logic here
    return true
}
```

## Advanced Usage

### Node Replacements

Build a set of node replacements and apply them to source code:

```go
var replacements astvisit.NodeReplacements

// Add a replacement
replacements.AddReplacement(oldNode, newNode, "debug-id")

// Add an insertion after a node
replacements.AddInsertAfter(node, "// New comment\n", "comment-added")

// Remove a node
replacements.AddRemoval(node, "removed-unused")

// Apply all replacements to source
newSource, err := replacements.Apply(fset, originalSource)
```

Replacements are applied in source order. Replacements at the same position,
like several insertions after the same node, are applied in the order they were added.
Overlapping replacements and nodes that are not from the parsed source,
like nodes of another file or without a position, return an error.

### Import Management

Format source with specific imports:

```go
var imports astvisit.Imports
imports.Add("fmt")
imports.AddNamed("bigmath", "math/big") // Named import

formatted, err := astvisit.FormatSource(source, imports, "mycompany.com")
```

If imports were added, they are grouped goimports-style,
else the result is the same as gofmt output.

### Parsing Utilities

```go
// Parse code snippets without package declaration
decls, comments, err := astvisit.ParseDeclarations(fset, `
    func foo() {}
    type Bar struct{}
`)

// Parse statements
stmts, comments, err := astvisit.ParseStatements(fset, `
    x := 42
    fmt.Println(x)
`)

// Parse full package
pkg, err := astvisit.ParsePackage(fset, "./mypackage", nil)
```

`ParsePackage` skips files that are excluded from the build for every GOOS/GOARCH,
like `//go:build ignore` generator files, and returns main packages like any other package.

### Converting AST to Strings

```go
// Convert expression to string
str := astvisit.ExprString(expr)
// Example: "map[string]int"

// Get node type name
typeName := astvisit.NodeType(node)
// Example: "FuncDecl"

// Format function signature
sig := astvisit.FuncTypeString(funcType)
// Example: "(x int, y string) error"
```

## Go 1.18+ Generics Support

The library fully supports Go generics including `IndexListExpr` for type parameters:

```go
type myVisitor struct {
    astvisit.VisitorImpl
}

func (v *myVisitor) VisitIndexListExpr(node *ast.IndexListExpr, cursor astvisit.Cursor) bool {
    // Handles: MyType[T, U], func[T any, U comparable](), etc.
    fmt.Printf("Generic type: %s\n", astvisit.ExprString(node))
    return true
}
```

## Examples

See the `rewrite_test.go` file for a complete example of traversing all files in a directory tree.

## API Stability

This package follows semantic versioning. The `Visitor` interface may be extended with new methods when Go adds new AST node types (as happened with `IndexListExpr` in Go 1.18).

## License

See LICENSE file for details.

## Contributing

Contributions are welcome! Please ensure:
- All AST node types are handled
- Tests pass
- Code is formatted with `gofmt`
- Documentation is updated
