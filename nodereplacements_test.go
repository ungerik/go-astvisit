package astvisit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func parseTestFile(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)
	return fset, file
}

// Generators like go-enum add several insertions after the same declaration.
// They must not panic and must appear in the order they were added,
// otherwise the generated code order would be random.
func TestNodeReplacements_Apply_InsertionsAtSamePosition(t *testing.T) {
	src := "package p\n\nconst A = 1\n\nfunc f() {}\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddInsertAfter(file.Decls[0], "\n// first\n")
	repls.AddInsertAfter(file.Decls[0], "\n// second\n")
	repls.AddInsertAfter(file.Decls[0], "\n// third\n")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)

	// A zero-width insertion never consumes source bytes,
	// so the "\n\n" after the const declaration is kept
	require.Equal(t, "package p\n\nconst A = 1\n// first\n\n// second\n\n// third\n\n\nfunc f() {}\n", string(result))
}

// An insertion must not eat the source that follows it,
// or code after the insertion point would get lost.
func TestNodeReplacements_Apply_InsertionKeepsFollowingSource(t *testing.T) {
	src := "package p\n\nvar a = 1\nvar b = 2\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddInsertAfter(file.Decls[0], "\nvar inserted = 3\n")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nvar a = 1\nvar inserted = 3\n\nvar b = 2\n", string(result))
}

// Sorting by (Pos, End) puts an insertion before a replacement that starts
// at the same position, independent of the order they were added in.
func TestNodeReplacements_Apply_InsertionBeforeReplacementAtSamePos(t *testing.T) {
	src := "package p\n\nfunc f() {}\n"
	fset, file := parseTestFile(t, src)
	funcDecl := file.Decls[0]

	var repls NodeReplacements
	repls.AddReplacement(funcDecl, "func g() {}")
	repls.AddReplacement(PosNode(funcDecl.Pos()), "// inserted\n")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\n// inserted\nfunc g() {}\n", string(result))
}

// Removing a node and inserting after it replaces the node's lines.
// The removal skips the node's trailing newline,
// which must not make the insertion at that newline fail.
func TestNodeReplacements_Apply_RemovalAndInsertAfterSameNode(t *testing.T) {
	src := "package p\n\nvar a = 1\nvar b = 2\nvar c = 3\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddRemoval(file.Decls[1])
	repls.AddInsertAfter(file.Decls[1], "var x = 4\n")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nvar a = 1\nvar x = 4\nvar c = 3\n", string(result))
}

// Generators replace or remove a declaration and add several declarations after it.
// The newline skipped by the removal or replacement must not make
// the second insertion look like it overlaps the first one.
func TestNodeReplacements_Apply_SeveralInsertionsAfterSkippedNewline(t *testing.T) {
	src := "package p\n\nvar a = 1\nvar b = 2\nvar c = 3\n"

	for name, replacement := range map[string]any{
		"removal":     nil,
		"replacement": "var b = 20\n",
	} {
		t.Run(name, func(t *testing.T) {
			fset, file := parseTestFile(t, src)
			var repls NodeReplacements
			repls.AddReplacement(file.Decls[1], replacement)
			repls.AddInsertAfter(file.Decls[1], "var x = 4\n")
			repls.AddInsertAfter(file.Decls[1], "var y = 5\n")
			result, err := repls.Apply(fset, []byte(src))
			require.NoError(t, err)
			if replacement == nil {
				require.Equal(t, "package p\n\nvar a = 1\nvar x = 4\nvar y = 5\nvar c = 3\n", string(result))
			} else {
				require.Equal(t, "package p\n\nvar a = 1\nvar b = 20\nvar x = 4\nvar y = 5\nvar c = 3\n", string(result))
			}
		})
	}
}

