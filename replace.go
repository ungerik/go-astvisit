package astvisit

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"sort"
)

type NodeReplacement struct {
	Node        ast.Node
	Replacement interface{} // nil, string, []byte, or anything accepted by format.Node
}

func SortNodeReplacements(replacements []NodeReplacement) {
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].Node.Pos() < replacements[j].Node.Pos() })
}

func ReplaceNodes(fset *token.FileSet, source []byte, replacements []NodeReplacement) ([]byte, error) {
	SortNodeReplacements(replacements)
	var (
		err       error
		buf       bytes.Buffer
		sourcePos = 0
	)
	for _, nr := range replacements {
		pos := fset.Position(nr.Node.Pos()).Offset
		end := fset.Position(nr.Node.End()).Offset
		gap := source[sourcePos:pos]
		if nr.Replacement != nil || containsNonSpace(gap) {
			_, err = buf.Write(gap)
			if err != nil {
				return nil, err
			}
		}
		switch r := nr.Replacement.(type) {
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
