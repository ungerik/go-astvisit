# Tutorial: write your first codemod

You'll build `sincefix`, a small command line tool that finds every
`time.Now().Sub(t)` in a Go module and rewrites it to `time.Since(t)`. It
previews changes on stdout, writes them with `-w`, and fails a CI check with
`-check`. Along the way you'll use the three parts of go-astvisit that most
codemods need:

- a **visitor** to find the code you care about,
- **`NodeReplacements`** to describe edits without disturbing anything else
  in the file,
- **`RewriteWithReplacements`** and **`RewriteDryRun`** to apply those edits to
  every file of a module.

A *codemod* is a program that changes source code, typically to migrate many
call sites at once.

## What you'll need

- Go 1.26 or newer (the version go-astvisit's `go.mod` requires).
- Basic Go. You don't need to know `go/ast` yet: Go source parses into a tree
  of nodes like `*ast.CallExpr` (a function call) or `*ast.SelectorExpr`
  (`x.Name`), and that's all this tutorial assumes.

## Step 1: Create a project to fix

Make a directory for the tutorial and a small module that uses the pattern
you'll replace:

```bash
mkdir -p ~/sincefix-tutorial/demo
cd ~/sincefix-tutorial/demo
go mod init example.com/demo
```

Create `main.go`:

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	time.Sleep(20 * time.Millisecond)
	elapsed := time.Now().Sub(start)
	fmt.Println("took more than 10ms:", elapsed > 10*time.Millisecond)
	fmt.Println("older than a second:", olderThan(start, time.Second))
}

func olderThan(t time.Time, d time.Duration) bool {
	return time.Now().Sub(t) > d
}
```

Run it:

```bash
go run .
```

```
took more than 10ms: true
older than a second: false
```

`time.Now().Sub(start)` works, but `time.Since(start)` says the same thing
more clearly. There are two of them to fix.

## Step 2: Find the calls

Create the tool as a second module next to the demo:

```bash
mkdir -p ~/sincefix-tutorial/sincefix
cd ~/sincefix-tutorial/sincefix
go mod init example.com/sincefix
go get github.com/ungerik/go-astvisit@latest
```

Create `main.go`:

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

// sinceVisitor finds calls of the form time.Now().Sub(t)
type sinceVisitor struct {
	astvisit.VisitorImpl
	fset *token.FileSet
}

func (v *sinceVisitor) VisitCallExpr(call *ast.CallExpr, cursor astvisit.Cursor) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sub" || len(call.Args) != 1 {
		return true
	}
	if astvisit.ExprString(sel.X) != "time.Now()" {
		return true
	}
	fmt.Printf("%s: %s\n", v.fset.Position(call.Pos()), astvisit.ExprString(call))
	return true
}

func fixFile(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (astvisit.NodeReplacements, astvisit.Imports, error) {
	astvisit.Visit(astFile, &sinceVisitor{fset: fset}, nil)
	return nil, nil, nil
}

func main() {
	err := astvisit.RewriteWithReplacements(os.Args[1], nil, os.Stdout, false, fixFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Run it on the demo module. The trailing `...` means "this directory and all
below it", like `./...` for the go command:

```bash
go run . ../demo/...
```

```
/home/you/sincefix-tutorial/demo/main.go:11:13: time.Now().Sub(start)
/home/you/sincefix-tutorial/demo/main.go:17:9: time.Now().Sub(t)
```

Both calls found. What each piece does:

- **`astvisit.VisitorImpl`** implements a `Visit` method for every node type
  that just returns `true`. Embedding it means you only write
  `VisitCallExpr`, the one method you need.
- **`VisitCallExpr`** is called for every function call. `call.Fun` is what's
  being called. For `time.Now().Sub(start)` that's the selector
  `time.Now().Sub`, whose left side `sel.X` is the call `time.Now()`.
- **`astvisit.ExprString`** turns an expression back into text, so you can
  compare it with `"time.Now()"` instead of checking three levels of node
  types.
- **`astvisit.Visit`** walks the file's tree and calls your methods.
  Returning `true` means "keep going into this node's children".
- **`RewriteWithReplacements`** parses every package under the path, calls
  `fixFile` for each file, and applies the replacements it returns. You
  returned none yet, so nothing changes.

## Step 3: Replace the calls

Now collect a replacement for each match. Replace `main.go` with:

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

// sinceVisitor finds calls of the form time.Now().Sub(t)
// and collects a replacement with time.Since(t) for each of them
type sinceVisitor struct {
	astvisit.VisitorImpl
	fset         *token.FileSet
	replacements astvisit.NodeReplacements
}

func (v *sinceVisitor) VisitCallExpr(call *ast.CallExpr, cursor astvisit.Cursor) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sub" || len(call.Args) != 1 {
		return true
	}
	if astvisit.ExprString(sel.X) != "time.Now()" {
		return true
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", v.fset.Position(call.Pos()), astvisit.ExprString(call))
	since := &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Since")},
		Args: call.Args,
	}
	v.replacements.AddReplacement(call, since)
	// Don't visit the children, a replacement within call would overlap
	return false
}

func fixFile(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (astvisit.NodeReplacements, astvisit.Imports, error) {
	visitor := &sinceVisitor{fset: fset}
	astvisit.Visit(astFile, visitor, nil)
	return visitor.replacements, nil, nil
}

func main() {
	err := astvisit.RewriteWithReplacements(os.Args[1], nil, os.Stdout, false, fixFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Run it again:

```bash
go run . ../demo/...
```

```
/home/you/sincefix-tutorial/demo/main.go:11:13: time.Now().Sub(start)
/home/you/sincefix-tutorial/demo/main.go:17:9: time.Now().Sub(t)
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	time.Sleep(20 * time.Millisecond)
	elapsed := time.Since(start)
	fmt.Println("took more than 10ms:", elapsed > 10*time.Millisecond)
	fmt.Println("older than a second:", olderThan(start, time.Second))
}

