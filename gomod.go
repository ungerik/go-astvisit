package astvisit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// goModule is a Go module with its root directory
// and the paths of the ignore directives of its go.mod file.
type goModule struct {
	dir    string
	ignore []string
}

// findModule returns the module that contains dir,
// or nil if neither dir nor any of its parents has a go.mod file.
func findModule(dir string) (*goModule, error) {
	for {
		mod, err := readModule(dir)
		if mod != nil || err != nil {
			return mod, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

// hasGoMod reports if dir contains a go.mod file.
// Like the go command, it doesn't parse the file
// because that's not necessary to skip a nested module.
func hasGoMod(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil && !info.IsDir()
}

// readModule returns the module with the root directory dir,
// or nil if dir has no go.mod file.
func readModule(dir string) (*goModule, error) {
	goModPath := filepath.Join(dir, "go.mod")
	// #nosec G304 - goModPath is the go.mod file of a directory to rewrite
	data, err := os.ReadFile(goModPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// ParseLax ignores directives newer than golang.org/x/mod,
	// but still parses the ignore directives
	modFile, err := modfile.ParseLax(goModPath, data, nil)
	if err != nil {
		return nil, err
	}
	mod := &goModule{dir: dir}
	for _, ignore := range modFile.Ignore {
		mod.ignore = append(mod.ignore, ignore.Path)
	}
	return mod, nil
}

// ignores reports if the go command ignores the directory dir of the module
// for ./... patterns because of an ignore directive of the go.mod file.
// An ignore path starting with "./" ignores the path relative
// to the module root, any other path ignores directories
// with that path at any depth within the module.
func (mod *goModule) ignores(dir string) bool {
	return slices.ContainsFunc(mod.ignore, func(ignore string) bool {
		return mod.ignoreMatches(ignore, dir)
	})
}

// ignoreMatches reports if the path of an ignore directive matches dir.
func (mod *goModule) ignoreMatches(ignore, dir string) bool {
	rel, err := filepath.Rel(mod.dir, dir)
	if err != nil {
		return false
	}
	rel = normalizeIgnorePath(rel)
	ignorePath, rootRelative := strings.CutPrefix(ignore, "./")
	ignorePath = normalizeIgnorePath(ignorePath)
	if rootRelative {
		return strings.HasPrefix(rel, ignorePath)
	}
	return strings.Contains(rel, ignorePath)
}

// withoutIgnoresOf returns the module without the ignore directives that match dir.
// Naming an ignored directory rewrites it on purpose, including its sub-directories.
func (mod *goModule) withoutIgnoresOf(dir string) *goModule {
	return &goModule{
		dir: mod.dir,
		ignore: slices.DeleteFunc(slices.Clone(mod.ignore), func(ignore string) bool {
			return mod.ignoreMatches(ignore, dir)
		}),
	}
}

// normalizeIgnorePath returns path with slash separators
// and a slash at the beginning and the end,
// like the go command does to match ignore directives.
func normalizeIgnorePath(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}
