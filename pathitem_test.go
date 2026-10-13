package astvisit

import (
	"fmt"
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

// Path holds PathItem values and Path.Last returns one,
// so printing a PathItem value with fmt must use the path syntax
// and not the struct fields, which are useless in logs and error messages.
func TestPathItem_String_Value(t *testing.T) {
	path := Path{
		{ParentFieldIndex: -1, Node: &ast.File{}, Type: "File"},
		{ParentField: "Decls", ParentFieldIndex: 0, Node: &ast.FuncDecl{}, Type: "FuncDecl"},
		{ParentField: "Body", ParentFieldIndex: -1, Node: &ast.BlockStmt{}, Type: "BlockStmt"},
	}
	require.Equal(t, ".Body/BlockStmt", fmt.Sprint(path.Last()))
	require.Equal(t, ".Decls[0]/FuncDecl", fmt.Sprintf("%v", path[1]))
	require.Equal(t, "/File.Decls[0]/FuncDecl.Body/BlockStmt", path.String())
}
