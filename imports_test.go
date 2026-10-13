package astvisit

import (
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/imports"
)

// Generators declare the imports they need next to their replacements
// with var declarations. Like for NodeReplacements, the zero value
// must be ready to use instead of panicking with a nil map.
func TestImports_ZeroValue(t *testing.T) {
	var imps Imports
	require.NotPanics(t, func() {
		imps.Add("fmt")
		imps.AddNamed("errs", "github.com/domonda/go-errs")
		imps.Add("fmt")
	})
	require.Equal(t, []Import{{Path: "fmt"}, {Name: "errs", Path: "github.com/domonda/go-errs"}}, imps.Sorted())
}

// ImportOf must accept every import spec that is valid Go,
// including raw string import paths, and keep the explicit name
// because code using the import refers to it by that name.
func TestImportOf(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "", "package p\n\nimport (\n\t\"fmt\"\n\terrs `github.com/domonda/go-errs`\n\t. \"strings\"\n)\n", parser.ImportsOnly)
	require.NoError(t, err)

	var result []Import
	for _, spec := range file.Imports {
		imp, err := ImportOf(spec)
		require.NoError(t, err)
		result = append(result, imp)
	}
	require.Equal(t, []Import{
		{Path: "fmt"},
		{Name: "errs", Path: "github.com/domonda/go-errs"},
		{Name: ".", Path: "strings"},
	}, result)
	require.Equal(t, `errs "github.com/domonda/go-errs"`, result[1].String())
}

// Without an added import FormatSource only has to gofmt the source.
// Also running goimports would take about twice as long and regroup imports
// that gofmt leaves alone, which causes diffs unrelated to the rewrite.
// When an import is added, goimports groups it with the existing imports.
func TestFormatSource_GoimportsOnlyForAddedImports(t *testing.T) {
	const src = "package p\n\nimport (\n\t\"github.com/stretchr/testify/require\"\n\t\"fmt\"\n)\n\nvar _, _ = fmt.Sprint, require.True\n"

	gofmted, err := format.Source([]byte(src))
	require.NoError(t, err)
	result, err := FormatSource([]byte(src), Imports{{Path: "fmt"}: {}})
	require.NoError(t, err)
	require.Equal(t, string(gofmted), string(result), "existing import must not run goimports")

	result, err = FormatSource([]byte(src), Imports{{Path: "strings"}: {}})
	require.NoError(t, err)
	require.Equal(t,
		"package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"github.com/stretchr/testify/require\"\n)\n\nvar _, _ = fmt.Sprint, require.True\n",
		string(result),
		"added import must be grouped by goimports",
	)
}

// Generated code refers to a named import by its name,
// so FormatSource must add the import with that name.
func TestFormatSource_NamedImports(t *testing.T) {
	var imps Imports
	imps.AddNamed("errs", "github.com/domonda/go-errs")
	imps.AddNamed("_", "embed")
	result, err := FormatSource([]byte("package p\n"), imps)
	require.NoError(t, err)
	require.Equal(t, "package p\n\nimport (\n\t_ \"embed\"\n\n\terrs \"github.com/domonda/go-errs\"\n)\n", string(result))
}

// Import paths used to be passed as quoted import lines.
// A quoted path or an invalid name must fail loudly
// instead of writing an import like "\"fmt\"" that doesn't compile.
func TestFormatSource_InvalidImport(t *testing.T) {
	for imp, wantErr := range map[Import]string{
		{Path: `"fmt"`}:                "malformed import path",
		{Path: "./sub"}:                "malformed import path",
		{Name: "my-fmt", Path: "fmt"}:  `invalid import name "my-fmt"`,
		{Name: "fmt", Path: "fmt fmt"}: "malformed import path",
	} {
		t.Run(imp.String(), func(t *testing.T) {
			result, err := FormatSource([]byte("package p\n"), Imports{imp: {}})
			require.ErrorContains(t, err, wantErr)
			require.Nil(t, result)
		})
	}
}

// FormatSource sets the global imports.LocalPrefix for goimports.
// Other users of golang.org/x/tools/imports in the process must get their value back.
func TestFormatSource_LocalPrefixRestored(t *testing.T) {
	orig := imports.LocalPrefix
	t.Cleanup(func() { imports.LocalPrefix = orig })
	imports.LocalPrefix = "orig.example"

	src := "package p\n\nimport (\n\t\"example.com/mine/x\"\n\t\"github.com/other/y\"\n)\n\nvar _, _ = x.A, y.B\n"
	result, err := FormatSource([]byte(src), Imports{{Path: "fmt"}: {}}, "example.com/mine")
	require.NoError(t, err)
	require.Equal(t, "package p\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/other/y\"\n\n\t\"example.com/mine/x\"\n)\n\nvar _, _ = x.A, y.B\n", string(result))
	require.Equal(t, "orig.example", imports.LocalPrefix)
}

