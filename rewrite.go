package astvisit

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RewriteAstFileFunc func(fset *token.FileSet, filePkg *ast.Package, astFile *ast.File, filePath string, verboseWriter, printOnly io.Writer) error

func Rewrite(path string, verboseWriter, printOnly io.Writer, rewriteFunc RewriteAstFileFunc) error {
	recursive := strings.HasSuffix(path, "...")
	if recursive {
		path = filepath.Clean(strings.TrimSuffix(path, "..."))
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()

	// Rewrite single file
	if !fileInfo.IsDir() {
		pkg, err := ParsePackage(fset, filepath.Dir(path), filterOutTests)
		if err != nil {
			return err
		}
		return rewriteFunc(fset, pkg, pkg.Files[path], path, verboseWriter, printOnly)
	}

	// Rewrite directory
	pkg, err := ParsePackage(fset, path, filterOutTests)
	if err != nil && (!recursive || !errors.Is(err, ErrPackageNotFound)) {
		return err
	}
	if err == nil {
		fileNames := make([]string, 0, len(pkg.Files))
		for fileName := range pkg.Files {
			fileNames = append(fileNames, fileName)
		}
		sort.Strings(fileNames)
		for _, fileName := range fileNames {
			err = rewriteFunc(fset, pkg, pkg.Files[fileName], fileName, verboseWriter, printOnly)
			if err != nil {
				return err
			}
		}
	} else if verboseWriter != nil {
		fmt.Fprintln(verboseWriter, err)
	}
	if !recursive {
		return nil
	}

	// Rewrite recursive sub-directories
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range files {
		fileName := file.Name()
		if !file.IsDir() || fileName[0] == '.' || fileName == "node_modules" {
			continue
		}
		err = Rewrite(filepath.Join(path, fileName, "..."), verboseWriter, printOnly, rewriteFunc)
		if err != nil {
			return err
		}
	}
	return nil
}

func filterOutTests(info os.FileInfo) bool {
	if strings.HasSuffix(info.Name(), "_test.go") {
		return false
	}
	return true
}
