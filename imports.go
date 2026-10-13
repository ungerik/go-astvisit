package astvisit

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/module"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

// Import is an import of a Go source file.
type Import struct {
	// Name is the explicit import name like "errs", "." or "_",
	// or empty to use the package name.
	Name string
	// Path is the import path without quotes like "github.com/domonda/go-errs".
	Path string
}

// ImportOf returns the Import of an import spec.
// It returns an error if the path of the spec is not a valid string literal.
func ImportOf(spec *ast.ImportSpec) (Import, error) {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return Import{}, fmt.Errorf("invalid import path %s: %w", spec.Path.Value, err)
	}
	imp := Import{Path: path}
	if spec.Name != nil {
		imp.Name = spec.Name.Name
	}
	return imp, nil
}

// String returns the import like in an import declaration,
// for example `"fmt"` or `errs "github.com/domonda/go-errs"`.
func (imp Import) String() string {
	if imp.Name == "" {
		return strconv.Quote(imp.Path)
	}
	return imp.Name + " " + strconv.Quote(imp.Path)
}

// check returns an error if imp can't be added to a Go source file,
// for example for a quoted path like `"fmt"` instead of fmt.
func (imp Import) check() error {
	if err := checkImportPath(imp.Path); err != nil {
		return err
	}
	if imp.Name != "" && imp.Name != "." && imp.Name != "_" && !token.IsIdentifier(imp.Name) {
		return fmt.Errorf("invalid import name %q of import path %q", imp.Name, imp.Path)
	}
	return nil
}

// checkImportPath returns an error if path is not the import path of a package,
// for example a quoted or relative path, or a pattern like "all" or "std"
// that the go command expands to other packages.
func checkImportPath(path string) error {
	switch path {
	case "all", "cmd", "std", "tool", "work":
		return fmt.Errorf("%q is a package pattern of the go command, not an import path", path)
	}
	return module.CheckImportPath(path)
}

// Imports is a set of imports to be added to a Go source file.
// A nil Imports is an empty set, Add and AddNamed make the map.
//
// Example:
//
//	var imports Imports
//	imports.Add("fmt")
//	imports.AddNamed("errs", "github.com/domonda/go-errs")
//	formatted, err := FormatSource(source, imports)
type Imports map[Import]struct{}

// Add adds an import of the package with the import path
// that uses the package name, like `import "fmt"`.
func (imps *Imports) Add(path string) {
	imps.AddNamed("", path)
}

// AddNamed adds an import of the package with the import path
// that uses the import name, like `import errs "github.com/domonda/go-errs"`.
// The name can also be "." or "_".
func (imps *Imports) AddNamed(name, path string) {
	if *imps == nil {
		*imps = make(Imports)
	}
	(*imps)[Import{Name: name, Path: path}] = struct{}{}
}

// Sorted returns the imports sorted by path and name.
func (imps Imports) Sorted() []Import {
	return slices.SortedFunc(maps.Keys(imps), func(a, b Import) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Name, b.Name))
	})
}

// PackageLocation is the name and source directory of a Go package.
type PackageLocation struct {
	Name string // Package name like "fmt", can differ from the last element of the import path
	Dir  string // Absolute path of the package source directory
	Std  bool   // True for a standard library package
}

