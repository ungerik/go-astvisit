package astvisit

import (
	"go/ast"
	"strconv"
	"strings"
)

type PathItem struct {
	ParentField      string
	ParentFieldIndex int

	Node ast.Node
	Type string
}

func (item *PathItem) writeTo(b *strings.Builder) {
	if item.ParentField != "" {
		b.WriteByte('.')
		b.WriteString(item.ParentField)
		if item.ParentFieldIndex >= 0 {
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(item.ParentFieldIndex))
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
