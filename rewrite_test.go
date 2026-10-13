package astvisit

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func ExampleRewrite() {
	cwd, _ := os.Getwd()
	cwd += string(os.PathSeparator)
	// Use verboseWriter to print encountered file paths
	err := Rewrite(
		"./...",
		nil, // verboseOut
		nil, // resultOut
		func(fset *token.FileSet, filePkg *ast.Package, astFile *ast.File, filePath string, verboseWriter io.Writer) ([]byte, error) {
			info, err := os.Stat(filePath)
			if err != nil {
				return nil, err
			}
			if info.IsDir() {
				return nil, errors.New("listed directory")
			}
			if filepath.Ext(filePath) != ".go" {
				return nil, errors.New("listed non .go file")
			}
			_, err = fmt.Println(strings.TrimPrefix(filePath, cwd))
			return nil, err
		})
	if err != nil {
		panic(err)
	}

	// Output:
	// cursor.go
	// doc.go
	// errors.go
	// gomod.go
	// imports.go
	// nodereplacements.go
	// nodetype.go
	// parse.go
	// path.go
	// pathitem.go
	// rewrite.go
	// tostring.go
	// visit.go
	// visitor.go
	// visitorimpl.go
	// test/dummy.go
}

// sinceVisitor collects a replacement of time.Now().Sub(t) with time.Since(t)
// for every call, like the codemod of docs/tutorial-write-a-codemod.md
type sinceVisitor struct {
	VisitorImpl
	replacements NodeReplacements
}

func (v *sinceVisitor) VisitCallExpr(call *ast.CallExpr, cursor Cursor) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sub" || len(call.Args) != 1 || ExprString(sel.X) != "time.Now()" {
		return true
	}
	v.replacements.AddReplacement(call, &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Since")},
		Args: call.Args,
	})
	// Don't visit the children, a replacement within call would overlap
	return false
}

// RewriteDryRun lists the files a codemod would change,
// for example to check in CI that no file needs the codemod.
// It uses the codemod of docs/tutorial-write-a-codemod.md,
// so an API change that breaks the docs' approach breaks this example.
func ExampleRewriteDryRun() {
	dir, err := os.MkdirTemp("", "example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	files := map[string]string{
		"age.go":   "package p\n\nimport \"time\"\n\nfunc Age(t time.Time) time.Duration {\n\treturn time.Now().Sub(t)\n}\n",
		"fixed.go": "package p\n\nimport \"time\"\n\nfunc Elapsed(t time.Time) time.Duration {\n\treturn time.Since(t)\n}\n",
	}
	for name, src := range files {
		err = os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600)
		if err != nil {
			panic(err)
		}
	}

	fixFile := FileReplacementsFunc(func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
		visitor := &sinceVisitor{}
		Visit(astFile, visitor, nil)
		return visitor.replacements, nil, nil
	})
	changedFiles, err := RewriteDryRun(filepath.Join(dir, "..."), nil, fixFile.PackageFunc())
	if err != nil {
		panic(err)
	}
	for _, filePath := range changedFiles {
		fmt.Println(filepath.Base(filePath))
	}

	// Output:
	// age.go
}

// The go command ignores testdata and directories starting with "_" or ".",
// and ./... patterns don't match vendor directories.
// Fixtures in testdata are often invalid or intentionally unformatted Go
// and must neither make a recursive rewrite fail nor be rewritten,
// and vendored third-party code must not be changed.
func TestRewrite_SkipsDirectoriesIgnoredByGoCommand(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go":                      "package a\n",
		"a_test.go":                 "package a\n",
		"sub/b.go":                  "package sub\n",
		"sub/b_test.go":             "package sub_test\n",
		"testdata/broken.go":        "package broken\n\nfunc {\n",
		"testdata/fixture/f.go":     "package fixture\n",
		"_old/old.go":               "package old\n",
		".hidden/h.go":              "package hidden\n",
		"node_modules/m/m.go":       "package m\n",
		"vendor/example.com/v/v.go": "package v\n",
	})

	require.Equal(t, []string{"a.go", "sub/b.go"}, visitedFiles(t, dir, filepath.Join(dir, "...")))
}

