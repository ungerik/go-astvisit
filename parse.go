package astvisit

import (
	"strings"

	"github.com/ungerik/go-fs"
)

// EnumPackageGoFiles reads all Go files of a package and returns their name and content via callback.
func EnumPackageGoFiles(packageDir string, callback func(fileName string, fileContent []byte) error) error {
	return fs.File(packageDir).ListDir(func(file fs.File) error {
		if file.IsDir() || file.Ext() != ".go" || strings.HasSuffix(file.Name(), "_test.go") {
			return nil
		}
		fileContent, err := file.ReadAll()
		if err != nil {
			return err
		}
		return callback(file.Name(), fileContent)
	})
}
