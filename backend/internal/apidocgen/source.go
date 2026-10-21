package apidocgen

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

type function struct {
	decl *ast.FuncDecl
	pkg  *packages.Package
	obj  *types.Func
}

type sources struct {
	functions   map[string]*function
	objects     map[*types.Func]*function
	permissions map[string]map[string]bool
}

func functionKey(obj *types.Func) string {
	if obj.Pkg() == nil {
		return ""
	} // Built-in interfaces such as error.Error.
	sig := obj.Type().(*types.Signature)
	receiver := ""
	if sig.Recv() != nil {
		t := sig.Recv().Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok {
			receiver = "(*" + n.Obj().Name() + ")."
		}
	}
	return obj.Pkg().Path() + "." + receiver + obj.Name()
}

func loadSources(dir string) (*sources, error) {
	pkgs, err := packages.Load(&packages.Config{Dir: dir, Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports},
		"./internal/features/...", "./internal/api/handler", "./internal/utils", "./internal/http/...")
	if err != nil {
		return nil, err
	}
	s := &sources{functions: map[string]*function{}, objects: map[*types.Func]*function{}, permissions: map[string]map[string]bool{}}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load %s: %s", pkg.PkgPath, pkg.Errors[0])
		}
		for _, file := range pkg.Syntax {
			for _, node := range file.Decls {
				decl, ok := node.(*ast.FuncDecl)
				if !ok || decl.Body == nil {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
				if !ok {
					continue
				}
				f := &function{decl: decl, pkg: pkg, obj: obj}
				s.functions[functionKey(obj)] = f
				s.objects[obj] = f
			}
		}
	}
	s.findPermissions()
	return s, nil
}

func callee(pkg *packages.Package, expr ast.Expr) *types.Func {
	switch node := expr.(type) {
	case *ast.Ident:
		obj, _ := pkg.TypesInfo.Uses[node].(*types.Func)
		return obj
	case *ast.SelectorExpr:
		if selection := pkg.TypesInfo.Selections[node]; selection != nil {
			obj, _ := selection.Obj().(*types.Func)
			return obj
		}
		obj, _ := pkg.TypesInfo.Uses[node.Sel].(*types.Func)
		return obj
	}
	return nil
}

func stringValue(pkg *packages.Package, expr ast.Expr) (string, bool) {
	value := pkg.TypesInfo.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

func hasContext(f *function) bool {
	params := f.obj.Type().(*types.Signature).Params()
	for i := 0; i < params.Len(); i++ {
		if types.TypeString(params.At(i).Type(), nil) == "*github.com/gin-gonic/gin.Context" {
			return true
		}
	}
	return false
}

// Walk helpers taking a Gin context as well, e.g. shared pagination and query parsers.
func (s *sources) walk(f *function, visit func(*function, ast.Node)) {
	seen := map[string]bool{}
	var walk func(*function)
	walk = func(current *function) {
		key := functionKey(current.obj)
		if seen[key] {
			return
		}
		seen[key] = true
		ast.Inspect(current.decl.Body, func(node ast.Node) bool {
			if node == nil {
				return false
			}
			visit(current, node)
			if call, ok := node.(*ast.CallExpr); ok {
				if obj := callee(current.pkg, call.Fun); obj != nil {
					next := s.objects[obj]
					if next == nil {
						next = s.functions[functionKey(obj)]
					}
					if next != nil && hasContext(next) {
						walk(next)
					}
				}
			}
			return true
		})
	}
	walk(f)
}

// Associate explicit route permissions with handler methods, including middleware
// variables. Group requirements remain enforced by the application and are
// documented in each tag description rather than guessed from URL names.
func (s *sources) findPermissions() {
	for _, f := range s.functions {
		if !strings.HasPrefix(f.decl.Name.Name, "Register") {
			continue
		}
		bindings := map[types.Object]ast.Expr{}
		ast.Inspect(f.decl.Body, func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for i, lhs := range assignment.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && i < len(assignment.Rhs) {
						bindings[f.pkg.TypesInfo.ObjectOf(id)] = assignment.Rhs[i]
					}
				}
			}
			return true
		})
		var collect func(ast.Expr, map[string]bool, map[types.Object]bool)
		collect = func(expr ast.Expr, result map[string]bool, seen map[types.Object]bool) {
			if id, ok := expr.(*ast.Ident); ok {
				obj := f.pkg.TypesInfo.ObjectOf(id)
				if !seen[obj] {
					seen[obj] = true
					if value := bindings[obj]; value != nil {
						collect(value, result, seen)
					}
				}
				return
			}
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				return
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "RequirePermission" {
				return
			}
			for _, arg := range call.Args {
				if value, ok := stringValue(f.pkg, arg); ok {
					result[value] = true
				}
			}
		}
		ast.Inspect(f.decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !isHTTPMethod(method.Sel.Name) {
				return true
			}
			obj := callee(f.pkg, call.Args[len(call.Args)-1])
			if obj == nil {
				return true
			}
			key := functionKey(obj) + ":" + method.Sel.Name
			if s.permissions[key] == nil {
				s.permissions[key] = map[string]bool{}
			}
			for _, arg := range call.Args[1 : len(call.Args)-1] {
				collect(arg, s.permissions[key], map[types.Object]bool{})
			}
			return true
		})
	}
}

func isHTTPMethod(value string) bool {
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}