// Windows checkouts with core.autocrlf have CRLF line endings.
// Replacements that change nothing must not rewrite such files,
// and real changes must keep the CRLF line endings,
// otherwise every run would produce a diff of all lines.
func TestRewriteWithReplacements_CRLF(t *testing.T) {
	const src = "package p\r\n\r\nfunc a() {}\r\n\r\nfunc b() {}\r\n\r\nfunc c() {}\r\n"

	t.Run("no-op replacement keeps file", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "p.go")
		require.NoError(t, os.WriteFile(filePath, []byte(src), 0o600))

		err := RewriteWithReplacements(filePath, nil, nil, false,
			func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
				var repls NodeReplacements
				repls.AddReplacement(astFile.Decls[0], "func a() {}")
				return repls, nil, nil
			},
		)
		require.NoError(t, err)
		requireFileContent(t, src, filePath)
	})

	t.Run("change keeps CRLF", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "p.go")
		require.NoError(t, os.WriteFile(filePath, []byte(src), 0o600))

		err := RewriteWithReplacements(filePath, nil, nil, false,
			func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
				var repls NodeReplacements
				repls.AddReplacement(astFile.Decls[0], "func x() {}\n")
				repls.AddRemoval(astFile.Decls[1])
				return repls, nil, nil
			},
		)
		require.NoError(t, err)
		requireFileContent(t, "package p\r\n\r\nfunc x() {}\r\n\r\nfunc c() {}\r\n", filePath)
	})
}

// The replacements are computed from the parsed AST, so they must be applied
// to exactly the parsed source. Reading the file again doubles the I/O and
// applies the positions to different bytes if the file changed in between.
func TestRewriteWithReplacements_UsesParsedSource(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "p.go")
	require.NoError(t, os.WriteFile(filePath, []byte("package p\n\nfunc a() {}\n"), 0o600))

	var result bytes.Buffer
	err := RewriteWithReplacements(filePath, nil, &result, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			// Simulate a change of the file after it was parsed
			err := os.WriteFile(filePath, []byte("package p\n\n// Changed\nfunc a() {}\n"), 0o600)
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0].(*ast.FuncDecl).Name, "b")
			return repls, nil, err
		},
	)
	require.NoError(t, err)
	require.Equal(t, "package p\n\nfunc b() {}\n", result.String())
}

// Replacements that only differ from the source in formatting change nothing.
// Rewriting the file anyway would produce unrelated diffs, for example when
// the file isn't gofmt-ed, and tools would need their own no-op detection.
func TestRewriteWithReplacements_FormattingOnlyIsUnchanged(t *testing.T) {
	const src = "package p\n\nfunc a()  {  }\n\nvar x=1\n"
	filePath := filepath.Join(t.TempDir(), "p.go")
	require.NoError(t, os.WriteFile(filePath, []byte(src), 0o600))

	var verbose bytes.Buffer
	err := RewriteWithReplacements(filePath, &verbose, nil, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0], "func a() {}")
			return repls, Imports{{Path: "fmt"}: {}}, nil
		},
	)
	require.NoError(t, err)
	requireFileContent(t, src, filePath)
	require.Contains(t, verbose.String(), "no changes in file: "+filePath)

	// A real change formats the whole file
	err = RewriteWithReplacements(filePath, nil, nil, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0], "func b() {}")
			return repls, nil, nil
		},
	)
	require.NoError(t, err)
	requireFileContent(t, "package p\n\nfunc b() {}\n\nvar x = 1\n", filePath)
}

// CI runs a check mode that lists the files a rewrite would change
// without writing them.
func TestRewriteDryRun(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.go":     "package p\n\nfunc a() {}\n",
		"b.go":     "package p\n\nfunc b() {}\n",
		"sub/c.go": "package sub\n\nfunc a() {}\n",
	}
	writeTestFiles(t, dir, files)

	renameA := FileReplacementsFunc(func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
		var repls NodeReplacements
		for _, decl := range astFile.Decls {
			if name := decl.(*ast.FuncDecl).Name; name.Name == "a" {
				repls.AddReplacement(name, "renamed")
			} else {
				repls.AddReplacement(name, name.Name) // no-op
			}
		}
		return repls, nil, nil
	})
	changed, err := RewriteDryRun(filepath.Join(dir, "..."), nil, renameA.PackageFunc())
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(dir, "a.go"), filepath.Join(dir, "sub", "c.go")}, changed)
	for name, content := range files {
		requireFileContent(t, content, filepath.Join(dir, name))
	}
}

