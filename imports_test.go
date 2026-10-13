package astvisit

import (
	"go/build"
	"go/format"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/imports"
)

// Long runs call FormatFileWithImports for every file.
// Adding every throwaway parse to the caller's FileSet would grow it without bounds.
func TestFormatFileWithImports_DoesNotGrowFileSet(t *testing.T) {
	fset := token.NewFileSet()
	base := fset.Base()
	_, err := FormatFileWithImports(fset, []byte("package p\n"), Imports{`"fmt"`: {}})
	require.NoError(t, err)
	require.Equal(t, base, fset.Base())
}

// Without an added import FormatFileWithImports only has to gofmt the source.
// Also running goimports would take about twice as long and regroup imports
// that gofmt leaves alone, which causes diffs unrelated to the rewrite.
// When an import is added, goimports groups it with the existing imports.
func TestFormatFileWithImports_GoimportsOnlyForAddedImports(t *testing.T) {
	const src = "package p\n\nimport (\n\t\"github.com/stretchr/testify/require\"\n\t\"fmt\"\n)\n\nvar _, _ = fmt.Sprint, require.True\n"

	gofmted, err := format.Source([]byte(src))
	require.NoError(t, err)
	result, err := FormatFileWithImports(token.NewFileSet(), []byte(src), Imports{`"fmt"`: {}})
	require.NoError(t, err)
	require.Equal(t, string(gofmted), string(result), "existing import must not run goimports")

	result, err = FormatFileWithImports(token.NewFileSet(), []byte(src), Imports{`"strings"`: {}})
	require.NoError(t, err)
	require.Equal(t,
		"package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"github.com/stretchr/testify/require\"\n)\n\nvar _, _ = fmt.Sprint, require.True\n",
		string(result),
		"added import must be grouped by goimports",
	)
}

// packages.Load reports import paths it can't resolve, like a package missing
// from go.mod or a directory without buildable Go files, as package errors
// instead of returning an error. LocatePackage must return them as error
// that explains why, instead of crashing the tool.
func TestLocatePackage_Unresolvable(t *testing.T) {
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

	for importPath, wantErr := range map[string]string{
		`"github.com/does/not/exist"`: "no required module provides package github.com/does/not/exist",
		"./ignored":                   "build constraints exclude all Go files",
		"./empty":                     "no Go files",
	} {
		t.Run(importPath, func(t *testing.T) {
			require.NotPanics(t, func() {
				loc, err := LocatePackage(dir, importPath)
				require.ErrorContains(t, err, wantErr)
				require.Nil(t, loc)
			})
		})
	}

	// Resolvable packages are still located
	loc, err := LocatePackage(dir, "fmt")
	require.NoError(t, err)
	require.Equal(t, "fmt", loc.PkgName)
	require.True(t, loc.Std)
}

// Std must tell standard library packages from module packages
// independent of the GOROOT the tool was built with,
// which is empty for a tool built with -trimpath
// and differs from the GOROOT of a toolchain selected by GOTOOLCHAIN.
func TestLocatePackage_Std(t *testing.T) {
	goroot := build.Default.GOROOT
	build.Default.GOROOT = "" // Like in a tool built with -trimpath
	t.Cleanup(func() { build.Default.GOROOT = goroot })

	for importPath, wantStd := range map[string]bool{
		"fmt":                                 true,
		"net/http":                            true,
		"github.com/stretchr/testify/require": false,
		"github.com/ungerik/go-astvisit":      false,
	} {
		t.Run(importPath, func(t *testing.T) {
			loc, err := LocatePackage(".", importPath)
			require.NoError(t, err)
			require.Equal(t, wantStd, loc.Std)
		})
	}
}

// Local tools often use module paths without a dot. Their packages are not
// standard library packages, which only the module of the package can tell.
func TestLocatePackage_DotlessModuleIsNotStd(t *testing.T) {
	// A global GOWORK would make go list reject the module in the temp dir
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":     "module myapp\n\ngo 1.26\n",
		"sub/sub.go": "package sub\n",
	})

	loc, err := LocatePackage(dir, "myapp/sub")
	require.NoError(t, err)
	require.Equal(t, "sub", loc.PkgName)
	require.False(t, loc.Std)
}

// A package with only test files has no Go files and no package errors.
// The error must still read cleanly, without a %!w(<nil>) placeholder.
func TestLocatePackage_TestOnlyPackageError(t *testing.T) {
	// A global GOWORK would make go list reject the module in the temp dir
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":             "module example.com/m\n\ngo 1.26\n",
		"onlytest/x_test.go": "package onlytest\n",
	})

	loc, err := LocatePackage(dir, "./onlytest")
	require.Nil(t, loc)
	require.ErrorContains(t, err, "no Go files")
	require.NotContains(t, err.Error(), "%!w")
}

// FormatFileWithImports sets the global imports.LocalPrefix for goimports.
// Other users of golang.org/x/tools/imports in the process must get their value back.
func TestFormatFileWithImports_LocalPrefixRestored(t *testing.T) {
	orig := imports.LocalPrefix
	t.Cleanup(func() { imports.LocalPrefix = orig })
	imports.LocalPrefix = "orig.example"

	src := "package p\n\nimport (\n\t\"example.com/mine/x\"\n\t\"github.com/other/y\"\n)\n\nvar _, _ = x.A, y.B\n"
	result, err := FormatFileWithImports(token.NewFileSet(), []byte(src), Imports{`"fmt"`: {}}, "example.com/mine")
	require.NoError(t, err)
	require.Equal(t, "package p\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/other/y\"\n\n\t\"example.com/mine/x\"\n)\n\nvar _, _ = x.A, y.B\n", string(result))
	require.Equal(t, "orig.example", imports.LocalPrefix)
}

// In GOPATH mode no package has a module, so a package with a dotless
// import path is not necessarily from the standard library.
func TestLocatePackage_StdInGOPATHMode(t *testing.T) {
	gopath := t.TempDir()
	writeTestFiles(t, gopath, map[string]string{"src/myapp/sub/sub.go": "package sub\n"})
	t.Setenv("GO111MODULE", "off")
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	projectDir := filepath.Join(gopath, "src", "myapp")

	for importPath, wantStd := range map[string]bool{
		"myapp/sub": false,
		"fmt":       true,
	} {
		t.Run(importPath, func(t *testing.T) {
			loc, err := LocatePackage(projectDir, importPath)
			require.NoError(t, err)
			require.Equal(t, wantStd, loc.Std)
		})
	}
}