// A removal skips the indentation before the removed node,
// but if the previous replacement ended within its line, the line break
// must be kept or the following line gets appended to that line.
// That produces invalid code, or silently comments out code
// when the previous replacement ended with a line comment.
func TestNodeReplacements_Apply_RemovalAfterReplacementKeepsLineBreak(t *testing.T) {
	t.Run("replaced statement", func(t *testing.T) {
		src := "package p\n\nfunc f() {\n\ta := 1\n\tb := 2\n\tc := 3\n}\n"
		fset, file := parseTestFile(t, src)
		stmts := file.Decls[0].(*ast.FuncDecl).Body.List

		var repls NodeReplacements
		repls.AddReplacement(stmts[0], "a := 10")
		repls.AddRemoval(stmts[1])
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "package p\n\nfunc f() {\n\ta := 10\n\tc := 3\n}\n", string(result))
	})

	t.Run("inserted line comment", func(t *testing.T) {
		src := "package p\n\nvar a = 1\nvar b = 2\nvar c = 3\n"
		fset, file := parseTestFile(t, src)

		var repls NodeReplacements
		repls.AddInsertAfter(file.Decls[0], " // a is special")
		repls.AddRemoval(file.Decls[1])
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "package p\n\nvar a = 1 // a is special\nvar c = 3\n", string(result))
	})
}

// Conflicting replacements are a bug in the caller
// that must be reported instead of crashing the tool.
func TestNodeReplacements_Apply_OverlappingReplacementsError(t *testing.T) {
	src := "package p\n\nfunc f() {}\n"
	fset, file := parseTestFile(t, src)
	funcDecl := file.Decls[0].(*ast.FuncDecl)

	t.Run("nested", func(t *testing.T) {
		var repls NodeReplacements
		repls.AddReplacement(funcDecl, "func g() {}")
		repls.AddReplacement(funcDecl.Name, "h")
		require.NotPanics(t, func() {
			_, err := repls.Apply(fset, []byte(src))
			require.ErrorContains(t, err, "overlaps")
		})
	})

	t.Run("same node twice", func(t *testing.T) {
		var repls NodeReplacements
		repls.AddReplacement(funcDecl, "func g() {}")
		repls.AddRemoval(funcDecl)
		require.NotPanics(t, func() {
			_, err := repls.Apply(fset, []byte(src))
			require.ErrorContains(t, err, "overlaps")
		})
	})

	t.Run("insertion inside replaced node", func(t *testing.T) {
		var repls NodeReplacements
		repls.AddReplacement(funcDecl, "func g() {}")
		repls.AddInsertAfter(funcDecl.Name, "_suffix")
		require.NotPanics(t, func() {
			_, err := repls.Apply(fset, []byte(src))
			require.ErrorContains(t, err, "overlaps")
		})
	})
}

// A node from another file or a source that doesn't match the FileSet
// must return an error instead of panicking.
func TestNodeReplacements_Apply_NodeOutsideSource(t *testing.T) {
	src := "package p\n\nfunc f() {}\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddRemoval(file.Decls[0])
	require.NotPanics(t, func() {
		_, err := repls.Apply(fset, []byte("package p\n"))
		require.Error(t, err)
	})
}

// Files with CRLF line endings, like in Windows checkouts with core.autocrlf,
// must have the whole line of a removed node removed, the same as with LF.
func TestNodeReplacements_Apply_RemovalWithCRLF(t *testing.T) {
	src := "package p\r\n\r\nvar a = 1\r\nvar b = 2\r\nvar c = 3\r\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddRemoval(file.Decls[1])
	repls.AddReplacement(file.Decls[2], "var d = 4\n")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\r\n\r\nvar a = 1\r\nvar d = 4\n", string(result))
}

// []byte and ast.Node replacements must handle the newline
// after the replaced node like string replacements do.
func TestNodeReplacements_Apply_ByteAndNodeReplacements(t *testing.T) {
	src := "package p\n\nvar a = 1\nvar b = 2\nvar c = 3\n"

	fset, file := parseTestFile(t, src)
	var repls NodeReplacements
	repls.AddReplacement(file.Decls[1], []byte("var b = 20\n"))
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nvar a = 1\nvar b = 20\nvar c = 3\n", string(result))

	fset, file = parseTestFile(t, src)
	repls = nil
	repls.AddReplacement(file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0], ast.NewIdent("two"))
	result, err = repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nvar a = 1\nvar b = two\nvar c = 3\n", string(result))
}