// A codemod that returns the formatted source of every file also returns
// the files it doesn't change, with LF line endings even in a CRLF checkout.
// The CI check must only report files whose content really changes,
// else it fails for every file of a Windows checkout with core.autocrlf.
func TestRewriteDryRun_UnchangedContentNotReported(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"crlf.go":        "package p\r\n\r\nfunc a() {}\r\n",
		"lf.go":          "package p\n\nfunc b() {}\n",
		"unformatted.go": "package p\n\nfunc c()  {  }\n",
	})

	gofmt := RewriteFileFunc(func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
		var buf bytes.Buffer
		err := format.Node(&buf, fset, astFile)
		return buf.Bytes(), err
	})
	changed, err := RewriteDryRun(dir, nil, gofmt.PackageFunc())
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(dir, "unformatted.go")}, changed)
}

// A tool must not leave a package half rewritten when one of its files fails.
func TestRewrite_NoFilesWrittenOnError(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go": "package p\n",
		"b.go": "package p\n",
	})

	err := Rewrite(dir, nil, nil,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
			if filepath.Base(filePath) == "b.go" {
				return nil, errors.New("b.go failed")
			}
			return []byte("package p\n\nvar rewritten = true\n"), nil
		},
	)
	require.EqualError(t, err, "b.go failed")
	requireFileContent(t, "package p\n", filepath.Join(dir, "a.go"))
}

// The package callback gets all files with their sources once,
// so a tool can compute a package model once and write all files together.
func TestRewritePackages(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.go":     "package p\n\nfunc a() {}\n",
		"b.go":     "package p\n\nfunc b() {}\n",
		"sub/c.go": "package sub\n",
	}
	writeTestFiles(t, dir, files)

	var pkgNames []string
	err := RewritePackages(filepath.Join(dir, "..."), nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		pkgNames = append(pkgNames, pkg.Name)
		require.Len(t, pkg.Files, len(pkg.FilePaths))
		for _, filePath := range pkg.FilePaths {
			rel, err := filepath.Rel(dir, filePath)
			require.NoError(t, err)
			require.Equal(t, files[filepath.ToSlash(rel)], string(pkg.Sources[filePath]))
			require.NotNil(t, pkg.Files[filePath])
		}
		if pkg.Name != "p" {
			return nil, nil
		}
		return map[string][]byte{
			filepath.Join(dir, "a.go"): []byte("package p\n\n// changed\n"),
			filepath.Join(dir, "b.go"): pkg.Sources[filepath.Join(dir, "b.go")],
		}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"p", "sub"}, pkgNames)
	requireFileContent(t, "package p\n\n// changed\n", filepath.Join(dir, "a.go"))
	requireFileContent(t, files["b.go"], filepath.Join(dir, "b.go"))

	t.Run("error writes nothing", func(t *testing.T) {
		err := RewritePackages(dir, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
			return map[string][]byte{filepath.Join(dir, "b.go"): []byte("package p\n")}, errors.New("failed")
		})
		require.EqualError(t, err, "failed")
		requireFileContent(t, files["b.go"], filepath.Join(dir, "b.go"))
	})

	t.Run("unknown file is an error", func(t *testing.T) {
		err := RewritePackages(dir, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
			return map[string][]byte{"b.go": []byte("package p\n")}, nil
		})
		require.ErrorContains(t, err, "package p has no file b.go")
		requireFileContent(t, files["b.go"], filepath.Join(dir, "b.go"))
	})
}

// Rewriting a single file passes the whole package for context,
// but must only change that file.
func TestRewritePackages_SingleFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go": "package p\n",
		"b.go": "package p\n",
	})
	aPath, bPath := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")

	err := RewritePackages(aPath, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		require.Len(t, pkg.Files, 2)
		require.Equal(t, []string{aPath}, pkg.FilePaths)
		return map[string][]byte{aPath: []byte("package p\n\n// a\n")}, nil
	})
	require.NoError(t, err)
	requireFileContent(t, "package p\n\n// a\n", aPath)
	requireFileContent(t, "package p\n", bPath)

	var visited []string
	err = Rewrite(aPath, nil, nil,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
			require.Len(t, pkg.Files, 2)
			visited = append(visited, filePath)
			return nil, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []string{aPath}, visited)
}

