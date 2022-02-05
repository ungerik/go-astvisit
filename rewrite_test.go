package astvisit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
)

func ExampleRewrite() {
	// Use verboseWriter to print encountered file paths
	err := Rewrite(
		"./...",
		os.Stdout, // verboseWriter
		nil,       // printOnly
		func(fset *token.FileSet, filePkg *ast.Package, astFile *ast.File, filePath string, verboseWriter, printOnly io.Writer) error {
			info, err := os.Stat(filePath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return errors.New("listed directory")
			}
			if filepath.Ext(filePath) != ".go" {
				return errors.New("listed non .go file")
			}
			_, err = fmt.Fprintln(verboseWriter, filePath)
			return err
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
