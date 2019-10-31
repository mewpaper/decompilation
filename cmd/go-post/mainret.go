package main

import (
	"go/ast"
	"log"
)

func init() {
	register(mainretFix)
}

var mainretFix = fix{
	name:     "mainret",
	date:     "2015-02-27",
	f:        mainret,
	desc:     `Replace return statements with calls to os.Exit in the "main" function.`,
	// Note: mainret is disabled by default, as it has been superseded by the
	// "cmain" rule.
	disabled: true,
}

func mainret(file *ast.File) bool {
	fixed := false

	// Locate the "main" function.
	mainFunc, ok := findMainFunc(file)
	if !ok {
		return false
	}

	// Apply the following transitions for the "main" function:
	//
	// 1)
	//    // from:
	//    return 42
	//
	//    // to:
	//    os.Exit(42)
	//
	// 2)
	//    // from:
	//    return 0
	//
	//    // to:
	//    return
	//
	// 3)
	//    // from:
	//    func main() {
	//       return
	//    }
	//
	//    // to:
	//    func main() {
	//    }
	//
	// 4)
	//    // from:
	//    func main(_0 int32, _1 **int8) int32 {
	//    }
	//
	//    // to:
	//    func main() {
	//    }
	walk(mainFunc, func(n interface{}) {
		stmt, ok := n.(*ast.Stmt)
		if !ok {
			return
		}
		retStmt, ok := (*stmt).(*ast.ReturnStmt)
		if !ok {
			return
		}
		switch len(retStmt.Results) {
		case 0:
			// Leave blank returns as is.
			return
		case 1:
			result := retStmt.Results[0]
			if isZero(result) {
				// Replace "return 0" with "return".
				retStmt.Results = nil
			} else {
				// Add "os" import if needed.
				addImport(file, "os")
				// Replace "return 42" with "os.Exit(42)".
				exit := createExit(result)
				*stmt = exit
			}
			fixed = true
		default:
			log.Fatalf("invalid number of arguments to return; expected 1, got %d", len(retStmt.Results))
		}
	})

	// Remove trailing blank return statement.
	list := mainFunc.Body.List
	n := len(list)
	if n > 0 {
		if isEmptyReturn(list[n-1]) {
			mainFunc.Body.List = list[:n-1]
			fixed = true
		}
	}

	// Update function signature.
	if mainFunc.Type.Results != nil && len(mainFunc.Type.Results.List) > 0 {
		mainFunc.Type.Results = nil
		fixed = true
	}
	//if mainFunc.Type.Params != nil && len(mainFunc.Type.Params.List) > 0 {
	//	mainFunc.Type.Params = nil
	//	fixed = true
	//}

	return fixed
}

// createExit creates and returns an "os.Exit" call with the specified argument.
func createExit(arg ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				// TODO: Locate the original identifier of "os" instead of creating
				// a new one.
				X:   ast.NewIdent("os"),
				Sel: ast.NewIdent("Exit"),
			},
			Args: []ast.Expr{
				&ast.CallExpr{
					Fun: ast.NewIdent("int"),
					Args: []ast.Expr{arg},
				},
			},
		},
	}
}

// isZero returns true if n is a integer literal with the value 0.
func isZero(n ast.Expr) bool {
	lit, ok := n.(*ast.BasicLit)
	return ok && lit.Value == "0"
}

// isEmptyReturn returns true if the given statement is an empty return
// statement (i.e. "return"), and false otherwise.
func isEmptyReturn(stmt ast.Stmt) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	return ok && len(ret.Results) == 0
}

// findMainFunc attempts to locate the "main" function of the provided file. The
// boolean value is true if successful, and false otherwise.
func findMainFunc(file *ast.File) (f *ast.FuncDecl, ok bool) {
	for _, f := range file.Decls {
		switch f := f.(type) {
		case *ast.FuncDecl:
			if f.Name.Name == "main" {
				return f, true
			}
		}
	}
	return nil, false
}
