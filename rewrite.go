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
	"slices"
	"strings"
)

// FileReplacementsFunc is called for each source file by RewriteWithReplacements.
// It should return the NodeReplacements and Imports to be applied to the file.
//
// Parameters:
//   - fset: Token file set for position information
//   - pkg: The parsed package containing the file
//   - astFile: The AST of the file being processed
//   - filePath: Absolute path to the file
//   - verboseOut: Writer for verbose output (may be nil)
//
// Returns:
//   - NodeReplacements: Set of node-level replacements to apply
//   - Imports: Set of import statements to ensure are present
//   - error: Any error that occurred during processing
type FileReplacementsFunc func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error)

// PackageFunc returns a RewritePackageFunc that calls f for every file of a package
// and applies the returned replacements and imports like RewriteWithReplacements.
// Use it to call RewritePackages or RewriteDryRun with a FileReplacementsFunc.
func (f FileReplacementsFunc) PackageFunc() RewritePackageFunc {
	return f.packageFunc(false)
}

// packageFunc returns the RewritePackageFunc of PackageFunc,
// or one that returns the output of NodeReplacements.DebugApply
// as rewritten sources if debug is true.
func (f FileReplacementsFunc) packageFunc(debug bool) RewritePackageFunc {
	return func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		return rewriteFiles(pkg, verboseOut, func(fset *token.FileSet, astPkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error) {
			replacements, imports, err := f(fset, astPkg, astFile, filePath, verboseOut)
			if err != nil || len(replacements) == 0 {
				return nil, err
			}
			return applyReplacements(fset, filePath, pkg.Sources[filePath], replacements, imports, debug)
		})
	}
}

// RewriteWithReplacements rewrites Go source files using NodeReplacements.
// This is a higher-level alternative to Rewrite that handles formatting and imports.
//
// Parameters:
//   - path: File or directory path. Append "..." for recursive directory traversal.
//   - verboseOut: Writer for verbose logging (nil to disable)
//   - resultOut: Writer for rewritten source (nil to write back to files)
//   - debug: If true, writes diff-style debug information to resultOut
//     instead of the rewritten source. Files are never changed in debug mode,
//     so debug requires a non-nil resultOut.
//   - fileReplacementsFunc: Function called for each file to compute replacements
//
// The replacements are applied to the source that was parsed, the result is formatted
// with FormatFileWithImports. A file is not changed if the formatted result equals
// the source formatted with the same imports, so replacements that change nothing
// don't reformat a file. See Rewrite for how files are selected and written.
//
// Returns an error if any file fails to process.
//
// Example:
//
//	err := astvisit.RewriteWithReplacements("./...", os.Stdout, nil, false,
//	    func(fset *token.FileSet, pkg *ast.Package, file *ast.File,
//	         filePath string, verboseOut io.Writer) (NodeReplacements, Imports, error) {
//	        var replacements NodeReplacements
//	        imports := make(Imports)
//	        // Compute replacements...
//	        return replacements, imports, nil
//	    })
func RewriteWithReplacements(path string, verboseOut, resultOut io.Writer, debug bool, fileReplacementsFunc FileReplacementsFunc) error {
	if fileReplacementsFunc == nil {
		return errors.New("nil fileReplacementsFunc")
	}
	if debug && resultOut == nil {
		// Debug output is not Go code and must not overwrite source files
		return errors.New("debug needs a non-nil resultOut")
	}
	return RewritePackages(path, verboseOut, resultOut, fileReplacementsFunc.packageFunc(debug))
}

// applyReplacements returns source of filePath with replacements and imports applied
// and formatted, or nil if that only changes the formatting of source.
func applyReplacements(fset *token.FileSet, filePath string, source []byte, replacements NodeReplacements, imports Imports, debug bool) ([]byte, error) {
	if debug {
		return replacements.apply(fset, source, filePath, true)
	}
	rewritten, err := replacements.apply(fset, source, filePath, false)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(source, rewritten) {
		return nil, nil
	}
	rewritten, err = formatFileWithImports(filePath, rewritten, imports)
	if err != nil {
		// An error of the imports would also happen for the parsed source
		// and is not an error of the replacements
		if _, sourceErr := formatFileWithImports(filePath, source, imports); sourceErr != nil {
			return nil, sourceErr
		}
		// The error positions are in the rewritten source, not in the file
		return nil, fmt.Errorf("invalid Go code after applying replacements: %w", err)
	}
	if bytes.Equal(source, rewritten) {
		// Up to date, typical for a check with RewriteDryRun,
		// no need to format the source to compare
		return nil, nil
	}
	formattedSource, err := formatFileWithImports(filePath, source, imports)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(formattedSource, rewritten) {
		return nil, nil
	}
	return rewritten, nil
}

