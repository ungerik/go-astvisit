package astvisit

import (
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

type Cursor interface {
	// Path returns the current path of the cursor
	Path() Path

	// Node returns the current Node.
	Node() ast.Node

	// Parent returns the parent of the current Node.
	Parent() ast.Node

	// ParentField returns the name of the parent Node field that contains the current Node.
	// If the parent is a *ast.Package and the current Node is a *ast.File, ParentField returns
	// the filename for the current Node.
	ParentField() string

	// ParentFieldIndex reports the index >= 0 of the current Node in the slice of Nodes that
	// contains it, or a value < 0 if the current Node is not part of a slice.
	// The index of the current node changes if InsertBefore is called while
	// processing the current node.
	ParentFieldIndex() int

	// Replace replaces the current Node with n.
	// The replacement node is not walked by Apply.
	Replace(n ast.Node)

	// Delete deletes the current Node from its containing slice.
	// If the current Node is not part of a slice, Delete panics.
	// As a special case, if the current node is a package file,
	// Delete removes it from the package's Files map.
	Delete()

	// InsertAfter inserts n after the current Node in its containing slice.
	// If the current Node is not part of a slice, InsertAfter panics.
	// Apply does not walk n.
	InsertAfter(n ast.Node)

	// InsertBefore inserts n before the current Node in its containing slice.
	// If the current Node is not part of a slice, InsertBefore panics.
	// Apply will not walk n.
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
