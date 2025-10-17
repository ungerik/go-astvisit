package astvisit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"sync/atomic"
)

var fileCounter uint32

func nextFileName() string {
	c := atomic.AddUint32(&fileCounter, 1)
	return fmt.Sprintf("file%d.go", c)
}

// ParseDeclarations parses Go declarations from source code.
// The source can be either a complete file or a code snippet without a package clause.
//
// If the source doesn't start with "package", it's automatically wrapped with
// "package p; " to make it parseable.
//
// Returns:
//   - []ast.Decl: The parsed declarations
//   - []*ast.CommentGroup: Associated comments
//   - error: Parse error if the source is invalid
//
// Example:
//
//	decls, comments, err := ParseDeclarations(fset, `
//	    func Hello() string {
//	        return "world"
//	    }
//	    type Person struct {
//	        Name string
//	    }
//	`)
func ParseDeclarations(fset *token.FileSet, sourceCode string) ([]ast.Decl, []*ast.CommentGroup, error) {
	// Try as whole source file.
	file, err := parser.ParseFile(fset, nextFileName(), sourceCode, parser.ParseComments)
	if err == nil {
		return file.Decls, file.Comments, nil
	}
	// If the error is that the source file didn't begin with a
	// package line and source fragments are ok, fall through to
	// try as a source fragment. Stop and return on any other error.
	if !strings.Contains(err.Error(), "expected 'package'") {
		return nil, nil, err
	}

	// If this is a declaration list, make it a source file
	// by inserting a package clause.
	// Insert using a ';', not a newline, so that the line numbers
	// in psrc match the ones in src.
	psrc := "package p; " + sourceCode
	file, err = parser.ParseFile(fset, "", psrc, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return file.Decls, file.Comments, nil
}

// ParseStatements parses Go statements from a code snippet.
// The statements are automatically wrapped in a function body to make them parseable.
//
// Returns:
//   - []ast.Stmt: The parsed statements
//   - []*ast.CommentGroup: Associated comments
//   - error: Parse error if the source is invalid
//
// Example:
//
//	stmts, comments, err := ParseStatements(fset, `
//	    x := 42
//	    y := x * 2
//	    fmt.Println(y)
//	`)
func ParseStatements(fset *token.FileSet, sourceCode string) ([]ast.Stmt, []*ast.CommentGroup, error) {
	fsrc := "package p; func _() { " + sourceCode + "\n\n}"
	file, err := parser.ParseFile(fset, nextFileName(), fsrc, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return file.Decls[0].(*ast.FuncDecl).Body.List, file.Comments, nil
}

// ParsePackage parses all Go source files in a directory into a single package.
// Comments are included in the parsed result.
//
// Parameters:
//   - fset: Token file set for position tracking
//   - pkgDir: Directory containing the package source files
//   - filter: Optional filter function to select which files to parse.
//     Only files where filter returns true (and ending in ".go") are parsed.
//     Pass nil to parse all .go files.
//
// Returns:
//   - *ast.Package: The parsed package containing all files
//   - error: ErrPackageNotFound if no Go files found, or parse error
//
// Note: The "main" package is automatically ignored/deleted from results.
//
// Example:
//
//	// Parse all non-test files
//	filter := func(info fs.FileInfo) bool {
//	    return !strings.HasSuffix(info.Name(), "_test.go")
//	}
//	pkg, err := ParsePackage(fset, "./mypackage", filter)
func ParsePackage(fset *token.FileSet, pkgDir string, filter func(fs.FileInfo) bool) (pkg *ast.Package, err error) {
	pkgs, err := parser.ParseDir(fset, pkgDir, filter, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	delete(pkgs, "main") // ignore main package
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrPackageNotFound, pkgDir)
	}
	if len(pkgs) > 1 {
		var pkgNames []string
		for _, pkg := range pkgs {
			pkgNames = append(pkgNames, pkg.Name)
		}
		return nil, fmt.Errorf("%d packages found in %s: %s", len(pkgs), pkgDir, strings.Join(pkgNames, ", "))
	}
	for _, pkg = range pkgs {
		return pkg, nil
	}
	panic("never reached")
}