// Debug output contains conflict markers and is not Go code.
// It must only be written to resultOut and never overwrite a source file,
// else the next run fails to parse the corrupted file.
func TestRewriteWithReplacements_DebugNeverWritesFiles(t *testing.T) {
	const src = "package p\n\nvar x = 1\n"
	filePath := filepath.Join(t.TempDir(), "p.go")
	writeTestFiles(t, filepath.Dir(filePath), map[string]string{"p.go": src})

	replaceX := func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
		var repls NodeReplacements
		repls.AddReplacement(astFile.Decls[0], "var x = 2", "replace-x")
		return repls, nil, nil
	}

	err := RewriteWithReplacements(filePath, nil, nil, true, replaceX)
	require.Error(t, err)
	requireFileContent(t, src, filePath)

	var resultOut bytes.Buffer
	err = RewriteWithReplacements(filePath, nil, &resultOut, true, replaceX)
	require.NoError(t, err)
	require.Contains(t, resultOut.String(), "<<<<<<< ")
	require.Contains(t, resultOut.String(), ">>>>>>> replace-x")
	requireFileContent(t, src, filePath)
}

// Generators are often files with a //go:build ignore constraint
// next to the package they generate code for. Like "go run gen.go",
// rewriting such a file by its path must work, but a recursive rewrite
// must still skip it.
func TestRewrite_ExplicitFileExcludedFromBuild(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"lib.go": "package lib\n",
		"gen.go": "//go:build ignore\n\npackage main\n",
	})
	genPath := filepath.Join(dir, "gen.go")

	err := RewritePackages(genPath, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		require.Equal(t, "main", pkg.Name)
		require.Equal(t, []string{genPath}, pkg.FilePaths)
		require.Equal(t, []string{"gen.go"}, fileNames(pkg.Files))
		return map[string][]byte{genPath: []byte("//go:build ignore\n\npackage main\n\n// changed\n")}, nil
	})
	require.NoError(t, err)
	requireFileContent(t, "//go:build ignore\n\npackage main\n\n// changed\n", genPath)

	require.Equal(t, []string{"lib.go"}, visitedFiles(t, dir, filepath.Join(dir, "...")))
}

// Test files are never rewritten, also not when a test file is passed as path.
// A test file behind a custom build tag would otherwise be parsed
// as a package of its own and silently rewritten by a codemod.
func TestRewrite_ExplicitTestFileIsError(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.go":                "package p\n",
		"a_test.go":           "package p\n",
		"integration_test.go": "//go:build integration\n\npackage p\n",
	}
	writeTestFiles(t, dir, files)

	for _, name := range []string{"a_test.go", "integration_test.go"} {
		t.Run(name, func(t *testing.T) {
			filePath := filepath.Join(dir, name)
			err := Rewrite(filePath, nil, nil,
				func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
					return []byte("package p\n\n// rewritten\n"), nil
				},
			)
			require.EqualError(t, err, "test file "+filePath+" is not rewritten")
			requireFileContent(t, files[name], filePath)
		})
	}
}

// When replacements produce invalid Go, the error must name the file.
// A synthetic file name leaves the user guessing which of the many files
// of a recursive rewrite was broken by the replacements.
func TestRewriteWithReplacements_InvalidResultNamesFile(t *testing.T) {
	const src = "package p\n\nfunc a() {}\n"
	filePath := filepath.Join(t.TempDir(), "p.go")
	writeTestFiles(t, filepath.Dir(filePath), map[string]string{"p.go": src})

	err := RewriteWithReplacements(filePath, nil, nil, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0], "func {")
			return repls, nil, nil
		},
	)
	require.ErrorContains(t, err, filePath+":3:6: expected 'IDENT', found '{'")
	requireFileContent(t, src, filePath)
}

