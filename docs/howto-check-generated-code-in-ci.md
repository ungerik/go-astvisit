# How to check in CI that no file needs rewriting

Make CI fail when someone commits code that your codemod would still change,
using `astvisit.RewriteDryRun`. Typical cases: a migration that new code must
follow, like using `time.Since(t)` instead of `time.Now().Sub(t)`, or files a
rewrite tool keeps in sync.

`RewriteDryRun` runs exactly what `RewritePackages` would run, but writes
nothing and returns the paths of the files that would change.

## Prerequisites

- A codemod as a `FileReplacementsFunc`, `RewriteFileFunc` or
  `RewritePackageFunc`. The examples use the codemod of the
  [tutorial](tutorial-write-a-codemod.md), moved into an importable package as
  `FixFile`, with the `fset` field and the stderr output removed from
  `sinceVisitor`.
- The codemod lives in the same module as the code it checks (this matters
  for test caching, see [Troubleshooting](#troubleshooting)).

The layout used below:

```
myproject/
├── go.mod                       module example.com/myproject
├── clock/clock.go               code that gets checked
├── cmd/sincefix/main.go         the command line tool
└── internal/sincefix/
    ├── sincefix.go              func FixFile(...) (NodeReplacements, Imports, error)
    └── sincefix_test.go         the check
```

## Steps

### Option A: a Go test

The check runs wherever `go test ./...` runs, without any CI configuration.

1. **Export the codemod** from a package, for example
   `internal/sincefix/sincefix.go`:

   ```go
   // FixFile is an astvisit.FileReplacementsFunc
   func FixFile(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (astvisit.NodeReplacements, astvisit.Imports, error) {
   	visitor := &sinceVisitor{}
   	astvisit.Visit(astFile, visitor, nil)
   	return visitor.replacements, nil, nil
   }
   ```

2. **Add a test** next to it that dry-runs the codemod on the whole module.
   `go test` runs in the package directory, so the path climbs to the module
   root:

   ```go
   package sincefix

   import (
   	"strings"
   	"testing"

   	"github.com/ungerik/go-astvisit"
   )

   // Fails if a file of the module still contains time.Now().Sub(t),
   // so that CI catches code that was committed without running the fix.
   func TestNoFileNeedsFix(t *testing.T) {
   	// go test runs in the package directory, ../../... is the whole module
   	changedFiles, err := astvisit.RewriteDryRun("../../...", nil, astvisit.FileReplacementsFunc(FixFile).PackageFunc())
   	if err != nil {
   		t.Fatal(err)
   	}
   	if len(changedFiles) > 0 {
   		t.Errorf("run `go run ./cmd/sincefix -w ./...`, these files need a fix:\n%s", strings.Join(changedFiles, "\n"))
   	}
   }
   ```

   Put the command that fixes the problem in the failure message. Whoever
   breaks the check sees how to fix it without reading the test.

3. **Run it.**

   ```bash
   go test ./...
   ```

   With a `time.Now().Sub(t)` left in `clock/clock.go`:

   ```
   --- FAIL: TestNoFileNeedsFix (0.00s)
       sincefix_test.go:19: run `go run ./cmd/sincefix -w ./...`, these files need a fix:
           /home/you/myproject/clock/clock.go
   FAIL
   FAIL	example.com/myproject/internal/sincefix	0.260s
   ```

### Option B: a `-check` flag in the CI pipeline

Use this when the codemod is a standalone tool, or when you want the check as
its own CI step.

1. **Add a `-check` flag** to the tool that calls `RewriteDryRun`, prints the
   paths, and exits with status 1 if there are any:

   ```go
   case *check:
   	changedFiles, err := astvisit.RewriteDryRun(path, nil, astvisit.FileReplacementsFunc(sincefix.FixFile).PackageFunc())
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
   ```

   The complete `main` is in step 6 of the [tutorial](tutorial-write-a-codemod.md#step-6-guard-against-regressions-with--check).
   Exit status 2 for errors keeps "the check found something" apart from "the
   check couldn't run".

2. **Run it in CI**, for example as a GitHub Actions step after checkout and
   `actions/setup-go`:

   ```yaml
   - name: Check that no file needs sincefix
     run: go run ./cmd/sincefix -check ./...
   ```

   Output when a file needs a fix:

   ```
   /home/you/myproject/clock/clock.go
   exit status 1
   ```

   (`exit status 1` is printed by `go run`, the step fails because of the
   exit code.)

## Verification

1. Add a line the codemod would change, for example
   `return time.Now().Sub(t)` in `clock/clock.go`.
2. Run `go test ./...` (option A) or `go run ./cmd/sincefix -check ./...`
   (option B). It must fail and name `clock/clock.go`.
3. Run `go run ./cmd/sincefix -w ./...` and run the check again. It must pass.

If step 2 passes, the check isn't looking at the file you changed. See below.

## Troubleshooting

**The test still passes (`ok … (cached)`) after I added a violation.** The go
command caches test results and re-runs a test when a file it read changed, but
it only watches files inside the module of the test. If the dry run checks
files outside that module, the cached pass hides new violations. Keep the test
in the module it checks, or run `go test -count=1` to skip the cache.

**The check passes but a file clearly needs a fix.**

- `_test.go` files are never rewritten, so they are never reported either.
- Files behind custom build tags (like `//go:build integration`),
  directories named `testdata`, `vendor`, `node_modules`, or starting with
  `.` or `_`, nested modules, and directories in an `ignore` directive of
  `go.mod` are skipped. See
  [How the Rewrite functions choose files](explanation-file-selection-writes.md).
- The path is relative to the working directory, which is the package
  directory under `go test`. Print `filepath.Abs(path)` if unsure.

**`package not found in /abs/dir`.** The path has no `...` and points at a
directory without Go files. Add `...` or point it at a package directory.

**The check fails in CI but `-w` changes nothing locally.** Check that CI runs
the same commit, with the same codemod and go-astvisit version from `go.mod`.
Line endings are not the cause: results are converted to the original file's
line endings before comparing, so a CRLF checkout alone doesn't make a
difference.

## Related

- [`RewriteDryRun` reference](https://pkg.go.dev/github.com/ungerik/go-astvisit#RewriteDryRun),
  including a runnable example with the tutorial's codemod
- [How to rewrite Go source files](howto-rewrite-source-files.md)
- [Tutorial: write your first codemod](tutorial-write-a-codemod.md)
- [How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md)
