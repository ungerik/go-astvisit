package astvisit

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"math"
	"reflect"
	"slices"
	"strings"
)

// PosNode implements ast.Node by returning the same underlying token.Pos
// from both its Pos() and End() methods. This creates a zero-width node that
// can be used as a marker for code insertions at a specific position.
//
// Example:
//
//	// Create an insertion point before a node
//	insertPos := PosNode(node.Pos())
//	replacements.AddReplacement(insertPos, "// New comment\n")
//
// Use NodeReplacements.AddInsertAfter to insert after a node,
// because the End of a /*-style comment is too early for CRLF line endings.
type PosNode token.Pos

func (p PosNode) Pos() token.Pos { return token.Pos(p) }
func (p PosNode) End() token.Pos { return token.Pos(p) }

// insertAfter is the zero-width node at the end of Node
// used for the insertions of NodeReplacements.AddInsertAfter.
// It keeps Node so that the end can be taken from the source
// where the End method of Node is wrong, see nodeOffsets.
type insertAfter struct {
	ast.Node
}

func (n insertAfter) Pos() token.Pos { return n.Node.End() }

// NodeRange represents a slice of nodes as a single ast.Node.
// The Pos() returns the minimum position and End() returns the maximum position
// of all contained nodes.
type NodeRange []ast.Node

func (r NodeRange) Pos() token.Pos {
	switch len(r) {
	case 0:
		return token.NoPos
	case 1:
		return r[0].Pos()
	}
	min := token.Pos(math.MaxInt)
	for _, n := range r {
		if p := n.Pos(); p < min {
			min = p
		}
	}
	return min
}

func (r NodeRange) End() token.Pos {
	switch len(r) {
	case 0:
		return token.NoPos
	case 1:
		return r[0].End()
	}
	max := token.Pos(-math.MaxInt)
	for _, n := range r {
		if p := n.End(); p > max {
			max = p
		}
	}
	return max
}

// NodeReplacement represents a single node replacement operation in source code.
type NodeReplacement struct {
	Node        ast.Node // The node to replace
	Replacement any      // nil (removal), string, []byte, or anything accepted by format.Node
	DebugID     string   // Optional identifier for debugging
}

// NodeReplacements is a collection of node replacement operations that can be
// applied to source code. Replacements are applied in source order (by position).
// Replacements at the same position, like several insertions after the same node,
// are applied in the order they were added.
//
// Example:
//
//	var replacements NodeReplacements
//	replacements.AddReplacement(oldNode, newNode, "fix-001")
//	replacements.AddInsertAfter(node, "// TODO: optimize\n")
//	replacements.AddRemoval(unusedNode)
//	newSource, err := replacements.Apply(fset, originalSource)
type NodeReplacements []NodeReplacement

// AddReplacement adds a node replacement operation.
// The replacement can be:
//   - nil: Remove the node
//   - string or []byte: Replace with literal source code
//   - ast.Node: Replace with formatted node
//   - Any type accepted by go/format.Node
//
// debugID is optional and used for identifying replacements in debug output.
func (repls *NodeReplacements) AddReplacement(node ast.Node, replacement any, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        node,
		Replacement: replacement,
		DebugID:     strings.Join(debugID, ""),
	})
}

// AddInsertAfter adds an insertion immediately after the given node.
// The insertion can be a string, []byte, or ast.Node.
//
// Example:
//
//	// Add a comment after a function
//	replacements.AddInsertAfter(funcDecl, "\n// Deprecated: use NewFunc instead\n")
func (repls *NodeReplacements) AddInsertAfter(node ast.Node, insertion any, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        insertAfter{node},
		Replacement: insertion,
		DebugID:     strings.Join(debugID, ""),
	})
}

// AddRemoval adds a node removal operation.
// A node on a line of its own is removed with the whole line,
// including its indentation and its newline (LF or CRLF).
// A node that shares its line with other code keeps the line break,
// for example when removing a trailing comment.
func (repls *NodeReplacements) AddRemoval(node ast.Node, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        node,
		Replacement: nil,
		DebugID:     strings.Join(debugID, ""),
	})
}

// Add appends all replacements from another NodeReplacements collection.
func (repls *NodeReplacements) Add(other NodeReplacements) {
	*repls = append(*repls, other...)
}