// visitedFiles returns the slash separated paths relative to dir
// of the files that Rewrite calls its function for with path.
func visitedFiles(t *testing.T, dir, path string) []string {
	t.Helper()
	var visited []string
	err := Rewrite(path, nil, nil,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
			rel, err := filepath.Rel(dir, filePath)
			visited = append(visited, filepath.ToSlash(rel))
			return nil, err
		},
	)
	require.NoError(t, err)
	return visited
}

// Like the go command for ./... patterns, a recursive rewrite must not
// change the code of nested modules, which are versioned separately,
// or of directories that an ignore directive in go.mod excludes,
// like generated code or files of other tools.
func TestRewrite_SkipsNestedModulesAndIgnoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":            "module example.com/m\n\ngo 1.25\n\nignore (\n\t./gen\n\tfixtures\n)\n",
		"a.go":              "package a\n",
		"gen/g.go":          "package gen\n",
		"fixtures/f.go":     "package fixtures\n",
		"nested/go.mod":     "module example.com/nested\n",
		"nested/n.go":       "package nested\n",
		"broken/go.mod":     "invalid go.mod of a nested module that is never read\n",
		"broken/b.go":       "package broken\n",
		"sub/s.go":          "package sub\n",
		"sub/gen/g.go":      "package gen\n",
		"sub/fixtures/f.go": "package fixtures\n",
	})

	// "./gen" only ignores gen in the module root, "fixtures" at any depth
	require.Equal(t, []string{"a.go", "sub/s.go", "sub/gen/g.go"}, visitedFiles(t, dir, filepath.Join(dir, "...")))
	// The ignore directives of the enclosing module apply to a sub-directory path
	require.Equal(t, []string{"sub/s.go", "sub/gen/g.go"}, visitedFiles(t, dir, filepath.Join(dir, "sub", "...")))
	// A nested module is rewritten when its directory is the path
	require.Equal(t, []string{"nested/n.go"}, visitedFiles(t, dir, filepath.Join(dir, "nested", "...")))
}

// Naming a directory that an ignore directive matches rewrites it on purpose.
// That must include its sub-directories, else a rewrite of gen/... would
// silently leave gen/sub stale. Ignore directives that don't match
// the named directory still apply below it.
func TestRewrite_NamedIgnoredDirectoryIncludesSubDirectories(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":            "module example.com/m\n\ngo 1.25\n\nignore (\n\t./gen\n\tfixtures\n)\n",
		"gen/g.go":          "package gen\n",
		"gen/sub/s.go":      "package sub\n",
		"gen/fixtures/f.go": "package fixtures\n",
	})

	require.Equal(t, []string{"gen/g.go", "gen/sub/s.go"}, visitedFiles(t, dir, filepath.Join(dir, "gen", "...")))
}

// The docs promise that a recursive rewrite selects the packages that
// go list ./... matches, except node_modules. The ignore directive matching
// is copied from the go command, so compare with the go command itself
// to notice when a Go release changes the rules.
func TestRewrite_SelectsPackagesLikeGoList(t *testing.T) {
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not found:", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	writeTestFiles(t, dir, map[string]string{
		"go.mod":            "module example.com/m\n\ngo 1.25\n\nignore (\n\t./gen\n\tfixtures\n\tweb/data\n)\n",
		"a.go":              "package m\n",
		"gen/g.go":          "package gen\n",
		"gen/sub/s.go":      "package sub\n",
		"pkg/p.go":          "package pkg\n",
		"pkg/gen/g.go":      "package gen\n",
		"pkg/fixtures/f.go": "package fixtures\n",
		"fixtures/f.go":     "package fixtures\n",
		"web/data/d.go":     "package data\n",
		"app/web/data/d.go": "package data\n",
		"web/data_old/d.go": "package data\n",
		"nested/go.mod":     "module example.com/m/nested\n",
		"nested/n.go":       "package nested\n",
		"testdata/t.go":     "package testdata\n",
		"_old/o.go":         "package old\n",
		".hidden/h.go":      "package hidden\n",
	})

	cmd := exec.Command(goCmd, "list", "-f", "{{.Dir}}", "./...")
	cmd.Dir = dir
	// The go command's defaults, independent of the developer's environment
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOPROXY=off")
	out, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		t.Fatalf("go list failed: %s", exitErr.Stderr)
	}
	require.NoError(t, err)
	var goListDirs []string
	for _, pkgDir := range strings.Fields(string(out)) {
		pkgDir, err = filepath.EvalSymlinks(pkgDir)
		require.NoError(t, err)
		rel, err := filepath.Rel(dir, pkgDir)
		require.NoError(t, err)
		goListDirs = append(goListDirs, filepath.ToSlash(rel))
	}
	slices.Sort(goListDirs)

	var rewriteDirs []string
	for _, file := range visitedFiles(t, dir, filepath.Join(dir, "...")) {
		rewriteDirs = append(rewriteDirs, filepath.ToSlash(filepath.Dir(file)))
	}
	slices.Sort(rewriteDirs)
	rewriteDirs = slices.Compact(rewriteDirs)

	require.Equal(t, []string{".", "pkg", "pkg/gen", "web/data_old"}, goListDirs, "go list")
	require.Equal(t, goListDirs, rewriteDirs, "Rewrite")
}

