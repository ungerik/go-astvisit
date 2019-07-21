package astvisit

import (
	"go/ast"
	"strings"
)

type Path []PathItem

func (s *Path) push(name string, index int, node ast.Node) {
	if len(*s) == 0 {
		*s = append(*s,
			PathItem{
				Node: node,
				Type: NodeType(node),
			},
		)
		return
	}

	*s = append(*s,
		PathItem{
			Name:  name,
			Index: index,
			Node:  node,
			Type:  NodeType(node),
		},
	)
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
