# go-astvisit documentation

The API reference is the godoc on
[pkg.go.dev](https://pkg.go.dev/github.com/ungerik/go-astvisit). These pages
cover what godoc doesn't: how to get started, how to do common tasks, and why
the library works the way it does.

## Start here

- [Tutorial: write your first codemod](tutorial-write-a-codemod.md): build a
  tool that rewrites `time.Now().Sub(t)` to `time.Since(t)` across a module,
  with preview, write and CI check modes.

## How-to guides

- [How to modify an AST with Cursor](howto-modify-ast-with-cursor.md):
  replace, delete and insert nodes during `Visit`, edit bottom-up with a post
  visitor, print the result.
- [How to rewrite Go source files](howto-rewrite-source-files.md): choose
  between `Rewrite`, `RewriteWithReplacements` and `RewritePackages`, add
  imports, preview and debug edits.
- [How to check in CI that no file needs rewriting](howto-check-generated-code-in-ci.md):
  use `RewriteDryRun` as a Go test or a `-check` flag.

## Explanation

- [Why NodeReplacements edits source text instead of reprinting the AST](explanation-text-replacements.md):
  how edits keep comments and layout, the whitespace rules, and the limits.
- [How the Rewrite functions choose files and when they write](explanation-file-selection-writes.md):
  which files and directories are included, when files are written, and
  where the behavior differs from the go command.

## Reference

- [pkg.go.dev/github.com/ungerik/go-astvisit](https://pkg.go.dev/github.com/ungerik/go-astvisit),
  or `go doc -all github.com/ungerik/go-astvisit` locally.
