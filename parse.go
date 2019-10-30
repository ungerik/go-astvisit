package astvisit

import (
	"strings"

	"github.com/ungerik/go-fs"
)

// EnumPackageGoFiles enums the path of all Go files of a package via callback.
func EnumPackageGoFiles(packageDir string, callback func(filePath string) error) error {
	return fs.File(packageDir).ListDir(func(file fs.File) error {
		if file.IsDir() || file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
			return nil
		}
		return callback(file.LocalPath())
	})
}
