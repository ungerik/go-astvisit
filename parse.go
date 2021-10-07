package astvisit

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ParseDeclarations either from a complete source file
// or from a code snippet without a package clause.
func ParseDeclarations(fset *token.FileSet, src string) ([]ast.Decl, error) {
	// Try as whole source file.
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err == nil {
		return file.Decls, nil
	}
	// If the error is that the source file didn't begin with a
	// package line and source fragments are ok, fall through to
	// try as a source fragment. Stop and return on any other error.
	if !strings.Contains(err.Error(), "expected 'package'") {
		return nil, err
	}

	// If this is a declaration list, make it a source file
	// by inserting a package clause.
	// Insert using a ';', not a newline, so that the line numbers
	// in psrc match the ones in src.
	psrc := "package p; " + src
	file, err = parser.ParseFile(fset, "", psrc, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return file.Decls, nil
}

// ParseStatements a from a code snippet
func ParseStatements(fset *token.FileSet, src string) ([]ast.Stmt, error) {
	fsrc := "package p; func _() { " + src + "\n\n}"
	file, err := parser.ParseFile(fset, "", fsrc, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return file.Decls[0].(*ast.FuncDecl).Body.List, nil
}

type PackageLocation struct {
	Package    string
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
		Package:    pkgs[0].Name,
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
		importName = loc.Package
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
		importName = loc.Package
	}
	return importName, loc, nil
}

// // EnumPackageGoFiles enums the path of all non test .go files of a package via callback.
// func EnumPackageGoFiles(pkgDir string, fset *token.FileSet, callback func(fset *token.FileSet, filePath string) error) error {
// 	return fs.File(pkgDir).ListDir(func(file fs.File) error {
// 		if file.IsDir() || file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
// 			return nil
// 		}
// 		return callback(fset, file.LocalPath())
// 	})
// }

// // EnumPackageGoFilesRecursive enums the path of all non test .go files of a package
// // and all sub-packages (basePkgDir/...) via callback.
// // For every package new token.FileSet is created and passed to the callback.
// func EnumPackageGoFilesRecursive(basePkgDir string, callback func(fset *token.FileSet, filePath string) error) error {
// 	fset := token.NewFileSet()
// 	return fs.File(basePkgDir).ListDir(func(file fs.File) error {
// 		if file.IsDir() {
// 			return EnumPackageGoFilesRecursive(file.LocalPath(), callback)
// 		}

// 		if file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
// 			return nil
// 		}
// 		return callback(fset, file.LocalPath())
// 	})
// }
