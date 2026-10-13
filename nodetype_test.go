package astvisit

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

// NodeType is called for every node of a Path during Visit and for debug output,
// so it must name every ast.Node instead of panicking: the node types of this package
// and go/ast node types without a case, like *ast.Directive that was added in Go 1.26.
// Names of go/ast types must look like the ones of the known go/ast types.
func TestNodeType(t *testing.T) {
	require.Equal(t, "FuncDecl", NodeType(&ast.FuncDecl{}))
	require.Equal(t, "<nil>", NodeType(nil))
	require.Equal(t, "Directive", NodeType(&ast.Directive{}))
	require.Equal(t, "astvisit.PosNode", NodeType(PosNode(1)))
	require.Equal(t, "astvisit.NodeRange", NodeType(NodeRange{}))
}
