# API refactoring: open questions

Result of a critical review of the exported API.
Compatibility with current callers is not a constraint, the goal is the best possible API.
Each section states the problem, a proposal, and the questions to decide before implementing.

Items marked "(confirmed)" were verified with a throwaway test.

Status checked on 2026-10-13 against commit `2eda386`:
solved 5, 9 (first question) and 10. Everything else is still open,
the exported API is unchanged. Line numbers refer to that commit.

Later on 2026-10-13: solved 3 and 7 (uncommitted at the time of writing).

Keep as is (not up for refactoring):

- Text-level `NodeReplacements` (preserves comments and formatting), `PosNode`, `NodeRange`
- go-command-compatible file selection (go.mod `ignore`, nested modules, build constraints),
  atomic per-package writes, CRLF preservation
- `Path`/`PathItem` tracking and root replacement in `Visit`
- `ErrPackageNotFound` sentinel

Suggested order: the small fixes (5, 9, 10) and 3 and 7 are done. Next 1 + 2 together
(they share the new `Package` type), and 4 last.

## 1. Rewrite API: too many entry points with positional parameters

Problem:

- 4 entry points (`Rewrite`, `RewriteWithReplacements`, `RewritePackages`, `RewriteDryRun`),
  3 callback types, 2 `.PackageFunc()` adapters.
  Capabilities are spread unevenly: dry-run only works with package funcs,
  debug only exists in `RewriteWithReplacements`.
- `RewriteWithReplacements(path, verboseOut, resultOut io.Writer, debug bool, f)` (`rewrite.go:83`):
  two adjacent `io.Writer`s still compile when swapped, plus a bare bool.
  `debug` is a runtime error unless `resultOut != nil`.
- `resultOut` concatenates every rewritten file into one writer without separator,
  which is only meaningful for a single file.
- `localImportPrefixes` can't be passed: `applyReplacements` calls `formatFileWithImports`
  without them (`rewrite.go:107`). Anyone needing local import grouping must write their own walker.

Proposal:

```go
type FileChange struct{ Path string; Old, New []byte }
func Rewrite(pattern string, log io.Writer, f PackageFunc) ([]FileChange, error)
func RewriteDryRun(pattern string, log io.Writer, f PackageFunc) ([]FileChange, error)
```

`resultOut` and `debug` disappear: a dry run returns `Old`/`New` and the caller prints or diffs them.
`DebugApply` stays for direct use, so debug markers can't be written into source files by misuse.

Open questions:

- [ ] Is losing the conflict-marker debug view of `RewriteWithReplacements` acceptable,
      or should there be a debug adapter that only works with `RewriteDryRun`?
- [ ] Should `Rewrite` return `[]FileChange` (with `Old`/`New` held in memory)
      or only the changed paths?
- [ ] Keep `log io.Writer`, or use `*slog.Logger`?
- [ ] Should multiple patterns be accepted, like the go command (`Rewrite(patterns []string, ...)`)?
- [ ] Should test files stay hard-coded as excluded (and a `_test.go` path an error),
      or become an option?

## 2. Callbacks receive the deprecated `*ast.Package` but not the source

Problem:

- `RewriteFileFunc` and `FileReplacementsFunc` (`rewrite.go:145`, `:30`) take
  `(fset, *ast.Package, *ast.File, filePath, verboseOut)`.
  `ast.Package` is deprecated since Go 1.22, `ParsePackage` returns it too (`parse.go:126`).
- The source bytes are already kept in `ParsedPackage.Sources`, but per-file callbacks
  don't get them, so a callback that needs the source must read the file again.
- `ParsedPackage` holds three parallel collections: `Files` map, `Sources` map, `FilePaths` slice.
  `FilePaths` means "files to rewrite", a rewrite concern in a parse result.
- `verboseOut` doesn't need to be in every callback signature, closures can capture their own logger.
- `ParsePackage` with a nil filter includes `_test.go` files, so any directory
  with an external `p_test` package fails with "2 packages found".

Proposal:

```go
type File struct{ Path string; Syntax *ast.File; Source []byte }
type Package struct {
	Dir, Name string
	Fset      *token.FileSet
	Files     []*File // sorted by Path
	Targets   []*File // files to rewrite: all, or the single named file
}
type PackageFunc func(pkg *Package) (map[string][]byte, error)
func PerFile(f func(pkg *Package, file *File) ([]byte, error)) PackageFunc
func PerFileReplacements(f func(pkg *Package, file *File) (NodeReplacements, Imports, error), localImportPrefixes ...string) PackageFunc
func ParsePackage(fset *token.FileSet, dir string, filter func(fs.DirEntry) bool) (*Package, error)
```

Open questions:

- [ ] `Targets []*File` field, a `Target bool` per `File`, or a separate callback parameter?
- [ ] Should `PackageFunc` return `map[string][]byte`, `map[*File][]byte`, or set a field on `File`?
- [ ] Should a nil `ParsePackage` filter mean "non-test files" like `go build`?
- [ ] Filter on `fs.DirEntry` (no stat per file) or just the file name?
- [ ] Should `ParsePackage` return the external `_test` package as well when tests are included?

