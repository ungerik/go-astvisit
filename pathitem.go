package astvisit

import (
	"go/ast"
	"strconv"
	"strings"
)

type PathItem struct {
	Name  string
	Index int
	Node  ast.Node
	Type  string
}

func (item *PathItem) writeTo(b *strings.Builder) {
	if item.Name != "" {
		b.WriteByte('.')
		b.WriteString(item.Name)
		if item.Index >= 0 {
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(item.Index))
			b.WriteByte(']')
		}
	}
	b.WriteByte('/')
	b.WriteString(item.Type)
}

func (item *PathItem) String() string {
	b := strings.Builder{}
	item.writeTo(&b)
	return b.String()
}