// RewriteFileFunc is called for each source file by Rewrite.
// It should return the rewritten source code, or nil if no changes are needed.
//
// Parameters:
//   - fset: Token file set for position information
//   - pkg: The parsed package containing the file
//   - astFile: The AST of the file being processed
//   - filePath: Absolute path to the file
//   - verboseOut: Writer for verbose output (may be nil)
//
// Returns:
//   - []byte: The rewritten source code, or nil for no changes
//   - error: Any error that occurred during processing
type RewriteFileFunc func(fset *token.FileSet, pkg *ast.Package, astFile *ast.File, filePath string, verboseOut io.Writer) ([]byte, error)

// PackageFunc returns a RewritePackageFunc that calls f for every file of a package.
// Use it to call RewritePackages or RewriteDryRun with a RewriteFileFunc.
func (f RewriteFileFunc) PackageFunc() RewritePackageFunc {
	return func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error) {
		return rewriteFiles(pkg, verboseOut, f)
	}
}

// rewriteFiles calls rewriteFileFunc for pkg.FilePaths
// and returns the rewritten sources by file path.
func rewriteFiles(pkg *ParsedPackage, verboseOut io.Writer, rewriteFileFunc RewriteFileFunc) (map[string][]byte, error) {
	astPkg := pkg.astPackage()
	rewritten := make(map[string][]byte, len(pkg.FilePaths))
	for _, filePath := range pkg.FilePaths {
		err := FprintfVerbose(verboseOut, "parsing file: %s\n", filePath)
		if err != nil {
			return nil, err
		}
		rewritten[filePath], err = rewriteFileFunc(pkg.Fset, astPkg, pkg.Files[filePath], filePath, verboseOut)
		if err != nil {
			return nil, err
		}
	}
	return rewritten, nil
}

// Rewrite transforms Go source files by applying a transformation function.
// It handles parsing, traversal, and writing modified files back to disk.
//
// Parameters:
//   - path: File or directory path. Append "..." for recursive directory traversal.
//     Example: "./..." processes all .go files in the current directory and subdirectories.
//   - verboseOut: Writer for verbose logging (nil to disable)
//   - resultOut: Writer for rewritten source (nil to write back to original files)
//   - rewriteFileFunc: Function called for each file to perform transformations
//
// The function:
//   - Skips test files (*_test.go)
//   - Skips files that ParsePackage skips, like files with a //go:build ignore constraint,
//     except for a file that is passed as path
//   - Skips directories that the go command ignores for ./... patterns
//     (starting with . or _, testdata, vendor, nested modules with their own go.mod file,
//     and directories matching an ignore directive of the module's go.mod file),
//     except for the directory of path itself, which is rewritten on purpose,
//     like its sub-directories if an ignore directive matches it
//   - Enters modules below path if path is not within a module,
//     for example the root of a repository with several modules
//   - Skips node_modules directories
//   - Processes files in sorted order for deterministic behavior
//   - Writes the files of a package only after rewriteFileFunc succeeded for all of them
//   - Writes only files with changed content, with the line endings (LF or CRLF) of the original file
//
// Rewrite is a wrapper around RewritePackages, see there for details.
//
// Returns an error if any file fails to process.
//
// Example - rename all identifiers:
//
//	err := astvisit.Rewrite("./...", os.Stdout, nil,
//	    func(fset *token.FileSet, pkg *ast.Package, file *ast.File,
//	         filePath string, verboseOut io.Writer) ([]byte, error) {
//	        // Use Visit to transform the AST
//	        astvisit.Visit(file, &myVisitor{}, nil)
//	        // Format and return the modified source
//	        var buf bytes.Buffer
//	        err := format.Node(&buf, fset, file)
//	        return buf.Bytes(), err
//	    })
func Rewrite(path string, verboseOut, resultOut io.Writer, rewriteFileFunc RewriteFileFunc) error {
	if rewriteFileFunc == nil {
		return errors.New("nil rewriteFileFunc")
	}
	return RewritePackages(path, verboseOut, resultOut, rewriteFileFunc.PackageFunc())
}

