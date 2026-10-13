package astvisit

import (
	"go/ast"
	"slices"

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
	// The returned Path is a copy that is not changed by the further traversal.
	// Copying allocates, so avoid calling Path for every node of a large AST
	// when ParentField, ParentFieldIndex, or Parent is enough.
	Path() Path

	// Node returns the current node being visited.
	Node() ast.Node

	// Parent returns the parent node of the current node, or nil if
	// the current node is the root.
	Parent() ast.Node

	// ParentField returns the name of the parent node's struct field
	// that contains the current node, or an empty string if
	// the current node is the root.
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

func newCursor(c *astutil.Cursor, path Path, root *ast.Node) Cursor {
	return &cursor{c, path, root}
}

// cursor implements the Cursor inteface by
// wrapping an astutil.Cursor together with a Path
// and the root node result of Visit.
type cursor struct {
	cursor *astutil.Cursor
	path   Path
	root   *ast.Node
}

// isRoot reports if the current node is the root node passed to Visit.
// astutil.Apply wraps the root in an internal parent node
// that must neither be returned nor modified via the astutil.Cursor.
func (c *cursor) isRoot() bool {
	return len(c.path) == 1
}

func (c *cursor) Path() Path {
	// Clone because Visit reuses the backing array of c.path
	return slices.Clone(c.path)
}

func (c *cursor) Node() ast.Node {
	return c.cursor.Node()
}

func (c *cursor) Parent() ast.Node {
	if c.isRoot() {
		return nil
	}
	return c.cursor.Parent()
}

func (c *cursor) ParentField() string {
	if c.isRoot() {
		return ""
	}
	return c.cursor.Name()
}

func (c *cursor) ParentFieldIndex() int {
	return c.cursor.Index()
}

func (c *cursor) Replace(n ast.Node) {
	if c.isRoot() {
		// Visit returns *c.root instead of the result of astutil.Apply,
		// so every root replacement must be stored there.
		// astutil.Cursor.Replace would also panic for an *ast.File root
		// because it expects an *ast.Package as parent of every file
		*c.root = n
		return
	}
	c.cursor.Replace(n)
}

func (c *cursor) Delete() {
	if c.isRoot() {
		// Same panic as astutil.Cursor.Delete for other nodes that are not
		// part of a slice, it would fail with a type assertion for an *ast.File root
		panic("Delete node not contained in slice")
	}
	c.cursor.Delete()
}

func (c *cursor) InsertAfter(n ast.Node) {
	c.cursor.InsertAfter(n)
}

func (c *cursor) InsertBefore(n ast.Node) {
	c.cursor.InsertBefore(n)
}
