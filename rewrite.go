package astvisit

import (
	"bytes"
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

// FileReplacementsFunc is called for a source file by RewriteWithReplacements
// to return NodeReplacements and Imports that will be applied to the source file.
type FileReplacementsFunc func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error)

func RewriteWithReplacements(path string, verboseOut, resultOut io.Writer, debug bool, fileReplacementsFunc FileReplacementsFunc) error {
	if fileReplacementsFunc == nil {
		return errors.New("nil fileReplacementsFunc")
	}
	return Rewrite(path, verboseOut, resultOut, func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
		replacements, imports, err := fileReplacementsFunc(fset, pkg, astFile, filePath, verboseOut)
		if err != nil {
			return nil, err
		}
		if len(replacements) == 0 {
			return nil, nil
		}

		source, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		if debug {
			return replacements.DebugApply(fset, source)
		}

		rewritten, err := replacements.Apply(fset, source)
		if err != nil {
			return nil, err
		}
		rewritten, err = FormatFileWithImports(fset, rewritten, imports)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(source, rewritten) {
			return nil, nil
		}
		return rewritten, nil
	})
}

// RewriteFileFunc is called for a source file by Rewrite to return
// the rewritten source or nil if the file should not be changed.
type RewriteFileFunc func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error)

func Rewrite(path string, verboseOut, resultOut io.Writer, rewriteFileFunc RewriteFileFunc) error {
	if rewriteFileFunc == nil {
		return errors.New("nil rewriteFileFunc")
	}
	recursive := strings.HasSuffix(path, "...")
	if recursive {
		path = strings.TrimSuffix(path, "...")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	pathInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()

	// Rewrite single file
	if !pathInfo.IsDir() {
		pkg, err := ParsePackage(fset, filepath.Dir(path), filterOutTests)
		if err != nil {
			return err
		}
		return rewriteFile(fset, pkg, path, verboseOut, resultOut, rewriteFileFunc)
	}

	// Rewrite directory
	pkg, err := ParsePackage(fset, path, filterOutTests)
	if err != nil && (!recursive || !errors.Is(err, ErrPackageNotFound)) {
		return err
	}
	if err == nil {
		filePaths := make([]string, 0, len(pkg.Files))
		for filePath := range pkg.Files {
			filePaths = append(filePaths, filePath)
		}
		sort.Strings(filePaths)
		for _, filePath := range filePaths {
			err = rewriteFile(fset, pkg, filePath, verboseOut, resultOut, rewriteFileFunc)
			if err != nil {
				return err
			}
		}
	} else {
		err = FprintfVerbose(verboseOut, "%s\n", err)
		if err != nil {
			return err
		}
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
		err = Rewrite(filepath.Join(path, fileName, "..."), verboseOut, resultOut, rewriteFileFunc)
		if err != nil {
			return err
		}
	}
	return nil
}

func rewriteFile(fset *token.FileSet, pkg *ast.Package, filePath string, verboseOut, resultOut io.Writer, rewriteFileFunc RewriteFileFunc) error {
	filePath = filepath.Clean(filePath)
	astFile, ok := pkg.Files[filePath]
	if !ok {
		return fmt.Errorf("package %s has no file %s", pkg.Name, filepath.Base(filePath))
	}
	err := FprintfVerbose(verboseOut, "parsing file: %s\n", filePath)
	if err != nil {
		return err
	}
	rewritten, err := rewriteFileFunc(fset, pkg, astFile, filePath, verboseOut)
	if err != nil {
		return err
	}
	if rewritten == nil {
		return FprintfVerbose(verboseOut, "no changes in file: %s\n", filePath)
	}
	err = FprintfVerbose(verboseOut, "changed file: %s\n", filePath)
	if err != nil {
		return err
	}
	if resultOut != nil {
		_, err = resultOut.Write(rewritten)
		return err
	}
	return os.WriteFile(filePath, rewritten, 0600)
}

func filterOutTests(info os.FileInfo) bool {
	if strings.HasSuffix(info.Name(), "_test.go") {
		return false
	}
	return true
}

// FprintfVerbose calls fmt.Fprintf if verboseOut is not nil
func FprintfVerbose(verboseOut io.Writer, format string, args ...any) error {
	if verboseOut == nil {
		return nil
	}
	_, err := fmt.Fprintf(verboseOut, format, args...)
	return err
}