func olderThan(t time.Time, d time.Duration) bool {
	return time.Since(t) > d
}
```

The match lines now go to stderr, and the rewritten file is printed to stdout.
`demo/main.go` itself is unchanged, because `resultOut` (the third argument)
is `os.Stdout`. Check with `grep Sub ../demo/main.go`.

What changed:

- **`AddReplacement(call, since)`** records "replace the bytes of `call` with
  `since`". The replacement is an `ast.Node` here, which gets printed as Go
  code. A plain `string` works too.
- The new call reuses **`call.Args`**, so the argument is kept as written,
  whatever expression it is.
- **`return false`** stops the visitor from going into the call's children.
  Two replacements may not cover the same bytes, so a second match nested
  inside the first would be an error. If calls are nested, running the tool
  twice catches the inner ones.

Only the two calls changed. Everything else in the file, comments and blank
lines included, is copied byte for byte. Why that matters is explained in
[Why NodeReplacements edits source text](explanation-text-replacements.md).

## Step 4: Write the changes

Add a `-w` flag like `gofmt -w`. Add `"flag"` to the imports and replace
`main` with:

```go
func main() {
	write := flag.Bool("w", false, "write the result to the source files instead of stdout")
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = "./..."
	}

	var err error
	if *write {
		err = astvisit.RewriteWithReplacements(path, os.Stderr, nil, false, fixFile)
	} else {
		err = astvisit.RewriteWithReplacements(path, nil, os.Stdout, false, fixFile)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
```

With `-w`, `resultOut` is `nil`, which tells go-astvisit to write the files.
`verboseOut` (the second argument) is `os.Stderr`, so you see what it does:

```bash
go run . -w ../demo/...
```

```
parsing file: /home/you/sincefix-tutorial/demo/main.go
/home/you/sincefix-tutorial/demo/main.go:11:13: time.Now().Sub(start)
/home/you/sincefix-tutorial/demo/main.go:17:9: time.Now().Sub(t)
changed file: /home/you/sincefix-tutorial/demo/main.go
```

Check the demo still works:

```bash
cd ../demo && grep -n Since main.go && go run . && cd ../sincefix
```

```
11:	elapsed := time.Since(start)
17:	return time.Since(t) > d
took more than 10ms: true
older than a second: false
```

## Step 5: Run it again

A codemod should do nothing on code it already fixed:

```bash
go run . -w ../demo/...
```

```
parsing file: /home/you/sincefix-tutorial/demo/main.go
no changes in file: /home/you/sincefix-tutorial/demo/main.go
```

No matches, no replacements, no write. Files are only written when their
content changes, so running the tool on a whole repository leaves untouched
files alone, even ones that aren't gofmt-clean.

## Step 6: Guard against regressions with `-check`

Old habits come back. Add a `-check` flag that lists files still needing the
fix and exits with status 1, for CI. Here is the complete `main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"

	"github.com/ungerik/go-astvisit"
)

