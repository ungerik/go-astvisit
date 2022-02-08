package astvisit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

type Imports map[string]struct{}

func (imports Imports) Sorted() []string {
	sorted := make([]string, 0, len(imports))
	for imp := range imports {
		sorted = append(sorted, imp)
	}
	sort.Strings(sorted)
	return sorted
}

func (imports Imports) Add(line string) {
	imports[line] = struct{}{}
}

func (imports Imports) Remove(line string) {
	delete(imports, line)
}

func (imports Imports) Contains(line string) bool {
	if _, ok := imports[line]; ok {
		return true
	}
	for imp := range imports {
		_, importPath, err := ImportNameAndPathOfImportLine(imp)
		if err == nil && importPath == line {
			return true
		}
	}
	_, importPath, err := ImportNameAndPathOfImportLine(line)
	if err == nil && importPath != line {
		return imports.Contains(importPath)
	}
	return false
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

// FormatFileWithImports formats sourceCode and makes sure that the passed importLines are included.
// Imports with localImportPrefixes will be sorted into separated groups after the third party imports.
func FormatFileWithImports(fset *token.FileSet, sourceCode []byte, importLines Imports, localImportPrefixes ...string) ([]byte, error) {
	astFile, err := parser.ParseFile(fset, nextFileName(), sourceCode, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	for _, importLine := range importLines.Sorted() {
		name, path, err := ImportNameAndPathOfImportLine(importLine)
		if err != nil {
			return nil, err
		}
		astutil.AddNamedImport(fset, astFile, name, path)
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(sourceCode)*2))
	err = format.Node(buf, fset, astFile)
	if err != nil {
		return nil, err
	}
	sourceCode = buf.Bytes()

	// TODO replace by something more efficient and elegant
	// than re-parsing everything and using the global variable imports.LocalPrefix
	importsLocalPrefixMtx.Lock()
	defer importsLocalPrefixMtx.Unlock()
	imports.LocalPrefix = strings.Join(localImportPrefixes, ",")
	return imports.Process(nextFileName(), sourceCode, &imports.Options{Comments: true, FormatOnly: true})
}

var importsLocalPrefixMtx sync.Mutex
