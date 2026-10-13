package astvisit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
)

var fileCounter atomic.Uint32

func nextFileName() string {
	c := fileCounter.Add(1)
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
//     Pass nil to parse all .go files that are part of the build (see below).
//
// Like the go command, ParsePackage skips files that are excluded from the build
// for every GOOS and GOARCH, with and without cgo. These are files with names
// starting with "_" or "." and files with build constraints that need custom
// build tags, like generator files with a //go:build ignore constraint.
// Files for other platforms than the current one are parsed.
// Main packages are returned like any other package.
//
// Returns:
//   - *ast.Package: The parsed package containing all files
//   - error: ErrPackageNotFound if no Go files found, parse error,
//     or an error if the files belong to more than one package
//
// Example:
//
//	// Parse all non-test files
//	filter := func(info fs.FileInfo) bool {
//	    return !strings.HasSuffix(info.Name(), "_test.go")
//	}
//	pkg, err := ParsePackage(fset, "./mypackage", filter)
func ParsePackage(fset *token.FileSet, pkgDir string, filter func(fs.FileInfo) bool) (pkg *ast.Package, err error) {
	parsed, err := parsePackage(fset, pkgDir, filter)
	if err != nil {
		return nil, err
	}
	return parsed.astPackage(), nil
}

func parsePackage(fset *token.FileSet, pkgDir string, filter func(fs.FileInfo) bool) (*ParsedPackage, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}
	pkgs := make(map[string]*ParsedPackage)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if filter != nil {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			if !filter(info) {
				continue
			}
		}
		filePath := filepath.Join(pkgDir, entry.Name())
		// #nosec G304 - filePath is a Go file in pkgDir
		source, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		buildable, err := isBuildable(pkgDir, entry.Name(), source)
		if err != nil {
			return nil, err
		}
		if !buildable {
			continue
		}
		file, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pkg := pkgs[file.Name.Name]
		if pkg == nil {
			pkg = &ParsedPackage{
				Dir:     pkgDir,
				Name:    file.Name.Name,
				Fset:    fset,
				Files:   make(map[string]*ast.File),
				Sources: make(map[string][]byte),
			}
			pkgs[file.Name.Name] = pkg
		}
		pkg.Files[filePath] = file
		pkg.Sources[filePath] = source
		pkg.FilePaths = append(pkg.FilePaths, filePath) // sorted because os.ReadDir sorts by name
	}
	switch len(pkgs) {
	case 0:
		return nil, fmt.Errorf("%w in %s", ErrPackageNotFound, pkgDir)
	case 1:
		for _, pkg := range pkgs {
			return pkg, nil
		}
	}
	pkgNames := slices.Sorted(maps.Keys(pkgs))
	return nil, fmt.Errorf("%d packages found in %s: %s", len(pkgs), pkgDir, strings.Join(pkgNames, ", "))
}

// parseFilePackage parses the package of the Go file filePath
// with FilePaths set to only filePath.
// Like the go command does for files named on the command line,
// a file that is excluded from the build is parsed as a package of its own.
// The filter selects the other files of the package, but is not applied
// to filePath itself, so the caller must check filePath.
func parseFilePackage(fset *token.FileSet, filePath string, filter func(fs.FileInfo) bool) (*ParsedPackage, error) {
	dir, name := filepath.Dir(filePath), filepath.Base(filePath)
	// #nosec G304 - filePath is the Go file to parse
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	buildable, err := isBuildable(dir, name, source)
	if err != nil {
		return nil, err
	}
	if !buildable {
		file, err := parser.ParseFile(fset, filePath, source, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		return &ParsedPackage{
			Dir:       dir,
			Name:      file.Name.Name,
			Fset:      fset,
			Files:     map[string]*ast.File{filePath: file},
			Sources:   map[string][]byte{filePath: source},
			FilePaths: []string{filePath},
		}, nil
	}
	pkg, err := parsePackage(fset, dir, filter)
	if err != nil {
		return nil, err
	}
	if _, ok := pkg.Files[filePath]; !ok {
		return nil, fmt.Errorf("package %s has no file %s", pkg.Name, name)
	}
	pkg.FilePaths = []string{filePath}
	return pkg, nil
}

// isBuildable reports whether the Go file name in dir with the given source
// is part of the build for at least one GOOS and GOARCH, with or without cgo,
// without custom build tags.
func isBuildable(dir, name string, source []byte) (bool, error) {
	ctxt := build.Default
	ctxt.OpenFile = func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(source)), nil
	}
	// Fast path for the current platform
	if match, err := ctxt.MatchFile(dir, name); match || err != nil {
		return match, err
	}
	for _, cgo := range []bool{true, false} {
		for _, goos := range knownOS {
			for _, goarch := range knownArch {
				ctxt.CgoEnabled, ctxt.GOOS, ctxt.GOARCH = cgo, goos, goarch
				if match, err := ctxt.MatchFile(dir, name); match || err != nil {
					return match, err
				}
			}
		}
	}
	return false, nil
}

// knownOS and knownArch are the GOOS and GOARCH values
// known to go/build as listed in Go's internal/syslist package.
var (
	knownOS = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js",
		"linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos",
	}
	knownArch = []string{
		"386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "loong64",
		"mips", "mipsle", "mips64", "mips64le", "mips64p32", "mips64p32le",
		"ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390", "s390x", "sparc", "sparc64", "wasm",
	}
)