// A repository can hold several modules without a go.mod at its root.
// A recursive rewrite of the repository must reach all of these modules,
// while each of them still skips its nested modules and ignored directories.
func TestRewrite_EntersModulesFromOutsideOfModules(t *testing.T) {
	dir := t.TempDir()
	requireOutsideModules(t, dir)
	writeTestFiles(t, dir, map[string]string{
		"a/go.mod":            "module example.com/a\n\ngo 1.25\n\nignore ./internal/gen\n",
		"a/a.go":              "package a\n",
		"a/internal/gen/g.go": "package gen\n",
		"a/nested/go.mod":     "module example.com/a/nested\n",
		"a/nested/n.go":       "package nested\n",
		"b/go.mod":            "module example.com/b\n",
		"b/b.go":              "package b\n",
	})

	require.Equal(t, []string{"a/a.go", "b/b.go"}, visitedFiles(t, dir, filepath.Join(dir, "...")))
}

// The ignore directives of go.mod decide which directories a recursive rewrite
// may change. If the go.mod of the module a walk starts in, or of a module
// it enters from outside of modules, can't be parsed, the walk must stop
// like go build does, instead of changing the directories that the
// ignore directives were meant to protect.
func TestRewrite_MalformedGoModIsError(t *testing.T) {
	const badGoMod = "module \"unterminated\n"
	failOnVisit := func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
		t.Errorf("rewrite of %s despite malformed go.mod", filePath)
		return nil, nil
	}

	t.Run("module of path", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFiles(t, dir, map[string]string{
			"go.mod":   badGoMod,
			"sub/s.go": "package sub\n",
		})
		err := Rewrite(filepath.Join(dir, "sub", "..."), nil, nil, failOnVisit)
		require.ErrorContains(t, err, filepath.Join(dir, "go.mod")+":1")
	})

	t.Run("module entered from outside of modules", func(t *testing.T) {
		dir := t.TempDir()
		requireOutsideModules(t, dir)
		writeTestFiles(t, dir, map[string]string{
			"a/go.mod": badGoMod,
			"a/a.go":   "package a\n",
			"b/b.go":   "package b\n",
		})
		err := Rewrite(filepath.Join(dir, "..."), nil, nil, failOnVisit)
		require.ErrorContains(t, err, filepath.Join(dir, "a", "go.mod")+":1")
	})
}

// An invalid import is a mistake in the returned imports,
// not in the replacements, so the error must not blame the replacements.
func TestRewriteWithReplacements_InvalidImportError(t *testing.T) {
	const src = "package p\n\nfunc a() {}\n"
	filePath := filepath.Join(t.TempDir(), "p.go")
	writeTestFiles(t, filepath.Dir(filePath), map[string]string{"p.go": src})

	err := RewriteWithReplacements(filePath, nil, nil, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0], "func b() {}")
			var imports Imports
			imports.Add(`"fmt"`) // Quoted like an import line
			return repls, imports, nil
		},
	)
	require.EqualError(t, err, `malformed import path "\"fmt\"": invalid char '"'`)
	requireFileContent(t, src, filePath)
}

