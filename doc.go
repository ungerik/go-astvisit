// Package astvisit provides a type-safe visitor pattern for traversing and
// modifying Go Abstract Syntax Trees (AST) with minimal reflection.
//
// The package offers a strongly-typed alternative to ast.Inspect and ast.Walk,
// with dedicated visitor methods for each AST node type. This approach provides
// better type safety, IDE support, and compile-time checking compared to
// reflection-based approaches.
//
// # Basic Usage
//
// Define a visitor by embedding VisitorImpl and overriding methods for the
// node types you're interested in:
//
//	type myVisitor struct {
//	    astvisit.VisitorImpl
//	}
//
//	func (v *myVisitor) VisitFuncDecl(node *ast.FuncDecl, cursor astvisit.Cursor) bool {
//	    fmt.Printf("Function: %s\n", node.Name.Name)
//	    return true // Continue visiting children
//	}
//
//	// Use the visitor
//	astvisit.Visit(astFile, &myVisitor{}, nil)
//
// # Pre and Post Order Traversal
//
// The Visit function supports both pre-order and post-order visitors:
//
//	preVisitor := &myPreVisitor{}
//	postVisitor := &myPostVisitor{}
//	astvisit.Visit(astFile, preVisitor, postVisitor)
//
// # Cursor API
//
// Each visitor method receives a Cursor that provides context and modification
// capabilities:
//
//	func (v *myVisitor) VisitIdent(node *ast.Ident, cursor astvisit.Cursor) bool {
//	    // Get parent context
//	    parent := cursor.Parent()
//	    path := cursor.Path()
//
//	    // Modify AST
//	    if node.Name == "old" {
//	        newIdent := &ast.Ident{Name: "new"}
//	        cursor.Replace(newIdent)
//	    }
//
//	    return true
//	}
//
// # Source Rewriting
//
// The Rewrite function enables bulk transformation of source files:
//
//	err := astvisit.Rewrite("./...", os.Stdout, nil,
//	    func(fset *token.FileSet, pkg *ast.Package, file *ast.File,
//	         filePath string, verboseOut io.Writer) ([]byte, error) {
//	        // Transform and return modified source
//	        return modifiedSource, nil
//	    })
//
// # Node Replacements
//
// For precise source code modifications, use NodeReplacements:
//
//	var replacements astvisit.NodeReplacements
//	replacements.AddReplacement(oldNode, newNode)
//	replacements.AddRemoval(unusedNode)
//	newSource, err := replacements.Apply(fset, originalSource)
//
// # Go 1.18+ Generics
//
// The package fully supports Go generics including IndexListExpr for
// multiple type parameters:
//
//	func (v *myVisitor) VisitIndexListExpr(node *ast.IndexListExpr, cursor astvisit.Cursor) bool {
//	    // Handles: MyType[T, U], receiver[T, U](), etc.
//	    return true
//	}
package astvisit
