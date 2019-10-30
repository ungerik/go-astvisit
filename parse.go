package astvisit

import (
	"go/token"
	"strings"

	"github.com/ungerik/go-fs"
)

// EnumPackageGoFiles enums the path of all non test .go files of a package via callback.
func EnumPackageGoFiles(pkgDir string, fset *token.FileSet, callback func(fset *token.FileSet, filePath string) error) error {
	return fs.File(pkgDir).ListDir(func(file fs.File) error {
		if file.IsDir() || file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
			return nil
		}
		return callback(fset, file.LocalPath())
	})
}

// EnumPackageGoFilesRecursive enums the path of all non test .go files of a package
// and all sub-packages (basePkgDir/...) via callback.
// For every package new token.FileSet is created and passed to the callback.
func EnumPackageGoFilesRecursive(basePkgDir string, callback func(fset *token.FileSet, filePath string) error) error {
	fset := token.NewFileSet()
	return fs.File(basePkgDir).ListDir(func(file fs.File) error {
		if file.IsDir() {
			return EnumPackageGoFilesRecursive(file.LocalPath(), callback)
		}

		if file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
			return nil
		}
		return callback(fset, file.LocalPath())
	})
}
