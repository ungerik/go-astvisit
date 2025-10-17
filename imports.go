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

// Imports is a set of import lines to be added to a Go source file.
// Each entry is a complete import statement without the "import" keyword.
//
// Examples:
//   - `"fmt"` - standard import
//   - `"github.com/pkg/errors"` - third-party import
//   - `math "math/big"` - named import
//   - `. "github.com/onsi/ginkgo"` - dot import
//
// Use with FormatFileWithImports to ensure imports are present in source code.
//
// Example:
//
//	imports := make(Imports)
//	imports.Add(`"fmt"`)
//	imports.Add(`"strings"`)
//	imports.Add(`errors "github.com/pkg/errors"`)
//	formatted, err := FormatFileWithImports(fset, source, imports)
type Imports map[string]struct{}

// Sorted returns all import lines sorted alphabetically.
func (imports Imports) Sorted() []string {
	sorted := make([]string, 0, len(imports))
	for imp := range imports {
		sorted = append(sorted, imp)
	}
	sort.Strings(sorted)
	return sorted
}

// Add adds an import line to the set.
// The line should not include the "import" keyword.
//
// Examples:
//
//	imports.Add(`"fmt"`)
//	imports.Add(`errors "github.com/pkg/errors"`)
func (imports Imports) Add(line string) {
	imports[line] = struct{}{}
}

// Remove removes an import line from the set.
func (imports Imports) Remove(line string) {
	delete(imports, line)
}

// Contains checks if an import line or path is in the set.
// It matches both complete import lines and just the import path.
//
// Examples:
//
//	imports.Add(`"fmt"`)
//	imports.Contains(`"fmt"`)        // true
//	imports.Contains("fmt")          // true (matches path)
//	imports.Contains(`import "fmt"`) // true (strips "import" prefix)
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

// PackageLocation contains information about a Go package's location and name.
type PackageLocation struct {
	PkgName    string // The package name (e.g., "fmt", "errors")
	SourcePath string // Absolute path to the package source directory
	Std        bool   // True if this is a standard library package
}

// LocatePackage finds the source location and name of a package by its import path.
// The projectDir is used as the context for resolving relative imports.
//
// Example:
//
//	loc, err := LocatePackage("/my/project", "fmt")
//	// loc.PkgName = "fmt"
//	// loc.SourcePath = "/usr/local/go/src/fmt"
//	// loc.Std = true
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

// LocatePackageOfImportLine parses an import line and locates the package.
// Returns the import name (which may differ from package name for named imports)
// and the package location.
//
// Example:
//
//	name, loc, err := LocatePackageOfImportLine("/my/project", `errors "github.com/pkg/errors"`)
//	// name = "errors" (the alias)
//	// loc.PkgName = "errors" (the actual package name)
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

// LocatePackageOfImportSpec locates the package referenced by an ast.ImportSpec.
// Returns the import name and package location.
//
// Example:
//
//	// For: import errors "github.com/pkg/errors"
//	name, loc, err := LocatePackageOfImportSpec("/my/project", importSpec)
//	// name = "errors" (the alias from importSpec.Name)
//	// loc.PkgName = "errors"
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

// ImportNameAndPathOfImportLine splits an import line into its name and path components.
// The import line should not include the "import" keyword (though it's stripped if present).
//
// Returns:
//   - importName: The alias/name if explicitly specified, empty string otherwise
//   - importPath: The import path without quotes
//   - error: If the line is malformed
//
// Examples:
//
//	ImportNameAndPathOfImportLine(`"fmt"`)
//	// returns: "", "fmt", nil
//
//	ImportNameAndPathOfImportLine(`errors "github.com/pkg/errors"`)
//	// returns: "errors", "github.com/pkg/errors", nil
//
//	ImportNameAndPathOfImportLine(`. "github.com/onsi/ginkgo"`)
//	// returns: ".", "github.com/onsi/ginkgo", nil
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

// FormatFileWithImports formats source code and ensures specified imports are present.
// It uses goimports-style formatting and grouping.
//
// Parameters:
//   - fset: Token file set for position tracking
//   - sourceCode: The Go source code to format
//   - importLines: Set of imports to ensure are present
//   - localImportPrefixes: Import path prefixes to group separately (e.g., "mycompany.com")
//
// The function:
//   - Adds any missing imports from importLines
//   - Formats the code with gofmt
//   - Groups imports: standard library, third-party, local (by prefix)
//   - Removes unused imports (via goimports)
//
// Example:
//
//	imports := make(Imports)
//	imports.Add(`"fmt"`)
//	imports.Add(`"github.com/pkg/errors"`)
//	formatted, err := FormatFileWithImports(fset, source, imports, "mycompany.com")
//
// Note: This function uses a global mutex for thread-safety when setting
// the LocalPrefix option in golang.org/x/tools/imports.
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