// ParsedPackage is a Go package parsed by RewritePackages and RewriteDryRun.
type ParsedPackage struct {
	// Dir is the absolute directory path of the package.
	Dir string
	// Name is the package name.
	Name string
	// Fset holds the positions of all Files.
	Fset *token.FileSet
	// Files holds the parsed files by absolute file path.
	Files map[string]*ast.File
	// Sources holds the source that each file in Files was parsed from
	// by the same file path. The positions in Fset refer to these bytes,
	// including the CR bytes of CRLF line endings.
	Sources map[string][]byte
	// FilePaths lists the sorted paths of the files to rewrite.
	// It contains all files of the package, except when
	// RewritePackages or RewriteDryRun were called for a single file.
	FilePaths []string
}

func (p *ParsedPackage) astPackage() *ast.Package {
	return &ast.Package{Name: p.Name, Files: p.Files}
}

// RewritePackageFunc is called once for each package by RewritePackages and RewriteDryRun.
// It returns the rewritten sources by file path. Only files listed in pkg.FilePaths
// are changed, files that are missing in the result or have nil sources are unchanged,
// an empty source is a change. A file path that is not in pkg.Files is an error,
// like a changed source for a file of pkg.Files that is not in pkg.FilePaths.
type RewritePackageFunc func(pkg *ParsedPackage, verboseOut io.Writer) (map[string][]byte, error)

// RewritePackages transforms the Go packages at path by calling rewritePackageFunc
// once per package with all parsed files and their sources.
// The files of a package are written only after rewritePackageFunc succeeded,
// so an error doesn't leave a package half rewritten.
//
// Parameters:
//   - path: File or directory path. Append "..." for recursive directory traversal.
//     For a file path the whole package is parsed, but only the file is rewritten.
//     Like the go command does for files named on the command line,
//     a file that is excluded from the build, for example by a //go:build ignore
//     constraint, is parsed as a package of its own.
//   - verboseOut: Writer for verbose logging (nil to disable)
//   - resultOut: Writer for rewritten source (nil to write back to original files)
//   - rewritePackageFunc: Function called for each package
//
// Files and directories are selected like described for Rewrite.
// A rewritten source is written with the line endings (LF or CRLF)
// of the first line of the original file, and only if it differs
// from the original file after that conversion.
//
// Use FileReplacementsFunc.PackageFunc or RewriteFileFunc.PackageFunc
// to call RewritePackages with a per file function.
func RewritePackages(path string, verboseOut, resultOut io.Writer, rewritePackageFunc RewritePackageFunc) error {
	if rewritePackageFunc == nil {
		return errors.New("nil rewritePackageFunc")
	}
	return rewritePackages(path, verboseOut, rewritePackageFunc, func(filePath string, source []byte) error {
		if resultOut != nil {
			_, err := resultOut.Write(source)
			return err
		}
		// File permissions 0644 are appropriate for generated source files
		return os.WriteFile(filePath, source, 0644) //#nosec G306
	})
}