// sort must be stable so that replacements at the same position
// keep the order they were added in.
func TestNodeReplacements_sort_Stable(t *testing.T) {
	var repls NodeReplacements
	for i := range 20 {
		repls.AddReplacement(PosNode(10), i)
	}
	repls.AddReplacement(PosNode(5), "first")
	repls.sort()
	require.Equal(t, "first", repls[0].Replacement)
	for i, repl := range repls[1:] {
		require.Equal(t, i, repl.Replacement)
	}
}

// Generators report or reuse their replacements after applying them,
// for example to list outdated code with the DebugIDs in a validation mode.
// Apply and DebugApply must not reorder the caller's replacements as a side effect,
// else those reports would silently change their order.
func TestNodeReplacements_Apply_KeepsOrderOfReplacements(t *testing.T) {
	src := "package p\n\nvar a = 1\nvar b = 2\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddReplacement(file.Decls[1], "var b = 3\n", "second decl")
	repls.AddReplacement(file.Decls[0], "var a = 4\n", "first decl")

	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nvar a = 4\nvar b = 3\n", string(result))
	require.Equal(t, "second decl", repls[0].DebugID, "Apply changed the order")
	require.Equal(t, "first decl", repls[1].DebugID, "Apply changed the order")

	_, err = repls.DebugApply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "second decl", repls[0].DebugID, "DebugApply changed the order")
	require.Equal(t, "first decl", repls[1].DebugID, "DebugApply changed the order")
}

// DebugApply output is read like a git merge conflict.
// Every marker must be on its own line, else a replacement can't be
// told apart from the "=======" marker and editors don't recognize the markers.
func TestNodeReplacements_DebugApply_MarkersOnOwnLines(t *testing.T) {
	src := "package p\n\nconst layout = \"2006-01-02\"\n\nfunc f() {}\n"
	fset, file := parseTestFile(t, src)
	layout := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]

	var repls NodeReplacements
	repls.AddReplacement(layout, "time.DateOnly", "date-layout")
	repls.AddRemoval(file.Decls[1], "remove-f")
	result, err := repls.DebugApply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\n\nconst layout = \n"+
		"<<<<<<< test.go\n\"2006-01-02\"\n=======\ntime.DateOnly\n>>>>>>> date-layout\n"+
		"<<<<<<< test.go\nfunc f() {}\n=======\n>>>>>>> remove-f\n",
		string(result),
	)
}

// go/scanner removes the carriage returns from the text of comments,
// so ast.Comment.End is too early for a /*-style comment with CRLF line endings.
// A license header codemod in a Windows checkout must replace, remove or append
// to the whole comment, leftovers like "*/" would silently corrupt the file.
func TestNodeReplacements_Apply_CRLFBlockComment(t *testing.T) {
	const comment = "/*\r\n * Old Corp\r\n */"
	const src = comment + "\r\n\r\npackage p\r\n"

	t.Run("replace comment group", func(t *testing.T) {
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddReplacement(file.Comments[0], "// New Corp")
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "// New Corp\r\n\r\npackage p\r\n", string(result))
	})

	t.Run("replace comment", func(t *testing.T) {
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddReplacement(file.Comments[0].List[0], "// New Corp")
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "// New Corp\r\n\r\npackage p\r\n", string(result))
	})

	t.Run("remove", func(t *testing.T) {
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddRemoval(file.Comments[0])
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "\r\npackage p\r\n", string(result))
	})

	t.Run("insert after", func(t *testing.T) {
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddInsertAfter(file.Comments[0], "\r\n\r\n//go:generate echo hi")
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, comment+"\r\n\r\n//go:generate echo hi\r\n\r\npackage p\r\n", string(result))
	})
}