// sort sorts the replacements by source position and then by end position,
// so that a zero-width insertion comes before a replacement starting at the same position.
// The sort is stable, so replacements with the same positions keep the order they were added in.
func (repls NodeReplacements) sort() {
	slices.SortStableFunc(repls, func(a, b NodeReplacement) int {
		return cmp.Or(
			cmp.Compare(a.Node.Pos(), b.Node.Pos()),
			cmp.Compare(a.Node.End(), b.Node.End()),
		)
	})
}

// Apply applies all replacements to the source code and returns the modified source.
// Replacements are applied in source order, the order of repls is not changed.
//
// The function intelligently handles whitespace:
//   - Removes the whole line of a deleted node on a line of its own
//   - Keeps the line break after a deleted node that shares its line with other code
//   - Avoids duplicate newlines when replacing
//
// Zero-width insertions like the ones from AddInsertAfter never consume source bytes.
// An error is returned if replacements overlap or if a node is not from source,
// for example a node of another file or a node without a position.
// The end of a /*-style comment is taken from source, because
// its End method doesn't count the carriage returns of CRLF line endings.
func (repls NodeReplacements) Apply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, "", false)
}

// DebugApply applies replacements and outputs diff-style debug information.
// Each replacement is shown with <<<<<<<, =======, and >>>>>>> markers
// on lines of their own, similar to git merge conflicts, to visualize what changed.
// The <<<<<<< marker is followed by the file name, the >>>>>>> marker by the DebugID.
func (repls NodeReplacements) DebugApply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, "", true)
}

