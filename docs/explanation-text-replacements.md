# Why NodeReplacements edits source text instead of reprinting the AST

A codemod usually changes a few expressions in a file full of comments, blank
lines and hand-tuned layout. If the tool reprints the whole syntax tree, those
details are what get damaged. `NodeReplacements` avoids reprinting: it uses the
parsed AST only to find byte ranges, then splices new text into the original
source bytes. Everything outside the replaced ranges is copied unchanged.

This page explains that design and what it costs. For the API itself see the
[godoc](https://pkg.go.dev/github.com/ungerik/go-astvisit#NodeReplacements).
For the task-level guide see
[How to rewrite Go source files](howto-rewrite-source-files.md).

## The problem

`go/printer` (and `go/format`, which wraps it) prints a tree using the token
positions stored in its nodes. Comments are not part of the tree. They live in
`ast.File.Comments` and the printer places them between nodes by comparing
positions. So when you modify a tree with a `Cursor` and print it, positions
decide the layout:

- **Deleted nodes leave gaps.** The printer sees a jump in line numbers where
  the node used to be and prints a blank line.
- **Inserted nodes with positions from somewhere else add blank lines.** A
  statement parsed with `ParseStatements` keeps positions from its own
  synthetic file, so the printer thinks it is far away from its neighbours.
- **Hand-built nodes have no position at all** (`token.NoPos`). They print
  cleanly, but comments that belonged near the old node no longer have a
  position to attach to and can move or end up in odd places.

Take the example in [How to modify an AST with Cursor](howto-modify-ast-with-cursor.md),
which deletes `debugLog("compute", x)` and inserts `log.Println("return")`
before each `return`. With the inserted statement parsed by `ParseStatements`
instead of built by hand, it prints:

```go
func compute(x int) int {

	if x > limit {
		log.Println("return")

		return limit
	}
```

Both blank lines are side effects of the printer seeing position gaps. The code
is correct, but a codemod that adds noise like this to a diff has to be cleaned
up by hand.

## The approach

`NodeReplacements` never prints the file. It turns each node into a byte range
`[Pos, End)` of the original source and walks the source once from start to
end:

```
source:  package p⏎⏎func a() {}⏎⏎func b() {}⏎⏎func c() {}⏎
                     └─Decls[0]─┘   └─Decls[1]─┘
edits:   Decls[0] → "func x() {}⏎"   Decls[1] → nil (removal)

sweep:   copy "package p⏎⏎"
         write "func x() {}⏎", skip Decls[0] and the ⏎ after it
         skip the "⏎" before Decls[1] (blank line before a removed
         line that is followed by a blank line)
         write nothing, skip Decls[1] and the ⏎ after it
         copy "⏎func c() {}⏎"

result:  package p⏎⏎func x() {}⏎⏎func c() {}⏎
```

The result is what `Apply` returns for this input. The tests in `rewrite_test.go`
run the same edits on a CRLF file through `RewriteWithReplacements`.

The steps of `NodeReplacements.Apply` (`nodereplacements.go`):

1. **Sort.** Replacements are sorted by start position, then by end position,
   with a stable sort. A zero-width insertion at position P comes before a
   replacement that starts at P. Several insertions at the same position keep
   the order in which you added them.
2. **Check the file.** Every node must be from the file that `source` was
   parsed from: a node of another file, a node without a position or with a
   synthesized end, or a `source` with a different size than the parsed file
   returns `replacement node <type> at <pos> is not from the source`, a `nil`
   node returns `replacement node <type> is nil and not from the source`.
   `Apply` alone can't tell apart two files of the same size;
   `RewriteWithReplacements` also compares the file name.
3. **Map positions to offsets.** `fset.Position(pos).Offset` gives the byte
   offset of each node in the source. The end of a `/* */` comment is taken
   from the source instead: go/scanner removes the carriage returns from
   comment text, so with CRLF line endings `ast.Comment.End()` is one byte
   early per line break.
4. **Validate.** A range that starts
   before the previous one ended returns
   `replacement node at <pos> overlaps the replacement node at <pos>`.
5. **Copy the gap, write the replacement.** The bytes between the previous
   edit and this one are copied. Then the replacement is written: `nil`
   writes nothing, a `string` or `[]byte` is written as is, anything else is
   printed with `format.Node`.
6. **Clean up whitespace.**
   - A removed node on a line of its own is removed with the whole line,
     its indentation and its newline (LF or CRLF), so removing `a()` from
     `"{\n\ta()\n\tb()\n}"` gives `"{\n\tb()\n}"`. A removed node that
     shares its line with code before it keeps the line break and loses the
     space before it, so removing the comment of `"a := b // nolint\n"` gives
     `"a := b\n"`. Blank lines right before a removed line are dropped only if
     a blank line follows it, else a comment before them would become the doc
     comment of the code after the removed line. They are also only dropped
     if nothing but whitespace separates them from the previous edit and that
     edit doesn't end in the middle of a line. Otherwise they are kept, and
     `RewriteWithReplacements` collapses a resulting double blank line when
     it formats the file.
   - A replacement that ends with a newline also consumes the newline after
     the node, so lines don't double up.
   - A zero-width insertion (from `AddInsertAfter` or a `PosNode`) never
     consumes source bytes.
7. **Copy the rest** of the source after the last edit.

### What RewriteWithReplacements adds on top

`RewriteWithReplacements` applies the edits to the exact bytes that were parsed
(`ParsedPackage.Sources`), never to a fresh read of the file. Then:

```
rewritten := Apply(source)
rewritten == source?                          → unchanged
format(rewritten, imports) == format(source, imports)?  → unchanged
otherwise                                     → write format(rewritten, imports)
```

Formatting both sides before comparing means a replacement that only changes
whitespace doesn't touch the file, and a file that wasn't gofmt-clean doesn't
get reformatted by a no-op run. `format` here is `FormatSource`: gofmt
output, plus goimports-style grouping when it had to add a missing import.

## Trade-offs

- **Edits can't nest.** One byte range can be edited once. If you replace a
  call and also want to change an argument inside it, build the whole new call
  yourself, or return `false` from the visitor after the outer match and run
  the tool again. The [tutorial](tutorial-write-a-codemod.md) does the latter.
- **Positions must belong to the bytes being patched.** Replacements are only
  valid for the source they were parsed from, with the same `token.FileSet`.
  A node from another parse, or from `ParseStatements`/`ParseDeclarations`
  (which parse a wrapped copy of your snippet), can be a replacement *value*
  but never a replacement *target*. Using one as a target returns an error.
- **Text edits can produce invalid Go.** The formatting step parses the result,
  so a broken edit returns an error like
  `invalid Go code after applying replacements: /abs/p.go:3:6: expected 'IDENT', found '{'`
  and nothing is written. The position is in the rewritten source, not in the
  file on disk. Use debug mode (below) to see which edit broke it.
- **A real change gofmts the whole file.** Formatting is what makes the output
  stable, but a file that wasn't gofmt-clean gets fully reformatted as soon as
  one real edit lands in it.
- **No type information.** You match nodes by syntax (`ExprString(x) ==
  "time.Now()"`). A shadowed or renamed import can fool such a match. The
  library doesn't load `go/types` for you.

### Debug mode

`DebugApply` (or `RewriteWithReplacements` with `debug` set) writes each edit
with conflict-style markers instead of applying it, so you can review exactly
which bytes each replacement covers:

```
const dayLayout = 
<<<<<<< /path/to/proj/dates.go
"2006-01-02"
=======
time.DateOnly
>>>>>>> date-layout
```

Every marker is on a line of its own. Between `=======` and `>>>>>>>` is the
replacement, after `>>>>>>>` the optional debug ID passed to `AddReplacement`. The output is not Go, which is
why `RewriteWithReplacements` refuses to run in debug mode without a
`resultOut` writer.

## Other approaches and when they fit better

The repository doesn't record which alternatives were weighed, so this is a
comparison, not history.

- **Modify the tree with `Cursor` and print it** (`Visit` plus `Rewrite`). Fine
  for generated files or code where you don't care about the blank lines and
  comment placement described above. Simpler when the edit is structural, like
  reordering declarations.
- **`golang.org/x/tools/go/analysis` suggested fixes.** Also position-based
  text edits, but they run inside the analysis framework with full type
  information. Better when matches need types (method sets, which package an
  identifier refers to).
- **`github.com/dave/dst`.** A "decorated" syntax tree that attaches comments
  and spacing to nodes, so reprinting keeps them. Better when you need many
  structural edits and still want comments to follow their nodes.

## Related

- [How to rewrite Go source files](howto-rewrite-source-files.md)
- [How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md)
- [Tutorial: write your first codemod](tutorial-write-a-codemod.md)