// A codemod that collects nodes of the whole package instead of the current file,
// or that builds nodes without positions, must get an error instead of
// a silent edit at offsets that belong to other bytes, or a panic in debug mode.
func TestNodeReplacements_Apply_NodeNotFromSourceError(t *testing.T) {
	fset := token.NewFileSet()
	src := "package p\n\nfunc f() {}\n\nvar y = 2\n"
	file, err := parser.ParseFile(fset, "a.go", src, 0)
	require.NoError(t, err)
	other, err := parser.ParseFile(fset, "b.go", "package p\n\nvar x = 1\n", 0)
	require.NoError(t, err)
	sameSize, err := parser.ParseFile(fset, "c.go", "package p\n\nfunc g() {}\n\nvar z = 3\n", 0)
	require.NoError(t, err)
	require.Equal(t, len(src), fset.File(sameSize.Pos()).Size())

	for name, add := range map[string]func(*NodeReplacements){
		"node of other file": func(repls *NodeReplacements) {
			repls.AddReplacement(other.Decls[0], "x")
		},
		"synthesized node": func(repls *NodeReplacements) {
			repls.AddReplacement(ast.NewIdent("synthesized"), "x")
		},
		"insert after synthesized node": func(repls *NodeReplacements) {
			repls.AddInsertAfter(ast.NewIdent("synthesized"), "x")
		},
		"node of other file after node of source": func(repls *NodeReplacements) {
			repls.AddReplacement(file.Decls[0], "func g() {}")
			repls.AddReplacement(other.Decls[0], "var z = 3")
		},
		"nil node": func(repls *NodeReplacements) {
			repls.AddRemoval(nil)
		},
		"typed nil node": func(repls *NodeReplacements) {
			var decl *ast.FuncDecl
			repls.AddRemoval(decl)
		},
		"insert after typed nil node": func(repls *NodeReplacements) {
			var decl *ast.FuncDecl
			repls.AddInsertAfter(decl, "x")
		},
		// Apply can't compare file names, only the files of the nodes
		"node of same size file after node of source": func(repls *NodeReplacements) {
			repls.AddReplacement(file.Decls[0], "func h() {}")
			repls.AddRemoval(sameSize.Decls[1])
		},
	} {
		t.Run(name, func(t *testing.T) {
			var repls NodeReplacements
			add(&repls)
			require.NotPanics(t, func() {
				_, err := repls.Apply(fset, []byte(src))
				require.ErrorContains(t, err, "not from the source")
				_, err = repls.DebugApply(fset, []byte(src))
				require.ErrorContains(t, err, "not from the source")
			})
		})
	}
}

// The end of a node must be checked like its start: a NodeRange spanning
// two files or a partly built node with a valid start but a synthesized end
// must return an error instead of editing unrelated bytes of source.
func TestNodeReplacements_Apply_NodeEndNotFromSourceError(t *testing.T) {
	fset := token.NewFileSet()
	src := "package p\n\nvar a = 1\n\nvar b = 2\n\nvar c = 3\n"
	file, err := parser.ParseFile(fset, "a.go", src, 0)
	require.NoError(t, err)
	other, err := parser.ParseFile(fset, "b.go", "package p\n\n// padding comment\nvar x = 1\n", 0)
	require.NoError(t, err)
	ident := file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0]

	for name, add := range map[string]func(*NodeReplacements){
		"node range spanning files": func(repls *NodeReplacements) {
			repls.AddRemoval(NodeRange{file.Decls[0], other.Decls[0]})
		},
		"replace node with synthesized end": func(repls *NodeReplacements) {
			repls.AddReplacement(&ast.BinaryExpr{X: ident, Y: ast.NewIdent("c")}, "x")
		},
		"insert after node with synthesized end": func(repls *NodeReplacements) {
			repls.AddInsertAfter(&ast.CallExpr{Fun: ident}, "// x\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var repls NodeReplacements
			add(&repls)
			_, err := repls.Apply(fset, []byte(src))
			require.ErrorContains(t, err, "not from the source")
		})
	}
}

// A NodeRange ending with a /*-comment in a CRLF file must be replaced up to
// the end of the comment in source, else "*/" is left behind and breaks the file.
func TestNodeReplacements_Apply_NodeRangeEndingWithCRLFBlockComment(t *testing.T) {
	src := "package p\r\n\r\nvar a = 1 /*\r\n x\r\n */\r\nvar b = 2\r\n"
	fset, file := parseTestFile(t, src)

	var repls NodeReplacements
	repls.AddReplacement(NodeRange{file.Decls[0], file.Comments[0]}, "var a = 10")
	result, err := repls.Apply(fset, []byte(src))
	require.NoError(t, err)
	require.Equal(t, "package p\r\n\r\nvar a = 10\r\nvar b = 2\r\n", string(result))
}

