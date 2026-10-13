# How to rewrite Go source files

Apply a change to every Go file of a package or module, preview it on stdout,
then write it back to the files.

## Prerequisites

- `go get github.com/ungerik/go-astvisit`
- A clean git working tree. Writes are all-or-nothing per package, not per
  run, so `git diff` and `git checkout .` are your undo
  ([details](explanation-file-selection-writes.md#trade-offs)).

## Pick the function

| You want to                                      | Use                       | Your callback returns                     |
|--------------------------------------------------|---------------------------|-------------------------------------------|
| Edit byte ranges and keep comments and layout    | `RewriteWithReplacements` | `NodeReplacements` and `Imports` per file |
| Modify the AST and print it yourself             | `Rewrite`                 | The new source per file, or `nil`         |
| Analyze a whole package, then edit several files | `RewritePackages`         | `map[filePath][]byte` per package         |
| List the files that would change, write nothing  | `RewriteDryRun`           | Same as `RewritePackages`                 |

All of them take:

- `path`: a file, a directory, or a directory followed by `...` for all
  subdirectories, like `"./..."`.
- `verboseOut`: progress messages, or `nil` for none.
- `resultOut` (not for `RewriteDryRun`): `nil` writes the files, a writer
  receives the changed sources instead and no file is touched.

Which files and directories are included is explained in
[How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md).
In short: no `_test.go` files, no `testdata`, `vendor`, `node_modules`, or
directories starting with `.` or `_`, no nested modules, and no directories
excluded by an `ignore` directive in `go.mod`.

## Steps

1. **Write a function that returns replacements.** This one replaces the
   layout string `"2006-01-02"` with the `time.DateOnly` constant and makes
   sure the file imports `time`:

   ```go
   package main

   import (
   	"fmt"
   	"go/ast"
   	"go/token"
   	"io"
   	"os"

   	"github.com/ungerik/go-astvisit"
   )

   func replaceDateLayout(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (astvisit.NodeReplacements, astvisit.Imports, error) {
   	var (
   		replacements astvisit.NodeReplacements
   		imports      = make(astvisit.Imports)
   	)
   	ast.Inspect(astFile, func(node ast.Node) bool {
   		lit, ok := node.(*ast.BasicLit)
   		if ok && lit.Kind == token.STRING && lit.Value == `"2006-01-02"` {
   			replacements.AddReplacement(lit, "time.DateOnly", "date-layout")
   			imports.Add(`"time"`)
   		}
   		return true
   	})
   	return replacements, imports, nil
   }

   func main() {
   	debug := len(os.Args) > 2 && os.Args[2] == "-debug"
   	err := astvisit.RewriteWithReplacements(os.Args[1], nil, os.Stdout, debug, replaceDateLayout)
   	if err != nil {
   		fmt.Fprintln(os.Stderr, err)
   		os.Exit(1)
   	}
   }
   ```

   The third argument of `AddReplacement` is an optional debug ID that shows
   up in debug output (step 4). A replacement can be a `string`, a `[]byte`,
   an `ast.Node`, or `nil` to remove the node. `AddRemoval` and
   `AddInsertAfter` are shortcuts for removing a node and inserting after it.

2. **Preview the result on stdout.** With `resultOut` set to `os.Stdout`, no
   file is changed. Only changed files are printed, each as its complete new
   source:

   ```bash
   go run . ./proj/...
   ```

   ```
   package proj

   import (
   	"fmt"
   	"time"
   )

   func FormatDay(year, month, day int) string {
   	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
   }

   const dayLayout = time.DateOnly
   ```

   The `"time"` import was added because the file didn't have it. Imports
   that are already present are left alone. When an import is added, the
   import block is grouped goimports-style; otherwise the output is plain
   gofmt output.

3. **Label the output** when more than one file changes. Changed sources are
   written to `resultOut` one after another with no separator. Pass a
   `verboseOut` writer to get a line before each one:

   ```go
   err := astvisit.RewriteWithReplacements(path, os.Stderr, os.Stdout, false, replaceDateLayout)
   ```

   `verboseOut` gets `parsing file: <path>` when the callback runs for a file,
   then `changed file: <path>` or `no changes in file: <path>`.

4. **Check edits in debug mode** if a result looks wrong. With `debug` set to
   `true`, every replacement is printed with conflict-style markers instead of
   being applied:

   ```bash
   go run . ./proj/... -debug
   ```

   ```
   const dayLayout = 
   <<<<<<< /path/to/proj/dates.go
   "2006-01-02"
   =======
   time.DateOnly
   >>>>>>> date-layout
   ```

   Between `<<<<<<<` and `=======` is the original source of the node, between
   `=======` and `>>>>>>>` its replacement. The file name follows `<<<<<<<`,
   the debug ID follows `>>>>>>>`.

   Debug mode never writes files and returns an error if `resultOut` is `nil`.

5. **Write the files** by passing `nil` as `resultOut`:

   ```go
   err := astvisit.RewriteWithReplacements(path, os.Stderr, nil, false, replaceDateLayout)
   ```

   Only files whose content changed are written. A file whose edits only
   change formatting is left alone. Files with CRLF line endings keep them.

### Modify the AST with Rewrite instead

`Rewrite` hands you each file's AST and expects the complete new source back.
Return `nil` when you changed nothing. `Rewrite` writes every file whose bytes
differ from the original, so returning the reprinted source of an untouched
file would reformat it.

```go
// sprintfVisitor replaces fmt.Sprintf("%s", x) with fmt.Sprint(x)
type sprintfVisitor struct {
	astvisit.VisitorImpl
	changed bool
}

func (v *sprintfVisitor) VisitCallExpr(call *ast.CallExpr, cursor astvisit.Cursor) bool {
	if astvisit.ExprString(call.Fun) != "fmt.Sprintf" || len(call.Args) != 2 {
		return true
	}
	if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Value == `"%s"` {
		call.Fun.(*ast.SelectorExpr).Sel.Name = "Sprint"
		call.Args = call.Args[1:]
		v.changed = true
	}
	return true
}

func rewriteFile(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
	visitor := &sprintfVisitor{}
	astvisit.Visit(astFile, visitor, nil)
	if !visitor.changed {
		return nil, nil // Unchanged, don't reformat the file
	}
	var buf bytes.Buffer
	err := format.Node(&buf, fset, astFile)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ...
err := astvisit.Rewrite(path, os.Stderr, nil, rewriteFile)
```

Reprinting the AST can move comments and add blank lines, see
[How to modify an AST with Cursor](howto-modify-ast-with-cursor.md#troubleshooting).

### Work on a whole package with RewritePackages

When a change needs to look at all files of a package before editing any of
them, `RewritePackages` calls your function once per package with a
`*astvisit.ParsedPackage`:

- `pkg.Files` and `pkg.Sources` hold the AST and the parsed bytes of every
  file, keyed by absolute path.
- `pkg.FilePaths` lists the files you may change (all of them, unless the
  `path` argument was a single file).
- `pkg.Fset` is the `token.FileSet` of all files.

You apply replacements and format the result yourself. This renames an
unexported function in every file of the package that declares it:

```go
const oldName, newName = "reportTitle", "dailyReportTitle"

// renameFunc renames the package level function oldName to newName
// in all files of a package, but only in packages that declare it.
func renameFunc(pkg *astvisit.ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
	declared := false
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok && f.Recv == nil && f.Name.Name == oldName {
				declared = true
			}
		}
	}
	if !declared {
		return nil, nil
	}
	rewritten := make(map[string][]byte)
	for _, filePath := range pkg.FilePaths {
		var replacements astvisit.NodeReplacements
		ast.Inspect(pkg.Files[filePath], func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && ident.Name == oldName {
				replacements.AddReplacement(ident, newName)
			}
			return true
		})
		if len(replacements) == 0 {
			continue
		}
		source, err := replacements.Apply(pkg.Fset, pkg.Sources[filePath])
		if err != nil {
			return nil, err
		}
		rewritten[filePath], err = astvisit.FormatFileWithImports(pkg.Fset, source, nil)
		if err != nil {
			return nil, err
		}
	}
	return rewritten, nil
}

// ...
err := astvisit.RewritePackages(path, os.Stderr, nil, renameFunc)
```

Matching by name is a heuristic: a local variable called `reportTitle` would be
renamed too. Use `gopls rename` when you need a rename that understands scopes.

Files missing from the returned map, or mapped to `nil`, are not changed. If
the callback returns an error, none of that package's files are written.

To group local imports separately, call `FormatFileWithImports` with your
module prefix, for example
`astvisit.FormatFileWithImports(pkg.Fset, source, imports, "example.com/myproject")`.
`RewriteWithReplacements` doesn't pass local prefixes, so this needs
`RewritePackages`.

### Reuse a per-file function for RewritePackages or RewriteDryRun

Both callback types convert to a package callback:

```go
astvisit.FileReplacementsFunc(replaceDateLayout).PackageFunc()
astvisit.RewriteFileFunc(rewriteFile).PackageFunc()
```

### Rewrite a single file

Pass the file path. The whole package is parsed, but only that file is written.
A file that the directory walk would skip because of a build constraint, like
`//go:build ignore`, is rewritten when you name it, and parsed as a package of
its own. Test files return an error.

## Verification

- Before writing, preview with `resultOut` set to `os.Stdout`, or list the
  files that would change with `RewriteDryRun`.
- After writing, review `git diff` and run `go build ./... && go vet ./...`.
- Run the tool a second time. With `verboseOut` set, every file should report
  `no changes in file:`. A second run that still changes files means your
  replacement matches its own output.

## Troubleshooting

Error messages below are the exact strings returned, with your paths in place
of `/abs/...`.

**`debug needs a non-nil resultOut`.** Debug output isn't Go, so it can't
replace files. Pass a writer like `os.Stdout`.

**`test file /abs/x_test.go is not rewritten`.** Test files are never
rewritten, not even when named explicitly.

**`package not found in /abs/dir`.** The directory has no Go files that pass
the filters. Without `...` this is an error you can check with
`errors.Is(err, astvisit.ErrPackageNotFound)`. With `...`, such directories are
skipped and noted in `verboseOut`.

**`2 packages found in /abs/dir: a, b`.** Two package names in one directory,
not counting test files and files excluded by build constraints. Fix the
package clauses, or mark one file `//go:build ignore`.

**`package proj has no file report.go`.** A `RewritePackages` callback
returned a key that isn't in `pkg.Files`. Use the absolute paths from
`pkg.FilePaths` as keys. The callback can't create new files.

**`package proj: file /abs/use.go is changed, but only /abs/report.go can be rewritten`.**
`RewritePackages` was called for a single file, and the callback changed another
file of the package, which would get lost. Rewrite the whole directory, or
return changes only for the files in `pkg.FilePaths`.

**`replacement node at /abs/a.go:3:1 overlaps the replacement node at /abs/a.go:3:1`.**
Two replacements cover the same bytes, often a node and one of its children.
Return `false` from the visitor after a match, or merge the two edits into one.

**`replacement node *ast.GenDecl at /abs/b.go:3:1 is not from the source`.**
The node wasn't parsed from this file's source, for example a node of another
file of the package, a node built without a position, or a node from
`ParseStatements`. Only nodes of the file being rewritten can be replaced.

**`invalid Go code after applying replacements: /abs/p.go:3:6: expected 'IDENT', found '{' (and 3 more errors)`.**
Your replacements produced invalid Go, and nothing was written. The line and
column refer to the source after the replacements were applied, not to the file
on disk. Run in debug mode to see which replacement is broken.

**The whole file got reformatted.** After a real change,
`RewriteWithReplacements` formats the result with gofmt rules. A file that wasn't gofmt-clean before is fully
reformatted in that case. Files without real changes are never reformatted.

**The edited file doesn't compile because of an unused import.** Imports are
only added, never removed. Remove the import yourself, for example with
`Rewrite`, `astutil.DeleteImport` and `format.Node`.

**gopls reports `ast.Package is deprecated`.** The `RewriteFileFunc` and
`FileReplacementsFunc` signatures contain `*ast.Package`, which `go/ast` marks
as deprecated. Ignore the warning, or use `RewritePackages`, whose callback
gets an `*astvisit.ParsedPackage` instead.

**A file I expected wasn't visited.** Check the selection rules in
[How the Rewrite functions choose files](explanation-file-selection-writes.md).
Files behind custom build tags like `//go:build integration`, toolchain tags
like `//go:build race` or older-release tags like `//go:build !go1.21` are
skipped in a directory walk, and so are nested modules and directories listed
in an `ignore` directive of `go.mod`. Pass such a directory or file as the path
to rewrite it.

## Related

- Reference: [`RewriteWithReplacements`](https://pkg.go.dev/github.com/ungerik/go-astvisit#RewriteWithReplacements),
  [`Rewrite`](https://pkg.go.dev/github.com/ungerik/go-astvisit#Rewrite),
  [`RewritePackages`](https://pkg.go.dev/github.com/ungerik/go-astvisit#RewritePackages),
  [`NodeReplacements`](https://pkg.go.dev/github.com/ungerik/go-astvisit#NodeReplacements),
  [`FormatFileWithImports`](https://pkg.go.dev/github.com/ungerik/go-astvisit#FormatFileWithImports)
- [How to check in CI that no file needs rewriting](howto-check-generated-code-in-ci.md)
- [Why NodeReplacements edits source text instead of reprinting the AST](explanation-text-replacements.md)
- [How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md)
- [Tutorial: write your first codemod](tutorial-write-a-codemod.md)
