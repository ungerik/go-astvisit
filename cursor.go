package astvisit

import (
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

// Cursor provides context and navigation information about the current position
// during AST traversal. It allows inspection of the node's location in the tree
// and supports modifications like replacing, deleting, or inserting nodes.
//
// The Cursor is passed to each visitor method and provides access to:
//   - The current node and its parent
//   - The full path from root to current node
//   - The field name and index in the parent structure
//   - Methods to modify the AST during traversal
//
// Example usage:
//
//	func (v *myVisitor) VisitIdent(node *ast.Ident, cursor Cursor) bool {
//	    // Get context
//	    parent := cursor.Parent()
//	    path := cursor.Path()
//	    field := cursor.ParentField()
//
//	    // Modify AST
//	    if node.Name == "old" {
//	        cursor.Replace(&ast.Ident{Name: "new"})
//	    }
//	    return true
//	}
type Cursor interface {
	// Path returns the full path from the root node to the current node,
	// including parent field names and slice indices at each level.
	Path() Path

	// Node returns the current node being visited.
	Node() ast.Node

	// Parent returns the parent node of the current node, or nil if
	// the current node is the root.
	Parent() ast.Node

	// ParentField returns the name of the parent node's struct field
	// that contains the current node.
	//
	// Special case: If the parent is *ast.Package and the current node is
	// *ast.File, ParentField returns the filename.
	ParentField() string

	// ParentFieldIndex returns the index of the current node within its
	// parent's slice field, or a value < 0 if the current node is not
	// part of a slice.
	//
	// Note: The index may change if InsertBefore is called on the current node.
	ParentFieldIndex() int

	// Replace replaces the current node with the given node n.
	// The replacement node will not be visited during the current traversal.
	//
	// Example:
	//	cursor.Replace(&ast.Ident{Name: "newName"})
	Replace(n ast.Node)

	// Delete removes the current node from its containing slice.
	//
	// Panics if the current node is not part of a slice.
	//
	// Special case: If the current node is a file in *ast.Package,
	// Delete removes it from the package's Files map.
	Delete()

	// InsertAfter inserts the given node n immediately after the current
	// node in its containing slice.
	//
	// The inserted node will not be visited during the current traversal.
	//
	// Panics if the current node is not part of a slice.
	InsertAfter(n ast.Node)

	// InsertBefore inserts the given node n immediately before the current
	// node in its containing slice.
	//
	// The inserted node will not be visited during the current traversal.
	// This changes the ParentFieldIndex of the current node.
	//
	// Panics if the current node is not part of a slice.
	InsertBefore(n ast.Node)
}

func newCursor(c *astutil.Cursor, path Path) Cursor {
	return &cursor{c, path}
}

// cursor implements the Cursor inteface by
// wrapping an astutil.Cursor together with a Path.
type cursor struct {
	cursor *astutil.Cursor
	path   Path
}

func (c *cursor) Path() Path {
	return c.path
}

func (c *cursor) Node() ast.Node {
	return c.cursor.Node()
}

func (c *cursor) Parent() ast.Node {
	return c.cursor.Parent()
}

func (c *cursor) ParentField() string {
	return c.cursor.Name()
}

func (c *cursor) ParentFieldIndex() int {
	return c.cursor.Index()
}

func (c *cursor) Replace(n ast.Node) {
	c.cursor.Replace(n)
}

func (c *cursor) Delete() {
	c.cursor.Delete()
}

func (c *cursor) InsertAfter(n ast.Node) {
	c.cursor.InsertAfter(n)
}

func (c *cursor) InsertBefore(n ast.Node) {
	c.cursor.InsertBefore(n)
}
