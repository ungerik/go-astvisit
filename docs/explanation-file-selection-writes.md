# How the Rewrite functions choose files and when they write

`Rewrite`, `RewriteWithReplacements`, `RewritePackages` and `RewriteDryRun` all
share one directory walker in `rewrite.go`. It decides which files
your callback sees and when the results reach the disk. The goal is that
running a codemod over `./...` reaches the directories the go command would
match, includes the files of every platform, and never leaves a package half
rewritten because your callback failed. Where it deliberately differs from the
go command is listed under [Trade-offs](#trade-offs).

For the task-level guide see
[How to rewrite Go source files](howto-rewrite-source-files.md).

## The problem

A naive `filepath.Walk` over every `.go` file breaks on real repositories:

- **`testdata` fixtures** are often invalid or deliberately unformatted Go.
  Parsing them aborts the run, and rewriting them destroys the fixtures.
- **`vendor`** holds third-party code that must not be edited.
- **Nested modules** (subdirectories with their own `go.mod`) are versioned
  and released separately, and a module can exclude directories with an
  `ignore` directive in its `go.mod`. `go build ./...` sees neither.
- **Generator files** like `gen.go` with `//go:build ignore` and
  `package main` sit next to library code. Parsing them with the library
  files gives two package names in one directory.
- **Platform files** like `file_windows.go` must be rewritten on a Mac too,
  or the codemod only covers half the code.
- **A callback error on file 3 of 5** must not leave files 1 and 2 rewritten,
  or the package may no longer compile.
- **CRLF checkouts** (Windows with `core.autocrlf`) turn every rewrite into a
  whole-file diff if the tool writes LF.

## The approach

```
Rewrite*(path)
│
├─ path is a file
│    ├─ *_test.go?            → error "test file … is not rewritten"
│    ├─ excluded from build?  → parse the file as a package of its own
│    └─ otherwise             → parse its whole package,
│                               but FilePaths = [this file]
│
└─ path is a directory
     ├─ path ends in "..."? find the module: nearest go.mod at or above path
     ├─ parse the package in the directory
     │    ├─ no Go files: "..." → note in verboseOut, continue
     │    │               else  → error ErrPackageNotFound
     │    └─ found → callback(pkg) → write changed files
     └─ path ends in "..."? for each subdirectory, sorted by name:
          skip ".*", "_*", testdata, vendor, node_modules
          has go.mod?   inside a module  → skip (nested module)
                        outside modules  → enter, it becomes the module
          no go.mod?    matches an ignore directive of the module → skip
          recurse
```

### Which files a package contains

Like `ParsePackage` (`parse.go`), the walker reads each directory and keeps a `.go` file only if:

| Rule                                               | Effect                                             |
|----------------------------------------------------|----------------------------------------------------|
| The name doesn't end in `_test.go`                 | Test files are never rewritten                     |
| `go/build` matches it for at least one known GOOS/GOARCH, with or without cgo, without custom build tags | Files of every platform are kept, files excluded everywhere are dropped |

In practice the second rule means:

- `file_windows.go` and `//go:build linux` files **are** included on any host.
- `//go:build ignore` files are skipped, so generators don't clash with the
  library package.
- Files behind custom tags like `//go:build integration` are **skipped** in a
  directory walk, and so are files that need toolchain tags (`race`, `msan`,
  `gccgo`) or an older Go version (`!go1.21`). Pass such a file explicitly to
  rewrite it.
- Names starting with `_` or `.` are skipped (a `go/build` rule).

All kept files must declare the same package name, otherwise the walk stops
with `2 packages found in <dir>: a, b`. Files are processed in name order.

### Which directories are walked

With a trailing `...`, the walker descends into every subdirectory except:

- names starting with `.` or `_`, `testdata` and `vendor`,
- nested modules: subdirectories with their own `go.mod` file,
- directories matching an `ignore` directive in the module's `go.mod`.
  `ignore ./gen` skips `gen` at the module root, `ignore fixtures` skips
  every directory named `fixtures` at any depth,
- `node_modules`, which is common in repositories that mix Go and JavaScript.

All but the last are the go command's rules for `./...` (see
`go help packages`). On a test tree with all of these cases, the walker visits
exactly the packages that `go list ./...` lists, except the one in
`node_modules`.

