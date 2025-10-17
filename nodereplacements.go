package astvisit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"math"
	"sort"
	"strings"
)

// PosNode implements ast.Node by returning the same underlying token.Pos
// from both its Pos() and End() methods. This creates a zero-width node that
// can be used as a marker for code insertions at a specific position.
//
// Example:
//
//	// Create an insertion point after a node
//	insertPos := PosNode(node.End())
//	replacements.AddReplacement(insertPos, "// New comment\n")
type PosNode token.Pos

func (p PosNode) Pos() token.Pos { return token.Pos(p) }
func (p PosNode) End() token.Pos { return token.Pos(p) }

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
		Node:        PosNode(node.End()),
		Replacement: insertion,
		DebugID:     strings.Join(debugID, ""),
	})
}

// AddRemoval adds a node removal operation.
// The node and its trailing newline (if any) will be removed from the source.
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

// Sort sorts the replacements by source position. This is automatically called
// by Apply and DebugApply, so you typically don't need to call it manually.
func (repls NodeReplacements) Sort() {
	sort.Slice(repls, func(i, j int) bool { return repls[i].Node.Pos() < repls[j].Node.Pos() })
}

// Apply applies all replacements to the source code and returns the modified source.
// Replacements are sorted by position before application.
//
// The function intelligently handles whitespace:
//   - Removes trailing newlines after deleted nodes
//   - Avoids duplicate newlines when replacing
//   - Preserves indentation where appropriate
func (repls NodeReplacements) Apply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, false)
}

// DebugApply applies replacements and outputs diff-style debug information.
// Each replacement is shown with <<<<<<, =======, and >>>>>>> markers,
// similar to git merge conflicts, to visualize what changed.
func (repls NodeReplacements) DebugApply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, true)
}

func (repls NodeReplacements) apply(fset *token.FileSet, source []byte, debug bool) ([]byte, error) {
	repls.Sort()
	var (
		result    = bytes.NewBuffer(make([]byte, 0, len(source)*2))
		sourcePos = 0
	)
	for _, repl := range repls {
		pos := fset.Position(repl.Node.Pos()).Offset
		end := fset.Position(repl.Node.End()).Offset
		gap := source[sourcePos:pos]
		if repl.Replacement != nil || containsNonSpace(gap) {
			_, err := result.Write(gap)
			if err != nil {
				return nil, err
			}
		}
		if debug {
			filename := fset.File(repl.Node.Pos()).Name()
			_, err := fmt.Fprintf(result, "<<<<<<< %s\n", filename)
			if err != nil {
				return nil, err
			}
			_, err = result.Write(source[pos:end])
			if err != nil {
				return nil, err
			}
			_, err = fmt.Fprint(result, "\n=======")
			if err != nil {
				return nil, err
			}
		}
		switch r := repl.Replacement.(type) {
		case nil:
			// If a removed node ends with a newline remove that too
			// so that all node lines get removed
			if end < len(source) && source[end] == '\n' {
				end++
			}
		case string:
			_, err := result.WriteString(r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			if len(r) > 0 && r[len(r)-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		case []byte:
			_, err := result.Write(r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			if len(r) > 0 && r[len(r)-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		default:
			err := format.Node(result, fset, r)
			if err != nil {
				return nil, err
			}
			// Skip newline after replaced if replacement also ands with a newline
			if result.Len() > 0 && result.Bytes()[result.Len()-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		}
		sourcePos = end
		if debug {
			_, err := fmt.Fprintf(result, "\n>>>>>>> %s\n", repl.DebugID)
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
