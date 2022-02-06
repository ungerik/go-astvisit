package astvisit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExampleRewrite() {
	cwd, _ := os.Getwd()
	cwd += string(os.PathSeparator)
	// Use verboseWriter to print encountered file paths
	err := Rewrite(
		"./...",
		nil, // verboseOut
		nil, // resultOut
		func(fset *token.FileSet, filePkg *ast.Package, astFile *ast.File, filePath string, verboseWriter io.Writer) ([]byte, error) {
			info, err := os.Stat(filePath)
			if err != nil {
				return nil, err
			}
			if info.IsDir() {
				return nil, errors.New("listed directory")
			}
			if filepath.Ext(filePath) != ".go" {
				return nil, errors.New("listed non .go file")
			}
			_, err = fmt.Println(strings.TrimPrefix(filePath, cwd))
			return nil, err
		})
	if err != nil {
		panic(err)
	}

	// Output:
	// cursor.go
	// errors.go
	// nodereplacements.go
	// nodetype.go
	// packagelocation.go
	// parse.go
	// path.go
	// pathitem.go
	// rewrite.go
	// tostring.go
	// visit.go
	// visitor.go
	// visitorimpl.go
	// test/dummy.go
}
