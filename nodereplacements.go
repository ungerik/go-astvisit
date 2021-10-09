package astvisit

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"sort"
)

type NodeReplacements []struct {
	Node        ast.Node
	Replacement interface{} // nil, string, []byte, or anything accepted by format.Node
}

func (repls *NodeReplacements) AddReplacement(node ast.Node, replacement interface{}) {
	*repls = append(*repls, struct {
		Node        ast.Node
		Replacement interface{}
	}{
		Node:        node,
		Replacement: replacement,
	})
}

func (repls *NodeReplacements) AddRemoval(node ast.Node) {
	*repls = append(*repls, struct {
		Node        ast.Node
		Replacement interface{}
	}{
		Node: node,
	})
}

func (repls *NodeReplacements) Add(other NodeReplacements) {
	*repls = append(*repls, other...)
}

func (repls NodeReplacements) Sort() {
	sort.Slice(repls, func(i, j int) bool { return repls[i].Node.Pos() < repls[j].Node.Pos() })
}

func (repls NodeReplacements) Apply(fset *token.FileSet, source []byte) ([]byte, error) {
	repls.Sort()
	var (
		err       error
		buf       bytes.Buffer
		sourcePos = 0
	)
	for _, repl := range repls {
		pos := fset.Position(repl.Node.Pos()).Offset
		end := fset.Position(repl.Node.End()).Offset
		gap := source[sourcePos:pos]
		if repl.Replacement != nil || containsNonSpace(gap) {
			_, err = buf.Write(gap)
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
			_, err = buf.WriteString(r)
			// Skip newline after replaced if replacement also ands with a newline
			if len(r) > 0 && r[len(r)-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		case []byte:
			_, err = buf.Write(r)
			// Skip newline after replaced if replacement also ands with a newline
			if len(r) > 0 && r[len(r)-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		default:
			err = format.Node(&buf, fset, r)
			// Skip newline after replaced if replacement also ands with a newline
			if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' && end < len(source) && source[end] == '\n' {
				end++
			}
		}
		if err != nil {
			return nil, err
		}
		sourcePos = end
	}
	_, err = buf.Write(source[sourcePos:])
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