The module is the nearest `go.mod` at or above the starting directory, so its
`ignore` directives also apply when you start in a subdirectory like
`./internal/...`. The starting directory itself is always processed, even if
it is a nested module or matches an `ignore` directive: naming it is how you
rewrite it on purpose. An `ignore` directive that matches the starting
directory doesn't apply to its subdirectories either, so with `ignore ./gen`
a rewrite of `./gen/...` covers all of `gen`. Directives that don't match the
starting directory still apply below it.

If the starting directory is not inside any module, for example the root of a
repository that holds several modules without a `go.mod` of its own, the
walker enters each module it finds and then applies that module's rules
below it.

Symbolic links to directories are not followed, because `os.DirEntry.IsDir`
is false for a symlink. Symbolic links to `.go` files are, like the go command
builds them: such a file is read and written through the link, which can change
a file outside of the walked tree.

### When files are written

```
callback(pkg) ── error ──► return error, nothing of this package is written
     │
     ok: map[filePath][]byte
     │
     ├─ a key that isn't in pkg.Files ──► error before any write
     ├─ a changed file not in FilePaths ─► error before any write
     │
     └─ for each path in pkg.FilePaths:
          nil source                    → unchanged (an empty one is a change)
          convert to the line endings of the original's first line
          equal to the original bytes?  → unchanged
          otherwise                     → write (or send to resultOut,
                                                  or record for RewriteDryRun)
```

Files are written with `os.WriteFile(path, source, 0644)`. Because the files
already exist, `os.WriteFile` truncates them and keeps their permissions; the
`0644` would only apply to a new file, and the walker never creates one.

When `resultOut` is set, each changed file's full source is written to it
instead, one after another with no separator. Pass a `verboseOut` writer to get
a `changed file: <path>` line before each one.

`RewriteDryRun` runs the same code but only collects the paths that would be
written, so its result is exactly the set of files `RewritePackages` would
change.

## Trade-offs

- **All-or-nothing per package, not per run.** A callback error in the third
  package leaves the first two packages already written. Run on a clean git
  working tree, or call `RewriteDryRun` first, if that matters.
- **All-or-nothing against callback errors, not I/O errors.** Files of a
  package are written one after another without temp files. A write error
  half way leaves earlier files of that package written.
- **One module per walk inside a module.** A rewrite of `./...` that starts
  inside a module stays inside it and skips nested modules. To rewrite a nested
  module, run it on that module's directory too.
- **The module's `go.mod` must parse.** The walker reads the `ignore`
  directives of the module it starts in, and of each module it enters from
  outside any module. A `go.mod` with a syntax error there stops the walk with
  the parse error, like it stops `go build`. Directives newer than the
  `golang.org/x/mod` version of this library are ignored. A nested module is detected by its `go.mod`
  file existing, and that file is never parsed.
- **Custom, toolchain and release tags are invisible in a walk.** Files
  behind tags like `integration` or `wireinject` are skipped silently. So are
  files that only build with toolchain tags (`//go:build race`, `msan`,
  `gccgo`) and fallbacks for older Go versions (`//go:build !go1.21`), because
  only GOOS, GOARCH and cgo are varied. A rename codemod can leave such files
  stale. List them explicitly if your codemod must reach them.
- **A single file still needs its whole package.** Passing `pkg/a.go` parses
  all files of `pkg`, so a syntax error in `pkg/b.go` fails the run. The
  callback gets the whole package but only `a.go` is in `FilePaths` and can be
  written.
- **Line endings follow the first line.** A file with mixed line endings is
  written with the ending of its first line everywhere.
- **No new files.** A callback can only return paths that are already in
  `pkg.Files`. Generating a new file is outside what the walker does.

## Related

- [How to rewrite Go source files](howto-rewrite-source-files.md)
- [How to check in CI that no file needs rewriting](howto-check-generated-code-in-ci.md)
- [Why NodeReplacements edits source text instead of reprinting the AST](explanation-text-replacements.md)
