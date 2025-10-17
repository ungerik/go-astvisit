package astvisit

import (
	"go/ast"
	"strings"
)

// Path represents the traversal path from the root AST node to the current node.
// It is a slice of PathItem entries, each representing a step in the tree hierarchy.
//
// The path tracks:
//   - The parent field name at each level
//   - The index within a slice (if applicable)
//   - The node at each level
//   - The node type name
//
// Example path string representation:
//
//	/File.Decls[0]/FuncDecl.Body/BlockStmt.List[2]/ReturnStmt
//
// This shows navigation from File → first declaration (FuncDecl) → Body → third statement (ReturnStmt)
//
// Path is accessible via Cursor.Path() during AST traversal.
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

// Last returns the last item in the path (the current node).
// Returns an empty PathItem if the path is empty.
func (s Path) Last() PathItem {
	if len(s) == 0 {
		return PathItem{}
	}
	return s[len(s)-1]
}

// String returns a human-readable representation of the path.
// The format shows the traversal from root to current node:
//
//	/File.Decls[0]/FuncDecl.Body/BlockStmt.List[2]/ReturnStmt
//
// This represents: File → Decls[0] → Body → List[2] → ReturnStmt
func (s Path) String() string {
	b := strings.Builder{}
	for _, item := range s {
		item.writeTo(&b)
	}
	return b.String()
}
