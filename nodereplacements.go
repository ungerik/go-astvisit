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

// PosNode implements ast.Node by returning
// the same underlying token.Pos
// from its Pos() and End() methods.
// Usable as zero size replacement node
// to mark the position of a code insertion.
type PosNode token.Pos

func (p PosNode) Pos() token.Pos { return token.Pos(p) }
func (p PosNode) End() token.Pos { return token.Pos(p) }

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

type NodeReplacement struct {
	Node        ast.Node
	Replacement interface{} // nil, string, []byte, or anything accepted by format.Node
	DebugID     string
}

type NodeReplacements []NodeReplacement

func (repls *NodeReplacements) AddReplacement(node ast.Node, replacement interface{}, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        node,
		Replacement: replacement,
		DebugID:     strings.Join(debugID, ""),
	})
}

func (repls *NodeReplacements) AddInsertAfter(node ast.Node, insertion interface{}, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        PosNode(node.End()),
		Replacement: insertion,
		DebugID:     strings.Join(debugID, ""),
	})
}

func (repls *NodeReplacements) AddRemoval(node ast.Node, debugID ...string) {
	*repls = append(*repls, NodeReplacement{
		Node:        node,
		Replacement: nil,
		DebugID:     strings.Join(debugID, ""),
	})
}

func (repls *NodeReplacements) Add(other NodeReplacements) {
	*repls = append(*repls, other...)
}

func (repls NodeReplacements) Sort() {
	sort.Slice(repls, func(i, j int) bool { return repls[i].Node.Pos() < repls[j].Node.Pos() })
}

func (repls NodeReplacements) Apply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, false)
}

func (repls NodeReplacements) DebugApply(fset *token.FileSet, source []byte) ([]byte, error) {
	return repls.apply(fset, source, true)
}

func (repls NodeReplacements) apply(fset *token.FileSet, source []byte, debug bool) ([]byte, error) {
	repls.Sort()
	var (
		result    = bytes.NewBuffer(make([]byte, 0, len(source)))
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