// LocatePackages finds the name and source directory of the packages
// with the import paths, resolved by the go command in the directory dir
// with a single go list call. The result maps every import path to its location.
//
// The import paths must not be quoted, relative like "./sub",
// or patterns like "all" or "std" of the go command.
// Returns an error that names every import path that can't be resolved,
// for example if no required module provides the package,
// or if the package has no Go files for the current platform.
//
// Example:
//
//	locs, err := LocatePackages("/my/project", "fmt", "github.com/domonda/go-errs")
//	// locs["fmt"].Name = "fmt"
//	// locs["fmt"].Dir = "/usr/local/go/src/fmt"
//	// locs["fmt"].Std = true
//
// The name that a file uses for an import is the explicit import name
// or the package name:
//
//	imp, err := ImportOf(importSpec)
//	...
//	name := cmp.Or(imp.Name, locs[imp.Path].Name)
func LocatePackages(dir string, importPaths ...string) (map[string]*PackageLocation, error) {
	importPaths = slices.Compact(slices.Sorted(slices.Values(importPaths)))
	var errs []error
	for _, path := range importPaths {
		if err := checkImportPath(path); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	locs := make(map[string]*PackageLocation, len(importPaths))
	if len(importPaths) == 0 {
		// packages.Load would load the package in dir without patterns
		return locs, nil
	}

	config := packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedModule,
		Dir:  dir,
	}
	pkgs, err := packages.Load(&config, importPaths...)
	if err != nil {
		return nil, err
	}
	pkgsByPath := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		pkgsByPath[pkg.PkgPath] = pkg
	}
	var goroot string // Read once for the first standard library candidate
	for _, path := range importPaths {
		pkg := pkgsByPath[path]
		switch {
		case pkg == nil:
			errs = append(errs, fmt.Errorf("could not locate package %q in %s", path, dir))
			continue
		case len(pkg.GoFiles) == 0 && len(pkg.Errors) == 0:
			// For example a directory with only test files
			errs = append(errs, fmt.Errorf("could not locate package %q in %s: no Go files", path, dir))
			continue
		case len(pkg.GoFiles) == 0:
			// packages.Load returns a package without files and with
			// package errors for an import path it can't resolve
			pkgErrs := make([]error, len(pkg.Errors))
			for i, pkgErr := range pkg.Errors {
				pkgErrs[i] = pkgErr
			}
			errs = append(errs, fmt.Errorf("could not locate package %q in %s: %w", path, dir, errors.Join(pkgErrs...)))
			continue
		}
		pkgDir := filepath.Dir(pkg.GoFiles[0])
		// Packages from GOROOT have no module and like for the go command
		// the first element of a standard library import path has no dot.
		// In GOPATH mode no package has a module, so such a package is only
		// from the standard library if it's in the GOROOT of the go command.
		firstPathElem, _, _ := strings.Cut(pkg.PkgPath, "/")
		std := pkg.Module == nil && !strings.Contains(firstPathElem, ".")
		if std {
			if goroot == "" {
				goroot, err = goEnvGOROOT(dir)
				if err != nil {
					return nil, err
				}
			}
			rel, err := filepath.Rel(filepath.Join(goroot, "src"), pkgDir)
			std = err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		locs[path] = &PackageLocation{
			Name: pkg.Name,
			Dir:  pkgDir,
			Std:  std,
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return locs, nil
}

// goEnvGOROOT returns the GOROOT of the go command run in dir.
// build.Default.GOROOT would be wrong for a tool built with -trimpath
// or a toolchain selected by GOTOOLCHAIN.
func goEnvGOROOT(dir string) (string, error) {
	cmd := exec.Command("go", "env", "GOROOT")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FormatSource formats Go source code like gofmt and adds the missing imports.
//
// If imports were added, all imports are grouped like goimports does:
// standard library, third-party, and import paths starting with
// one of the localImportPrefixes like "example.com/myproject".
// Without added imports the result equals gofmt output,
// existing import groups are not changed.
//
// Returns an error if the source is not valid Go or an import is invalid,
// for example for a quoted path like `"fmt"` instead of fmt.
//
// Example:
//
//	var imports Imports
//	imports.Add("fmt")
//	imports.AddNamed("errs", "github.com/domonda/go-errs")
//	formatted, err := FormatSource(source, imports, "example.com/myproject")
//
// Note: goimports reads the local import prefixes from the global variable
// LocalPrefix of golang.org/x/tools/imports. FormatSource sets and restores it
// while holding a mutex, other users of the variable are not synchronized.
func FormatSource(source []byte, imps Imports, localImportPrefixes ...string) ([]byte, error) {
	return formatSource(nextFileName(), source, imps, localImportPrefixes...)
}

// formatSource implements FormatSource
// with filename used for the positions in errors.
func formatSource(filename string, source []byte, imps Imports, localImportPrefixes ...string) ([]byte, error) {
	// A FileSet of its own, so that repeated calls don't grow a caller's FileSet
	localFset := token.NewFileSet()
	astFile, err := parser.ParseFile(localFset, filename, source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	importsAdded := false
	for _, imp := range imps.Sorted() {
		err = imp.check()
		if err != nil {
			return nil, err
		}
		if astutil.AddNamedImport(localFset, astFile, imp.Name, imp.Path) {
			importsAdded = true
		}
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(source)*2))
	err = format.Node(buf, localFset, astFile)
	if err != nil {
		return nil, err
	}
	if !importsAdded {
		return buf.Bytes(), nil
	}
	source = buf.Bytes()

	// TODO replace by something more efficient and elegant
	// than re-parsing everything and using the global variable imports.LocalPrefix
	importsLocalPrefixMtx.Lock()
	defer importsLocalPrefixMtx.Unlock()
	// Restore the global LocalPrefix for other users of golang.org/x/tools/imports
	defer func(localPrefix string) { imports.LocalPrefix = localPrefix }(imports.LocalPrefix)
	imports.LocalPrefix = strings.Join(localImportPrefixes, ",")
	return imports.Process(filename, source, &imports.Options{Comments: true, FormatOnly: true})
}

var importsLocalPrefixMtx sync.Mutex
