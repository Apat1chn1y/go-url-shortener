// Package analyzer предоставляет статический анализатор для проверки использования
// panic и вызовов log.Fatal/os.Exit вне функции main пакета main.
package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

const doc = "checker detects usage of panic and calls to log.Fatal/os.Exit outside main.main"

var generatedRegexp = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// Analyzer - экземпляр анализатора.
var Analyzer = &analysis.Analyzer{
	Name: "checker",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Собираем позиции вызовов, находящихся внутри main() (только для пакета main)
	mainPositions := make(map[token.Pos]bool)
	if pass.Pkg.Name() == "main" {
		for _, f := range pass.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
					if fn.Body != nil {
						ast.Inspect(fn.Body, func(node ast.Node) bool {
							if node != nil {
								mainPositions[node.Pos()] = true
							}
							return true
						})
					}
					return false
				}
				return true
			})
		}
	}

	for _, file := range pass.Files {
		if isGeneratedFile(file) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if isPanic(x) {
					pass.Reportf(x.Pos(), "use of panic detected")
					return true
				}
				if isForbiddenCall(x, pass) {
					if _, ok := mainPositions[x.Pos()]; !ok {
						pass.Reportf(x.Pos(), "call to %s outside main.main", getCallName(x))
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// isGeneratedFile проверяет, является ли файл сгенерированным.
func isGeneratedFile(file *ast.File) bool {
	if file.Doc != nil {
		for _, c := range file.Doc.List {
			if generatedRegexp.MatchString(c.Text) {
				return true
			}
		}
	}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if generatedRegexp.MatchString(c.Text) {
				return true
			}
		}
	}
	return false
}

// isPanic проверяет, является ли вызов panic.
func isPanic(call *ast.CallExpr) bool {
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name == "panic"
	}
	return false
}

// isForbiddenCall проверяет, является ли вызов log.Fatal или os.Exit.
func isForbiddenCall(call *ast.CallExpr, pass *analysis.Pass) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}
	pkgPath := pkgName.Imported().Path()
	selName := sel.Sel.Name
	if pkgPath == "log" && (selName == "Fatal" || selName == "Fatalf" || selName == "Fatalln") {
		return true
	}
	if pkgPath == "os" && selName == "Exit" {
		return true
	}
	return false
}

// getCallName возвращает имя вызываемой функции для сообщения.
func getCallName(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name
	}
	return "unknown"
}
