package astvisit

import (
	"go/ast"
	"strconv"
	"strings"
)

// PathItem represents a single step in an AST traversal path.
// It captures the relationship between a node and its parent.
//
// Example PathItem values:
//   - ParentField: "Body", ParentFieldIndex: -1, Node: *ast.BlockStmt, Type: "BlockStmt"
//     Represents a single (non-slice) field named "Body" containing a BlockStmt
//
//   - ParentField: "List", ParentFieldIndex: 2, Node: *ast.ReturnStmt, Type: "ReturnStmt"
//     Represents the third element (index 2) in a "List" slice field containing a ReturnStmt
//
// PathItem is part of a Path that tracks the complete traversal from root to current node.
type PathItem struct {
	// ParentField is the name of the parent node's struct field that contains this node.
	// Empty for the root node.
	ParentField string

	// ParentFieldIndex is the index within a slice field, or -1 if not in a slice.
	ParentFieldIndex int

	// Node is the AST node at this path position.
	Node ast.Node

	// Type is the string representation of the node's type (e.g., "FuncDecl", "Ident").
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

// String returns a string representation of this path item.
// Format examples:
//   - ".Body/BlockStmt" (single field)
//   - ".List[2]/ReturnStmt" (slice element at index 2)
func (item *PathItem) String() string {
	b := strings.Builder{}
	item.writeTo(&b)
	return b.String()
}