## 3. `Imports` is a set of raw strings (solved)

Problem (`imports.go:41`, `Imports map[string]struct{}` of import lines):

- Zero value panics: `var imps Imports; imps.Add(...)` (confirmed).
  The zero value of `NodeReplacements` works, so the two collections behave differently.
- `Contains` matches a whole line or a bare path, `Remove` only the exact line (confirmed:
  for `f "fmt"`, `Contains("fmt")` is true but `Remove("fmt")` removes nothing).
  `"fmt"` and `fmt "fmt"` are different entries.
- Lines are split by searching for `"` instead of `strconv.Unquote`,
  so valid backquoted import paths fail (confirmed: `` `fmt` `` → "invalid quoted import").
- `FormatFileWithImports` takes an `fset` that its docs say is not used (`imports.go:276`).
- `LocatePackage` has three variants (`imports.go:110–206`) and each call runs `packages.Load`,
  which starts a `go list` process. For a standard library candidate it now also runs
  `go env GOROOT` (`isInGOROOT`), so locating N imports starts up to 2N processes.

Proposal:

```go
type Import struct{ Name, Path string } // Name "" = default import
type Imports map[Import]struct{}
func (imps *Imports) Add(path string) // creates the map if nil
func (imps *Imports) AddNamed(name, path string)
func ImportOf(spec *ast.ImportSpec) Import
func FormatSource(src []byte, imps Imports, localImportPrefixes ...string) ([]byte, error)
func LocatePackages(dir string, importPaths ...string) (map[string]*PackageLocation, error) // one go list
type PackageLocation struct{ Name, Dir string; Std bool } // was PkgName, SourcePath
```

Implemented as proposed, plus `Import.String()` (the import declaration form)
and `Imports.Sorted() []Import`. Removed `Imports.Contains`/`Remove` (map index and
`delete` are exact now), `ImportNameAndPathOfImportLine`, `LocatePackage`,
`LocatePackageOfImportLine`, `LocatePackageOfImportSpec` and `FormatFileWithImports`.
`ImportOf` returns `(Import, error)`, because `strconv.Unquote` can fail for a synthesized spec.

Decisions:

- [x] `map[Import]struct{}` or a deduplicating `[]Import`?
      `map[Import]struct{}`, `Sorted` gives a deterministic order.
- [x] How to handle the same path added with two different names?
      Both are kept as different imports, which is valid Go.
- [x] Keep a parser for import lines (`ParseImportLine`) at all, once `Import` is structured?
      No, all line-based functions are removed.
- [x] Should `LocatePackages` return partial results plus an error when some paths can't be resolved?
      No, it returns nil and one error that names every unresolved path.
      `go env GOROOT` runs at most once per call, so a call starts at most 2 processes.
- [x] The global `imports.LocalPrefix` mutex only protects calls through this package.
      Is there a way to pass local prefixes to `golang.org/x/tools/imports` without the global?
      No, `imports.Options` has no local prefix field (x/tools v0.52), the mutex stays.

Additional checks, so that mistakes fail loudly instead of writing broken imports:

- `FormatSource` rejects invalid import paths (`module.CheckImportPath`), like a quoted
  `"fmt"` from the old import line habit, and invalid import names.
- `LocatePackages` rejects paths that `go list` would resolve but that can't be mapped back
  to the requested path: relative `./sub`, absolute and `...` patterns,
  and the go command patterns `all`, `cmd`, `std`, `tool`, `work`.

## 4. The 56-method `Visitor` interface

Problem:

- A typo or wrong signature in an override (`VisitFuncdecl`, or missing the `Cursor` parameter)
  compiles because `VisitorImpl` is embedded, and is silently never called.
- No category handlers: handling every statement means overriding about 20 methods,
  there is no `ast.Stmt` or `ast.Expr` hook.
- `post` uses the same interface but its bool result is ignored (`visit.go:43`),
  and the whole traversal can't be stopped early (`astutil.Apply` supports this).
- `Cursor` is an interface and a new cursor is allocated for every pre and post callback
  (`cursor.go:97`).
- x/tools now has `inspector.All[N]` and `inspector.Cursor` for typed read-only traversal.
  What this package adds is typed traversal plus modification plus `Path`.

Proposal:

```go
func Visit(root ast.Node, handlers ...Handler) ast.Node
func Pre[N ast.Node](fn func(N, *Cursor) Action) Handler // N may be ast.Stmt, ast.Expr, ...
func Post[N ast.Node](fn func(N, *Cursor)) Handler
type Action int // Continue, SkipChildren, Stop
type Cursor struct{ /* concrete type, same methods as today */ }
```

The node type is inferred from the function parameter, so a typo can't compile,
and interface types give category handlers for free.

Open questions:

