package astvisit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The ignore directives must match the same directories as for the go command,
// else a rewrite of ./... changes code that go build ./... doesn't see
// or misses code that it does.
func TestGoModule_Ignores(t *testing.T) {
	mod := &goModule{
		dir:    filepath.FromSlash("/repo"),
		ignore: []string{"./gen", "fixtures", "web/node_data", "./tools/"},
	}
	for dir, want := range map[string]bool{
		"/repo/gen":                    true,  // "./gen" at the module root
		"/repo/gen/sub":                true,  // and below it
		"/repo/pkg/gen":                false, // but not at any depth
		"/repo/generator":              false, // only whole path elements match
		"/repo/fixtures":               true,  // "fixtures" at any depth
		"/repo/pkg/fixtures/sub":       true,
		"/repo/pkg/myfixtures":         false,
		"/repo/web/node_data":          true, // multi element path at any depth
		"/repo/app/web/node_data/sub":  true,
		"/repo/web/node_data_old":      false,
		"/repo/tools":                  true, // trailing slash in ignore path
		"/repo/pkg":                    false,
		"/repo/app/web/node_data_more": false,
	} {
		require.Equal(t, want, mod.ignores(filepath.FromSlash(dir)), dir)
	}
}

// A walk that starts in a sub-directory of a module
// must find the go.mod file of the module to apply its ignore directives.
func TestFindModule(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod":     "module example.com/m\n\ngo 1.25\n\nignore ./gen\n",
		"sub/a/a.go": "package a\n",
	})

	mod, err := findModule(filepath.Join(dir, "sub", "a"))
	require.NoError(t, err)
	require.NotNil(t, mod)
	require.Equal(t, dir, mod.dir)
	require.Equal(t, []string{"./gen"}, mod.ignore)

	requireOutsideModules(t, dir)
	mod, err = findModule(filepath.Dir(dir))
	require.NoError(t, err)
	require.Nil(t, mod)
}

// requireOutsideModules skips a test that needs dir to be outside of any module,
// which is not the case if TMPDIR is set to a directory within a module.
// The go.mod files are searched independently of findModule,
// so that a bug in findModule can't hide this test precondition.
func requireOutsideModules(t *testing.T, dir string) {
	t.Helper()
	for d := filepath.Dir(dir); ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			t.Skipf("%s is within the module %s, set TMPDIR to a directory outside of Go modules", dir, d)
		}
		if filepath.Dir(d) == d {
			return
		}
	}
}

// A go.mod with a directive newer than golang.org/x/mod must not abort every walk,
// only the ignore directives are needed, like the go command ignores
// directives it doesn't need when a newer toolchain is selected.
func TestReadModule_UnknownDirective(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, map[string]string{
		"go.mod": "module example.com/m\n\nfuturedirective foo\n\nignore ./gen\n",
	})

	mod, err := readModule(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"./gen"}, mod.ignore)
}
