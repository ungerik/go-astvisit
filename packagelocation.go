package astvisit

import (
	"fmt"
	"go/ast"
	"go/build"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

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

func LocatePackageOfImportLine(projectDir, importLine string) (importName string, loc *PackageLocation, err error) {
	var importPath string
	importName, importPath, err = ImportNameAndPathOfImportLine(importLine)
	if err != nil {
		return "", nil, err
	}
	loc, err = LocatePackage(projectDir, importPath)
	if err != nil {
		return "", nil, err
	}
	if importName == "" {
		importName = loc.PkgName
	}
	return importName, loc, nil
}

func LocatePackageOfImportSpec(projectDir string, importSpec *ast.ImportSpec) (importName string, loc *PackageLocation, err error) {
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

// ImportNameAndPathOfImportLine splits an importLine as used in
// an import statement into a importName and importPath part.
// The importName will be empty if the import is not explicitely named.
func ImportNameAndPathOfImportLine(importLine string) (importName, importPath string, err error) {
	importLine = strings.TrimPrefix(importLine, "import ")
	begQuote := strings.IndexByte(importLine, '"')
	endQuote := strings.LastIndexByte(importLine, '"')
	if begQuote == -1 || begQuote == endQuote {
		return "", "", fmt.Errorf("invalid quoted import: %s", importLine)
	}
	importPath = importLine[begQuote+1 : endQuote]
	importName = strings.TrimSpace(importLine[:begQuote])
	return importName, importPath, nil
}