// sinceVisitor finds calls of the form time.Now().Sub(t)
// and collects a replacement with time.Since(t) for each of them
type sinceVisitor struct {
	astvisit.VisitorImpl
	fset         *token.FileSet
	replacements astvisit.NodeReplacements
}

func (v *sinceVisitor) VisitCallExpr(call *ast.CallExpr, cursor astvisit.Cursor) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sub" || len(call.Args) != 1 {
		return true
	}
	if astvisit.ExprString(sel.X) != "time.Now()" {
		return true
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", v.fset.Position(call.Pos()), astvisit.ExprString(call))
	since := &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Since")},
		Args: call.Args,
	}
	v.replacements.AddReplacement(call, since)
	// Don't visit the children, a replacement within call would overlap
	return false
}

func fixFile(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (astvisit.NodeReplacements, astvisit.Imports, error) {
	visitor := &sinceVisitor{fset: fset}
	astvisit.Visit(astFile, visitor, nil)
	return visitor.replacements, nil, nil
}

func main() {
	write := flag.Bool("w", false, "write the result to the source files instead of stdout")
	check := flag.Bool("check", false, "list the files that need a fix and exit with status 1")
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		path = "./..."
	}

	switch {
	case *check:
		changedFiles, err := astvisit.RewriteDryRun(path, nil, astvisit.FileReplacementsFunc(fixFile).PackageFunc())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, filePath := range changedFiles {
			fmt.Println(filePath)
		}
		if len(changedFiles) > 0 {
			os.Exit(1)
		}
	case *write:
		err := astvisit.RewriteWithReplacements(path, os.Stderr, nil, false, fixFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	default:
		err := astvisit.RewriteWithReplacements(path, nil, os.Stdout, false, fixFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
```

`RewriteDryRun` runs the same parsing and replacement logic but writes
nothing. It returns the paths that `-w` would change. It takes a
package-level callback, and `FileReplacementsFunc(fixFile).PackageFunc()`
converts your per-file function into one.

Reintroduce the old pattern at the end of `demo/main.go`:

```go
func age(t time.Time) time.Duration {
	return time.Now().Sub(t)
}
```

Then check:

```bash
go run . -check ../demo/...
```

```
/home/you/sincefix-tutorial/demo/main.go:21:9: time.Now().Sub(t)
/home/you/sincefix-tutorial/demo/main.go
exit status 1
```

The first line is the match on stderr, the second the file that needs a fix,
and `exit status 1` is `go run` reporting the exit code. A CI step running
this command fails. Run `go run . -w ../demo/...` and `-check` passes again
with exit status 0.

## What you built

`sincefix` is a complete codemod:

- a **visitor** (`sinceVisitor`) that matches calls by their shape and text,
- **replacements** that change only the matched bytes and leave comments and
  layout alone,
- a **preview** mode on stdout, a **`-w`** mode that writes only changed files,
  and a **`-check`** mode for CI.

The same structure works for any syntactic migration: swap out
`VisitCallExpr` and the replacement.

One limit to know: the match is purely syntactic. A file that imports `time`
under another name, or has a local variable called `time`, isn't handled
correctly. Matching by meaning needs type information from `go/types`, which
go-astvisit doesn't load.

Where to go next:

- [How to rewrite Go source files](howto-rewrite-source-files.md): adding
  imports, debug mode, and whole-package rewrites with `RewritePackages`.
- [How to check in CI that no file needs rewriting](howto-check-generated-code-in-ci.md):
  the `-check` idea as a Go test.
- [How to modify an AST with Cursor](howto-modify-ast-with-cursor.md):
  replace, delete and insert nodes in memory.
- [How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md):
  why `testdata` and test files were never touched.
- API reference on [pkg.go.dev](https://pkg.go.dev/github.com/ungerik/go-astvisit).
