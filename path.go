package astvisit

import (
	"go/ast"
	"strings"
)

type Path []PathItem

func (s *Path) push(parentField string, parentFieldIndex int, node ast.Node) {
	item := PathItem{
		Node: node,
		Type: NodeType(node),
	}

	if len(*s) > 0 {
		item.ParentField = parentField
		item.ParentFieldIndex = parentFieldIndex
	}

	*s = append(*s, item)
}

func (s *Path) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s Path) Last() PathItem {
	if len(s) == 0 {
		return PathItem{}
	}
	return s[len(s)-1]
}

func (s Path) String() string {
	b := strings.Builder{}
	for i := range s {
		s[i].writeTo(&b)
	}
	return b.String()
}
