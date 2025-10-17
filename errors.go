package astvisit

import "errors"

// ErrPackageNotFound is returned by ParsePackage when no Go source files
// are found in the specified directory (after applying any filter function).
//
// This error can be wrapped with additional context. Use errors.Is() to check:
//
//	pkg, err := ParsePackage(fset, dir, filter)
//	if errors.Is(err, astvisit.ErrPackageNotFound) {
//	    // Handle missing package
//	}
var ErrPackageNotFound = errors.New("package not found")