// apply implements Apply and DebugApply.
// If filename is not empty, all nodes must be from the file with that name,
// else only the size of their file must match source.
func (repls NodeReplacements) apply(fset *token.FileSet, source []byte, filename string, debug bool) ([]byte, error) {
	for _, repl := range repls {
		node := repl.Node
		if n, ok := node.(insertAfter); ok {
			node = n.Node
		}
		if isNilNode(node) {
			return nil, fmt.Errorf("replacement node %T is nil and not from the source", node)
		}
	}
	// Sort a clone, because repls shares its backing array with the caller
	repls = slices.Clone(repls)
	repls.sort()
	var (
		result    = bytes.NewBuffer(make([]byte, 0, len(source)*2))
		sourcePos = 0      // Source before sourcePos is written or skipped
		prevEnd   = 0      // End of the previous replaced node without a skipped newline
		prevNode  ast.Node // Previous replaced node
	)
	var file *token.File // The file of all replacement nodes
	for _, repl := range repls {
		// The offsets of a node from another file or without a position
		// would point to unrelated bytes of source
		node := repl.Node
		if n, ok := node.(insertAfter); ok {
			// The End of a node without a position can still be valid
			node = n.Node
		}
		nodeFile := fset.File(node.Pos())
		if nodeFile == nil ||
			fset.File(node.End()) != nodeFile || node.End() < node.Pos() || // Range spanning files or synthesized end
			nodeFile.Size() != len(source) ||
			(filename != "" && nodeFile.Name() != filename) ||
			(file != nil && nodeFile != file) {
			return nil, fmt.Errorf("replacement node %T at %s is not from the source", repl.Node, fset.Position(repl.Node.Pos()))
		}
		file = nodeFile
		pos, end := nodeOffsets(fset, repl.Node, source)
		if end < pos || end > len(source) {
			return nil, fmt.Errorf("replacement node at %s is outside of the source", fset.Position(repl.Node.Pos()))
		}
		if pos < prevEnd {
			return nil, fmt.Errorf("replacement node at %s overlaps the replacement node at %s", fset.Position(repl.Node.Pos()), fset.Position(prevNode.Pos()))
		}
		prevEnd, prevNode = end, repl.Node
		// The previous replacement may have skipped a newline at pos
		pos = max(pos, sourcePos)
		end = max(end, pos)
		zeroWidth := pos == end

		gap := source[sourcePos:pos]
		// A removed node on a line of its own is removed with the whole line,
		// a removed node that shares its line with other code keeps the line break
		removedLine := false
		if repl.Replacement == nil {
			startsLine := !containsNonSpace(lastLine(gap)) &&
				(bytes.IndexByte(gap, '\n') >= 0 || !containsNonSpace(lastLine(result.Bytes())))
			removedLine = startsLine && newlineLen(source[end:]) > 0
			switch {
			case removedLine && !containsNonSpace(gap) && !containsNonSpace(lastLine(result.Bytes())) && blankLineAfter(source, end):
				// The blank lines before the removed line are not needed
				// because a blank line follows it. If code follows it,
				// they are kept, else a comment before them
				// would become the doc comment of that code
				gap = nil
			case removedLine:
				// Keep the newlines before the line, but not its indentation
				gap = gap[:bytes.LastIndexByte(gap, '\n')+1]
			case !startsLine:
				// Code before the node on its line, remove the space in between
				gap = bytes.TrimRight(gap, " \t")
			}
		}
		_, err := result.Write(gap)
		if err != nil {
			return nil, err
		}
		if debug {
			err := writeDebugMarker(result, "<<<<<<< "+file.Name())
			if err != nil {
				return nil, err
			}
			_, err = result.Write(source[pos:end])
			if err != nil {
				return nil, err
			}
			err = writeDebugMarker(result, "=======")
			if err != nil {
				return nil, err
			}
		}
		var skipNewline bool
		switch r := repl.Replacement.(type) {
		case nil:
			// Remove the newline after a node on a line of its own
			// so that all node lines get removed
			skipNewline = removedLine
		case string:
			_, err := result.WriteString(r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			skipNewline = strings.HasSuffix(r, "\n")
		case []byte:
			_, err := result.Write(r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			skipNewline = bytes.HasSuffix(r, []byte("\n"))
		default:
			err := format.Node(result, fset, r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			skipNewline = bytes.HasSuffix(result.Bytes(), []byte("\n"))
		}
		// A zero-width insertion never consumes source bytes
		if skipNewline && !zeroWidth {
			end += newlineLen(source[end:])
		}
		sourcePos = end
		if debug {
			err := writeDebugMarker(result, ">>>>>>> "+repl.DebugID)
			if err != nil {
				return nil, err
			}
		}
	}
	// Write rest of source after replacements
	_, err := result.Write(source[sourcePos:])
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// writeDebugMarker writes marker as a line of its own to result
// like the markers of a git merge conflict.
func writeDebugMarker(result *bytes.Buffer, marker string) error {
	if result.Len() > 0 && !bytes.HasSuffix(result.Bytes(), []byte("\n")) {
		err := result.WriteByte('\n')
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(result, marker)
	return err
}

// nodeOffsets returns the offsets of the start and end of node in source.
// go/scanner removes the carriage returns from the text of comments,
// so the End of a /*-style comment with CRLF line endings is before
// the end of the comment in source, which is used instead.
func nodeOffsets(fset *token.FileSet, node ast.Node, source []byte) (pos, end int) {
	pos = fset.Position(node.Pos()).Offset
	end = fset.Position(node.End()).Offset
	switch n := node.(type) {
	case insertAfter:
		_, end = nodeOffsets(fset, n.Node, source)
		return end, end
	case *ast.Comment:
		if strings.HasPrefix(n.Text, "/*") && pos+2 <= len(source) {
			if i := bytes.Index(source[pos+2:], []byte("*/")); i >= 0 {
				end = pos + 2 + i + 2
			}
		}
	case *ast.CommentGroup:
		if len(n.List) > 0 {
			_, end = nodeOffsets(fset, n.List[len(n.List)-1], source)
		}
	case NodeRange:
		for _, child := range n {
			_, childEnd := nodeOffsets(fset, child, source)
			end = max(end, childEnd)
		}
	}
	return pos, end
}

// isNilNode reports if node is nil or a nil pointer,
// which would panic in its Pos method.
func isNilNode(node ast.Node) bool {
	if node == nil {
		return true
	}
	v := reflect.ValueOf(node)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// blankLineAfter reports if the line after the node ending at end in source
// is a blank line or the end of source.
func blankLineAfter(source []byte, end int) bool {
	next := source[end+newlineLen(source[end:]):]
	return len(next) == 0 || newlineLen(next) > 0
}

// newlineLen returns the length of the LF or CRLF newline
// at the beginning of b or zero if b doesn't start with a newline.
func newlineLen(b []byte) int {
	switch {
	case bytes.HasPrefix(b, []byte("\n")):
		return 1
	case bytes.HasPrefix(b, []byte("\r\n")):
		return 2
	}
	return 0
}

// lastLine returns the bytes of b after the last newline.
func lastLine(b []byte) []byte {
	return b[bytes.LastIndexByte(b, '\n')+1:]
}

func containsNonSpace(b []byte) bool {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return true
	}
	return false
}
