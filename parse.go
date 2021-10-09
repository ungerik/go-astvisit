package astvisit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"

	"golang.org/x/tools/go/packages"
)

var fileCounter uint32

func nextFileName() string {
	c := atomic.AddUint32(&fileCounter, 1)
	return fmt.Sprintf("file%d.go", c)
}

// ParseDeclarations either from a complete source file
// or from a code snippet without a package clause.
func ParseDeclarations(fset *token.FileSet, src string) ([]ast.Decl, []*ast.CommentGroup, error) {
	// Try as whole source file.
	file, err := parser.ParseFile(fset, nextFileName(), src, parser.ParseComments)
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
	psrc := "package p; " + src
	file, err = parser.ParseFile(fset, "", psrc, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return file.Decls, file.Comments, nil
}

// ParseStatements a from a code snippet
func ParseStatements(fset *token.FileSet, src string) ([]ast.Stmt, []*ast.CommentGroup, error) {
	fsrc := "package p; func _() { " + src + "\n\n}"
	file, err := parser.ParseFile(fset, nextFileName(), fsrc, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return file.Decls[0].(*ast.FuncDecl).Body.List, file.Comments, nil
}

var ErrPackageNotFound = errors.New("package not found")

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
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrPackageNotFound, pkgDir)
	}
	if len(pkgs) > 1 {
		return nil, fmt.Errorf("%d packages found in %s", len(pkgs), pkgDir)
	}
	for _, pkg = range pkgs {
		return pkg, nil
	}
	panic("never reached")
}

type PackageLocation struct {
	PkgName    string
	SourcePath string
	Std        bool
}

func LocatePackage(projectDir, importPath string) (*PackageLocation, error) {
	if len(importPath) >= 2 && importPath[0] == '"' && importPath[len(importPath)-1] == '"' {
		importPath = importPath[1 : len(importPath)-1]
	}
	config := packages.Config{
		Mode: packages.NeedName + packages.NeedFiles,
		Dir:  projectDir,
	}
	pkgs, err := packages.Load(&config, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("could not load importPath %q for projectDir %q", importPath, projectDir)
	}
	return &PackageLocation{
		PkgName:    pkgs[0].Name,
		SourcePath: filepath.Dir(pkgs[0].GoFiles[0]),
		Std:        strings.HasPrefix(pkgs[0].GoFiles[0], build.Default.GOROOT),
	}, nil
}

func ImportLineInfo(projectDir, importLine string) (importName string, loc *PackageLocation, err error) {
	importLine = strings.TrimPrefix(importLine, "import")
	begQuote := strings.IndexByte(importLine, '"')
	endQuote := strings.LastIndexByte(importLine, '"')
	if begQuote == -1 || begQuote == endQuote {
		return "", nil, fmt.Errorf("invalid quoted import: %s", importLine)
	}
	importPath := importLine[begQuote+1 : endQuote]

	loc, err = LocatePackage(projectDir, importPath)
	if err != nil {
		return "", nil, err
	}
	importName = strings.TrimSpace(importLine[:begQuote])
	if importName == "" {
		importName = loc.PkgName
	}
	return importName, loc, nil
}

func ImportSpecInfo(projectDir string, importSpec *ast.ImportSpec) (importName string, loc *PackageLocation, err error) {
	loc, err = LocatePackage(projectDir, importSpec.Path.Value)
	if err != nil {
		return "", nil, err
	}
	importName = ExprString(importSpec.Name)
	if importName == "" {
		importName = loc.PkgName
	}
	return importName, loc, nil
}