// Rewrite and RewritePackages callers can write the result of Apply without gofmt,
// so removing a node on a line of its own must remove the whole line,
// including its indentation, else the next line gets doubled indentation.
func TestNodeReplacements_Apply_RemovalRemovesIndentation(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"first statement": {
			src:  "package p\n\nfunc f() {\n\ta()\n\tb()\n}\n",
			want: "package p\n\nfunc f() {\n\tb()\n}\n",
		},
		"last statement": {
			src:  "package p\n\nfunc f() {\n\tb()\n\ta()\n}\n",
			want: "package p\n\nfunc f() {\n\tb()\n}\n",
		},
		"CRLF": {
			src:  "package p\r\n\r\nfunc f() {\r\n\ta()\r\n\tb()\r\n}\r\n",
			want: "package p\r\n\r\nfunc f() {\r\n\tb()\r\n}\r\n",
		},
		"code after node on same line": {
			// The line stays, so its indentation must stay too
			src:  "package p\n\nfunc f() {\n\ta(); b()\n}\n",
			want: "package p\n\nfunc f() {\n\t; b()\n}\n",
		},
		"code before node on same line": {
			// The line stays, so its line break must stay too
			src:  "package p\n\nfunc f() {\n\tb(); a()\n}\n",
			want: "package p\n\nfunc f() {\n\tb();\n}\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fset, file := parseTestFile(t, tc.src)
			var removal ast.Node
			ast.Inspect(file, func(node ast.Node) bool {
				if call, ok := node.(*ast.ExprStmt); ok && ExprString(call.X) == "a()" {
					removal = call
				}
				return true
			})
			var repls NodeReplacements
			repls.AddRemoval(removal)
			result, err := repls.Apply(fset, []byte(tc.src))
			require.NoError(t, err)
			require.Equal(t, tc.want, string(result))
		})
	}
}

// Removing a node that shares its line with code before it, like a trailing comment,
// must keep the line break after it, else the line merges with the next one
// into invalid Go and RewriteWithReplacements can't remove trailing comments at all.
func TestNodeReplacements_Apply_RemovalWithinLineKeepsLineBreak(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"trailing comment": {
			src:  "package p\n\nfunc f(b, d int) {\n\ta := b // nolint\n\tc := d\n\t_, _ = a, c\n}\n",
			want: "package p\n\nfunc f(b, d int) {\n\ta := b\n\tc := d\n\t_, _ = a, c\n}\n",
		},
		"trailing comment CRLF": {
			src:  "package p\r\n\r\nfunc f(b, d int) {\r\n\ta := b // nolint\r\n\tc := d\r\n\t_, _ = a, c\r\n}\r\n",
			want: "package p\r\n\r\nfunc f(b, d int) {\r\n\ta := b\r\n\tc := d\r\n\t_, _ = a, c\r\n}\r\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fset, file := parseTestFile(t, tc.src)
			var repls NodeReplacements
			repls.AddRemoval(file.Comments[0])
			result, err := repls.Apply(fset, []byte(tc.src))
			require.NoError(t, err)
			require.Equal(t, tc.want, string(result))
		})
	}
}

// The blank lines before a removed line are only dropped if a blank line follows it,
// else a comment before the removed line would become the doc comment
// of the next declaration and change what go doc shows.
func TestNodeReplacements_Apply_RemovalKeepsBlankLineBeforeCode(t *testing.T) {
	t.Run("blank line follows", func(t *testing.T) {
		src := "package p\n\nfunc a() {}\n\nfunc b() {}\n\nfunc c() {}\n"
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddReplacement(file.Decls[0], "func x() {}\n")
		repls.AddRemoval(file.Decls[1])
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "package p\n\nfunc x() {}\n\nfunc c() {}\n", string(result))
	})

	t.Run("code follows", func(t *testing.T) {
		src := "package p\n\n// Section: old\n\nvar B = 1\nvar C = 2\n"
		fset, file := parseTestFile(t, src)
		var repls NodeReplacements
		repls.AddReplacement(file.Comments[0], "// Section: new\n")
		repls.AddRemoval(file.Decls[0])
		result, err := repls.Apply(fset, []byte(src))
		require.NoError(t, err)
		require.Equal(t, "package p\n\n// Section: new\n\nvar C = 2\n", string(result))
	})
}
