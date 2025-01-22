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

// ParseDeclarations either from a complete source file
// or from a code snippet without a package clause.
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

// ParseStatements a from a code snippet
func ParseStatements(fset *token.FileSet, sourceCode string) ([]ast.Stmt, []*ast.CommentGroup, error) {
	fsrc := "package p; func _() { " + sourceCode + "\n\n}"
	file, err := parser.ParseFile(fset, nextFileName(), fsrc, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return file.Decls[0].(*ast.FuncDecl).Body.List, file.Comments, nil
}

// ParsePackage parses the package source in pkgDir including comments.
// If filter != nil, only the files with fs.FileInfo entries passing through
// the filter (and ending in ".go") are considered.
// Returns a wrapped ErrPackageNotFound error if the filtered pkgDir
// does not contain Go source for a package.
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