// A FileReplacementsFunc error must leave the whole package unwritten,
// the same as a RewriteFileFunc error, else the package may not compile.
func TestRewriteWithReplacements_NoFilesWrittenOnError(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go": "package p\n\nvar a = 1\n",
		"b.go": "package p\n\nvar b = 1\n",
	})

	err := RewriteWithReplacements(dir, nil, nil, false,
		func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
			if filepath.Base(filePath) == "b.go" {
				return nil, nil, errors.New("b.go failed")
			}
			var repls NodeReplacements
			repls.AddReplacement(astFile.Decls[0], "var a = 2")
			return repls, nil, nil
		},
	)
	require.EqualError(t, err, "b.go failed")
	requireFileContent(t, "package p\n\nvar a = 1\n", filepath.Join(dir, "a.go"))
}

// Collecting nodes of the whole package instead of the current file is a codemod bug.
// A node of another file with the same size as the current file passes any size check,
// so RewriteWithReplacements must compare the file name, else it deletes unrelated code.
func TestRewriteWithReplacements_NodeOfSameSizeFileError(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"color.go": "package p\n\nfunc (Color) String() string { return \"red\" }\n\ntype Color int\n",
		"shape.go": "package p\n\nfunc (Shape) String() string { return \"box\" }\n\ntype Shape int\n",
	})

	removeOtherFilesDecls := FileReplacementsFunc(func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
		var repls NodeReplacements
		for path, other := range pkg.Files {
			if path != filePath {
				repls.AddRemoval(other.Decls[0])
			}
		}
		return repls, nil, nil
	})
	_, err := RewriteDryRun(dir, nil, removeOtherFilesDecls.PackageFunc())
	require.ErrorContains(t, err, "not from the source")
}

// A broken package below path must fail a recursive rewrite instead of being
// skipped like a directory without Go files, else a codemod silently misses code.
// Without "..." a directory without Go files is an error too.
func TestRewrite_ErrorInSubDirectoryAborts(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go":            "package a\n",
		"empty/README.md": "No Go files\n",
		"sub/b.go":        "package b\n\nfunc {\n",
	})
	noop := func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
		return nil, nil
	}

	err := Rewrite(filepath.Join(dir, "..."), nil, nil, noop)
	require.ErrorContains(t, err, filepath.Join(dir, "sub", "b.go")+":3:6")

	err = Rewrite(filepath.Join(dir, "empty"), nil, nil, noop)
	require.ErrorIs(t, err, ErrPackageNotFound)
}

// An empty result is a change like any other, it must not be mistaken
// for nil (unchanged) and silently skipped.
func TestRewritePackages_EmptyResultIsWritten(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{"a.go": "package a\n"})
	filePath := filepath.Join(dir, "a.go")

	emptyFile := func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		return map[string][]byte{filePath: {}}, nil
	}
	changedFiles, err := RewriteDryRun(dir, nil, emptyFile)
	require.NoError(t, err)
	require.Equal(t, []string{filePath}, changedFiles)

	require.NoError(t, RewritePackages(dir, nil, nil, emptyFile))
	requireFileContent(t, "", filePath)
}

// RewritePackages for a single file can only write that file.
// A change the callback returns for another file of the package must be an error,
// else for example a rename across files is silently written half.
func TestRewritePackages_SingleFileRejectsChangesOfOtherFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"a.go": "package p\n\nfunc Old() {}\n",
		"b.go": "package p\n\nvar _ = Old\n",
	})
	aPath, bPath := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")

	err := RewritePackages(aPath, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		return map[string][]byte{
			aPath: []byte("package p\n\nfunc New() {}\n"),
			bPath: []byte("package p\n\nvar _ = New\n"),
		}, nil
	})
	require.ErrorContains(t, err, bPath)
	requireFileContent(t, "package p\n\nfunc Old() {}\n", aPath)
	requireFileContent(t, "package p\n\nvar _ = Old\n", bPath)

	// Unchanged sources for other files are fine
	err = RewritePackages(aPath, nil, nil, func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		return map[string][]byte{aPath: []byte("package p\n\nfunc Old() {}\n\n// changed\n"), bPath: pkg.Sources[bPath]}, nil
	})
	require.NoError(t, err)
	requireFileContent(t, "package p\n\nfunc Old() {}\n\n// changed\n", aPath)
}

func requireFileContent(t *testing.T, want, filePath string) {
	t.Helper()
	got, err := os.ReadFile(filePath) //#nosec G304 -- test file path
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}
