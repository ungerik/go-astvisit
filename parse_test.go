package astvisit

import (
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTestFiles writes files with paths relative to dir
// and creates missing directories.
func writeTestFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

// fileNames returns the sorted base names of the file paths in files.
func fileNames(files map[string]*ast.File) []string {
	names := make([]string, 0, len(files))
	for path := range files {
		names = append(names, filepath.Base(path))
	}
	slices.Sort(names)
	return names
}

// Tools built on ParsePackage must see code in main packages,
// for example enum types declared in a command.
func TestParsePackage_MainPackage(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	pkg, err := ParsePackage(token.NewFileSet(), dir, nil)
	require.NoError(t, err)
	require.Equal(t, "main", pkg.Name)
	require.Equal(t, []string{"main.go"}, fileNames(pkg.Files))
}

// A `//go:build ignore` generator file, often with package main,
// is not part of the package and must neither cause a
// "2 packages found" error nor be parsed at all,
// because such files may be templates that aren't valid Go.
func TestParsePackage_IgnoresFilesExcludedFromBuild(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"enum.go":         "package colors\n\ntype Color int\n",
		"gen.go":          "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
		"template.go":     "//go:build ignore\n\npackage {{.Package}}\n",
		"integration.go":  "//go:build integration\n\npackage colors_integration\n",
		"_scratch.go":     "package scratch\n",
		".hidden.go":      "package hidden\n",
		"gen_generate.go": "//go:build ignore && linux\n\npackage main\n",
	})

	pkg, err := ParsePackage(token.NewFileSet(), dir, nil)
	require.NoError(t, err)
	require.Equal(t, "colors", pkg.Name)
	require.Equal(t, []string{"enum.go"}, fileNames(pkg.Files))
}

// Files for other platforms than the current one are part of the package
// and must still be parsed, so that a rewrite covers all of them.
func TestParsePackage_KeepsFilesOfAllPlatforms(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go":                "package p\n",
		"a_windows.go":        "package p\n",
		"a_plan9_386.go":      "package p\n",
		"linux_386.go":        "//go:build linux && 386\n\npackage p\n",
		"nonlinux.go":         "//go:build !linux\n\npackage p\n",
		"nocgo.go":            "//go:build !cgo\n\npackage p\n",
		"cgo.go":              "//go:build cgo\n\npackage p\n\nimport \"C\"\n",
		"unix.go":             "//go:build unix\n\npackage p\n",
		"legacy_build_tag.go": "// +build windows\n\npackage p\n",
	})

	// Like a tool running with CGO_ENABLED=1 or CGO_ENABLED=0,
	// both must keep the cgo and the !cgo files
	cgoEnabled := build.Default.CgoEnabled
	t.Cleanup(func() { build.Default.CgoEnabled = cgoEnabled })
	for _, cgo := range []bool{true, false} {
		build.Default.CgoEnabled = cgo
		pkg, err := ParsePackage(token.NewFileSet(), dir, nil)
		require.NoError(t, err)
		require.Equal(t, "p", pkg.Name)
		require.Equal(t,
			[]string{"a.go", "a_plan9_386.go", "a_windows.go", "cgo.go", "legacy_build_tag.go", "linux_386.go", "nocgo.go", "nonlinux.go", "unix.go"},
			fileNames(pkg.Files),
			"CgoEnabled: %t", cgo,
		)
	}
}

// Buildable files with different package names are still an error,
// listing the names in a deterministic order.
func TestParsePackage_MultiplePackagesError(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"b.go": "package b\n",
		"a.go": "package a\n",
	})

	_, err := ParsePackage(token.NewFileSet(), dir, nil)
	require.ErrorContains(t, err, "2 packages found in "+dir+": a, b")
}

// A malformed //go:build line is an error for the go command.
// Skipping the file silently would leave it out of every rewrite
// without any hint, so ParsePackage must report it with the file name.
func TestParsePackage_MalformedBuildConstraintError(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go":   "package p\n",
		"bad.go": "//go:build (linux\n\npackage p\n",
	})

	_, err := ParsePackage(token.NewFileSet(), dir, nil)
	require.ErrorContains(t, err, "bad.go: parsing //go:build line")
}

// Rewrite relies on the filter to exclude _test.go files,
// so a file that the filter rejects must never be parsed.
func TestParsePackage_Filter(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go":      "package p\n",
		"a_test.go": "package p_test\n",
	})

	pkg, err := ParsePackage(token.NewFileSet(), dir, filterOutTests)
	require.NoError(t, err)
	require.Equal(t, []string{"a.go"}, fileNames(pkg.Files))

	_, err = ParsePackage(token.NewFileSet(), t.TempDir(), nil)
	require.ErrorIs(t, err, ErrPackageNotFound)
}

// knownOS and knownArch are copied from Go's internal/syslist package.
// A port that a new Go release adds must be added to them,
// else files constrained to that port are silently skipped by every rewrite.
func TestKnownOSArch_CoverGoDistList(t *testing.T) {
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not found:", err)
	}
	out, err := exec.Command(goCmd, "tool", "dist", "list").Output()
	require.NoError(t, err)
	for _, port := range strings.Fields(string(out)) {
		goos, goarch, _ := strings.Cut(port, "/")
		require.Contains(t, knownOS, goos, port)
		require.Contains(t, knownArch, goarch, port)
	}
}