- [ ] Replace the `Visitor` interface, or offer the generic handlers next to it?
- [ ] Dispatch cost: linear type checks per handler, or a lookup by node type?
- [ ] `Action` enum or keep `bool` (and lose `Stop`)?
- [ ] Concrete `*Cursor` struct (no allocation per callback, no mocking) or keep the interface?
- [ ] Keep `VisitPackage` / `*ast.Package` support at all, given the deprecation?

## 5. `NodeReplacements.Apply` reorders the caller's slice (solved)

`apply` sorted the slice in place through a value receiver.
After `Apply`, `r[0]` was a different replacement (confirmed).

- [x] Sort a copy instead (`nodereplacements.go:201`),
      tested by `TestNodeReplacements_Apply_KeepsOrderOfReplacements`.
- [x] Unexport `Sort`, since `Apply` and `DebugApply` already sort.

## 6. `NodeReplacements` verbs don't match `Cursor`

`Cursor` has `Replace`/`Delete`/`InsertBefore`/`InsertAfter`,
`NodeReplacements` has `AddReplacement`/`AddRemoval`/`AddInsertAfter`/`Add`.
There is no insert-before, the workaround `AddReplacement(PosNode(pos), ...)` is hard to discover.

- [ ] Rename to the same four verbs as `Cursor`, plus `Append` for merging?
- [ ] Keep `Replacement any` (string, `[]byte`, or anything `format.Node` accepts),
      which fails only at `Apply` time for unsupported types?
- [ ] Keep `debugID ...string` as a variadic optional parameter
      (several values are silently concatenated)?

## 7. `ExprString` is named like a printer but changes the code (solved)

`interface{}` became `any`, `T{a, b}` became `T{ a, b }` and `struct{ A int }` became
`struct { A int }`, `FuncLit` bodies became `{ ... }` (all confirmed again on 2026-10-13,
commit `275ff9c` made the output parse back but kept these normalizations),
and unknown types produced `"TODO %T"`.

- [x] Print faithfully (gofmt-equal), or rename/document it as a compact display form?
      Faithful on a single line: `interface{}` stays, gofmt spacing like `T{a, b}`,
      `struct{ A int; B string }` and `interface{ M() }`.
      Not gofmt-equal for binary operators, which always have spaces around them
      (gofmt's precedence-based spacing is not reimplemented), and for struct and interface
      types with several fields, which gofmt writes on several lines.
- [x] Should the `{ ... }` elision of function literal bodies stay?
      No, function literals are formatted by gofmt as if their source was on one line,
      so a short body stays on the line and a longer one gets a statement per line.
      Names in function literals are not qualified by `ExprStringWithExportedNameQualifyer`.

Unknown expression types are written as `(ast: <type>)` like `types.ExprString`,
`go/printer` would panic for them.

## 8. Misspelled exported names

- [ ] `ExprStringWithExportedNameQualifyer` → `QualifiedExprString`?
- [ ] `TypeExprNameQualifyers(expr, map)` → `Qualifiers(expr) []string`,
      returning the result instead of filling a passed-in map?

## 9. `PathItem.String` has a pointer receiver (solved, one question open)

`fmt.Print(path[i])` printed `{Body -1 <nil> BlockStmt}` (confirmed).

- [x] Use a value receiver (`pathitem.go:54`), tested by `TestPathItem_String_Value`.
- [ ] Is the `PathItem.Type` string field needed, or can it be derived with `NodeType(Node)`?

## 10. `NodeType` and `Visit` panic on unknown node types (solved)

`NodeType(PosNode(1))` panicked (confirmed), `NodeRange` too.
`Visit` panicked on any node type go/ast adds in the future.
Go 1.26 already added `*ast.Directive`, which `NodeType` didn't know
(it isn't part of the tree, so `Visit` only sees it as the root).

- [x] Fall back to the `%T` type name in `NodeType`, without the `*ast.` prefix
      (`nodetype.go:196`), tested by `TestNodeType`.
- [x] Should `Visit` skip unknown node types instead of panicking?
      Yes, but together with their children (`visit.go:249` returns false):
      `astutil.Apply` calls the pre function before its own type switch panics
      for a node type it doesn't support, so traversing the children would still panic.
      Tested by `TestVisit_UnknownNodeTypeDoesNotPanic` and
      `TestVisitNode_UnknownNodeTypeIsNotTraversed`.
      Trade-off: a future node type that `astutil.Apply` does support
      is skipped with its children until this package gets a Visitor method for it.

## 11. `FprintfVerbose` is exported

`rewrite.go:468`: a generic helper exported only because callbacks receive `verboseOut`.

- [ ] Unexport it once callbacks no longer take `verboseOut` (see 1 and 2).

## 12. Inconsistent source input types

`ParseDeclarations` and `ParseStatements` take `string`, the formatting functions take `[]byte`.

- [ ] Standardize on `[]byte`, `string`, or a generic `string | []byte`?
- [ ] Should `ParseDeclarations`/`ParseStatements` return a struct instead of three values?