// packages.Load reports import paths it can't resolve, like a package missing
// from go.mod or a directory without buildable Go files, as package errors
// instead of returning an error. LocatePackages must return them as one error
// that names every unresolved path, so that all can be fixed at once,
// instead of crashing the tool or returning incomplete locations.
func TestLocatePackages_Unresolvable(t *testing.T) {
	// The expected errors need the go command's default offline resolution,
	// a global GOFLAGS=-mod=mod would ask GOPROXY and fail differently
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":          "module example.com/m\n\ngo 1.26\n",
		"ignored/gen.go":  "//go:build ignore\n\npackage main\n",
		"empty/README.md": "No Go files\n",
	})

	require.NotPanics(t, func() {
		locs, err := LocatePackages(dir, "fmt", "github.com/does/not/exist", "example.com/m/ignored", "example.com/m/empty")
		require.ErrorContains(t, err, `could not locate package "github.com/does/not/exist"`)
		require.ErrorContains(t, err, "no required module provides package github.com/does/not/exist")
		require.ErrorContains(t, err, `could not locate package "example.com/m/ignored"`)
		require.ErrorContains(t, err, "build constraints exclude all Go files")
		require.ErrorContains(t, err, `could not locate package "example.com/m/empty"`)
		require.NotContains(t, err.Error(), `"fmt"`)
		require.Nil(t, locs)
	})

	// Resolvable packages are still located
	locs, err := LocatePackages(dir, "fmt")
	require.NoError(t, err)
	require.Equal(t, "fmt", locs["fmt"].Name)
	require.True(t, locs["fmt"].Std)
}

// go list also resolves relative directories and patterns like "std",
// but their packages can't be mapped back to the requested path
// and loading all packages of a pattern can take long.
// LocatePackages must reject them with a clear error before running go list.
func TestLocatePackages_NotAnImportPath(t *testing.T) {
	for path, wantErr := range map[string]string{
		"./sub":    "malformed import path",
		`"fmt"`:    "malformed import path",
		"/abs/dir": "malformed import path",
		"net/...":  "malformed import path",
		"all":      `"all" is a package pattern of the go command, not an import path`,
		"std":      `"std" is a package pattern of the go command, not an import path`,
	} {
		t.Run(path, func(t *testing.T) {
			locs, err := LocatePackages(".", path)
			require.ErrorContains(t, err, wantErr)
			require.Nil(t, locs)
		})
	}
}

// Without import paths packages.Load would load the package in dir,
// so LocatePackages must not call it and return an empty result.
func TestLocatePackages_NoImportPaths(t *testing.T) {
	locs, err := LocatePackages(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, locs)
}

// Std must tell standard library packages from module packages
// independent of the GOROOT the tool was built with,
// which is empty for a tool built with -trimpath
// and differs from the GOROOT of a toolchain selected by GOTOOLCHAIN.
func TestLocatePackages_Std(t *testing.T) {
	goroot := build.Default.GOROOT
	build.Default.GOROOT = "" // Like in a tool built with -trimpath
	t.Cleanup(func() { build.Default.GOROOT = goroot })

	wantStd := map[string]bool{
		"fmt":                                 true,
		"net/http":                            true,
		"github.com/stretchr/testify/require": false,
		"github.com/ungerik/go-astvisit":      false,
	}
	var importPaths []string
	for importPath := range wantStd {
		importPaths = append(importPaths, importPath)
	}
	locs, err := LocatePackages(".", importPaths...)
	require.NoError(t, err)
	require.Len(t, locs, len(wantStd))
	for importPath, std := range wantStd {
		require.Equal(t, std, locs[importPath].Std, importPath)
	}
}

// Local tools often use module paths without a dot. Their packages are not
// standard library packages, which only the module of the package can tell.
func TestLocatePackages_DotlessModuleIsNotStd(t *testing.T) {
	// A global GOWORK would make go list reject the module in the temp dir
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":     "module myapp\n\ngo 1.26\n",
		"sub/sub.go": "package sub\n",
	})

	locs, err := LocatePackages(dir, "myapp/sub")
	require.NoError(t, err)
	require.Equal(t, "sub", locs["myapp/sub"].Name)
	require.Equal(t, filepath.Join(dir, "sub"), locs["myapp/sub"].Dir)
	require.False(t, locs["myapp/sub"].Std)
}

// A package with only test files has no Go files and no package errors.
// The error must still read cleanly, without a %!w(<nil>) placeholder.
func TestLocatePackages_TestOnlyPackageError(t *testing.T) {
	// A global GOWORK would make go list reject the module in the temp dir
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":             "module example.com/m\n\ngo 1.26\n",
		"onlytest/x_test.go": "package onlytest\n",
	})

	locs, err := LocatePackages(dir, "example.com/m/onlytest")
	require.Nil(t, locs)
	require.ErrorContains(t, err, "no Go files")
	require.NotContains(t, err.Error(), "%!w")
}

// In GOPATH mode no package has a module, so a package with a dotless
// import path is not necessarily from the standard library.
func TestLocatePackages_StdInGOPATHMode(t *testing.T) {
	gopath := t.TempDir()
	writeTestFiles(t, gopath, map[string]string{"src/myapp/sub/sub.go": "package sub\n"})
	t.Setenv("GO111MODULE", "off")
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	projectDir := filepath.Join(gopath, "src", "myapp")

	locs, err := LocatePackages(projectDir, "myapp/sub", "fmt")
	require.NoError(t, err)
	require.False(t, locs["myapp/sub"].Std)
	require.True(t, locs["fmt"].Std)
}