// RewriteDryRun works like RewritePackages but doesn't write any files.
// It returns the paths of the files that RewritePackages would change,
// for example to check in CI that generated code is up to date.
func RewriteDryRun(path string, verboseOut io.Writer, rewritePackageFunc RewritePackageFunc) (changedFiles []string, err error) {
	if rewritePackageFunc == nil {
		return nil, errors.New("nil rewritePackageFunc")
	}
	err = rewritePackages(path, verboseOut, rewritePackageFunc, func(filePath string, _ []byte) error {
		changedFiles = append(changedFiles, filePath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return changedFiles, nil
}

func rewritePackages(path string, verboseOut io.Writer, rewritePackageFunc RewritePackageFunc, writeFile func(filePath string, source []byte) error) error {
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

	// Rewrite single file
	if !pathInfo.IsDir() {
		if !filterOutTests(pathInfo) {
			return fmt.Errorf("test file %s is not rewritten", path)
		}
		pkg, err := parseFilePackage(token.NewFileSet(), path, filterOutTests)
		if err != nil {
			return err
		}
		return rewritePackage(pkg, verboseOut, rewritePackageFunc, writeFile)
	}

	// Rewrite directory
	var mod *goModule
	if recursive {
		// The ignore directives of the module containing path
		// apply to its sub-directories
		mod, err = findModule(path)
		if err != nil {
			return err
		}
		if mod != nil {
			mod = mod.withoutIgnoresOf(path)
		}
	}
	return rewriteDir(path, recursive, mod, verboseOut, rewritePackageFunc, writeFile)
}

// rewriteDir rewrites the package in dir and if recursive
// the packages in the sub-directories of dir that the go command
// matches for ./... within the module mod, or if mod is nil
// because dir is not within a module, the packages of all modules below dir.
func rewriteDir(dir string, recursive bool, mod *goModule, verboseOut io.Writer, rewritePackageFunc RewritePackageFunc, writeFile func(filePath string, source []byte) error) error {
	pkg, err := parsePackage(token.NewFileSet(), dir, filterOutTests)
	if err != nil && (!recursive || !errors.Is(err, ErrPackageNotFound)) {
		return err
	}
	if err == nil {
		err = rewritePackage(pkg, verboseOut, rewritePackageFunc, writeFile)
		if err != nil {
			return err
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
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fileName := file.Name()
		// Skip directories ignored by the go command for ./... patterns and node_modules
		if !file.IsDir() || fileName[0] == '.' || fileName[0] == '_' || fileName == "testdata" || fileName == "vendor" || fileName == "node_modules" {
			continue
		}
		subDir := filepath.Join(dir, fileName)
		subMod := mod
		switch {
		case hasGoMod(subDir) && mod != nil:
			// Skip nested modules like the go command
			continue
		case hasGoMod(subDir):
			// Enter a module from a directory that is not within a module
			subMod, err = readModule(subDir)
			if err != nil {
				return err
			}
		case mod != nil && mod.ignores(subDir):
			continue
		}
		err = rewriteDir(subDir, true, subMod, verboseOut, rewritePackageFunc, writeFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func rewritePackage(pkg *ParsedPackage, verboseOut io.Writer, rewritePackageFunc RewritePackageFunc, writeFile func(filePath string, source []byte) error) error {
	rewritten, err := rewritePackageFunc(pkg, verboseOut)
	if err != nil {
		return err
	}
	for filePath, source := range rewritten {
		original, ok := pkg.Sources[filePath]
		if !ok {
			return fmt.Errorf("package %s has no file %s", pkg.Name, filePath)
		}
		// Only the files of pkg.FilePaths are written,
		// the change of another file would get lost silently
		if source != nil && !slices.Contains(pkg.FilePaths, filePath) && !bytes.Equal(withLineEndingsOf(source, original), original) {
			return fmt.Errorf("package %s: file %s is changed, but only %s can be rewritten", pkg.Name, filePath, strings.Join(pkg.FilePaths, ", "))
		}
	}
	for _, filePath := range pkg.FilePaths {
		original := pkg.Sources[filePath]
		source := rewritten[filePath]
		// Only nil means unchanged, an empty source is a change
		changed := source != nil
		if changed {
			source = withLineEndingsOf(source, original)
			changed = !bytes.Equal(source, original)
		}
		if !changed {
			err = FprintfVerbose(verboseOut, "no changes in file: %s\n", filePath)
			if err != nil {
				return err
			}
			continue
		}
		err = FprintfVerbose(verboseOut, "changed file: %s\n", filePath)
		if err != nil {
			return err
		}
		err = writeFile(filePath, source)
		if err != nil {
			return err
		}
	}
	return nil
}

// withLineEndingsOf returns source with CRLF line endings
// if the first line of original ends with CRLF, else with LF line endings.
func withLineEndingsOf(source, original []byte) []byte {
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	if i := bytes.IndexByte(original, '\n'); i > 0 && original[i-1] == '\r' {
		source = bytes.ReplaceAll(source, []byte("\n"), []byte("\r\n"))
	}
	return source
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
