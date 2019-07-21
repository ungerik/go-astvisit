package astvisit

import "go/ast"

type Cursor interface {
	Path() Path

	// Node returns the current Node.
	Node() ast.Node

	// Parent returns the parent of the current Node.
	Parent() ast.Node

	// Name returns the name of the parent Node field that contains the current Node.
	// If the parent is a *ast.Package and the current Node is a *ast.File, Name returns
	// the filename for the current Node.
	Name() string

	// Index reports the index >= 0 of the current Node in the slice of Nodes that
	// contains it, or a value < 0 if the current Node is not part of a slice.
	// The index of the current node changes if InsertBefore is called while
	// processing the current node.
	Index() int

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
