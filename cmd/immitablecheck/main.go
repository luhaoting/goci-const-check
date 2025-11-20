package main

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var Analyzer = &analysis.Analyzer{
	Name: "immutablefield",
	Doc:  "report assignments to struct fields marked immutable (tag `immutable:\"true\"` or comment // immutable)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// set of immutable fields (use map of *types.Var -> true)
	imm := map[*types.Var]bool{}

	// 1) Walk AST to find struct type declarations and record immutable fields
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}

			// Try to get the named type for this TypeSpec
			obj := pass.TypesInfo.Defs[ts.Name]
			if obj == nil {
				return true
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				return true
			}
			strct, ok := named.Underlying().(*types.Struct)
			if !ok {
				return true
			}

			// iterate fields in AST struct and types.Struct in parallel
			for i, field := range st.Fields.List {
				// anonymous fields (embedded) may map differently; skip if no names
				if len(field.Names) == 0 {
					// embedded: there can still be a types.Struct field j; still check tag/comments
				}
				// check tag text for immutable:"true"
				tagIsImmutable := false
				if field.Tag != nil {
					tagText := strings.Trim(field.Tag.Value, "`\"")
					// cheap check for immutable tag
					if strings.Contains(tagText, `immutable:"true"`) || strings.Contains(tagText, `immutable:"1"`) {
						tagIsImmutable = true
					}
				}
				// check trailing comment for "immutable"
				if !tagIsImmutable && field.Comment != nil {
					for _, c := range field.Comment.List {
						if strings.Contains(strings.ToLower(c.Text), "immutable") {
							tagIsImmutable = true
							break
						}
					}
				}
				// also check doc comment
				if !tagIsImmutable && field.Doc != nil {
					for _, c := range field.Doc.List {
						if strings.Contains(strings.ToLower(c.Text), "immutable") {
							tagIsImmutable = true
							break
						}
					}
				}

				if tagIsImmutable {
					// defensive: types.Struct.NumFields() may be fewer if something odd; check bounds
					if i < strct.NumFields() {
						tv := strct.Field(i)
						imm[tv] = true
					}
				}
			}
			return true
		})
	}

	// 2) Walk files to find assignments / incdec that modify selector expressions
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.AssignStmt:
				// LHS may contain selector expressions
				for _, lhs := range stmt.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok {
						if selInfo, found := pass.TypesInfo.Selections[sel]; found {
							// selection is a field or method; we care about fields
							if v, ok := selInfo.Obj().(*types.Var); ok {
								if imm[v] {
									pass.Reportf(sel.Pos(), "assignment to immutable field %s", v.Name())
								}
							}
						}
					}
				}
			case *ast.IncDecStmt:
				if sel, ok := stmt.X.(*ast.SelectorExpr); ok {
					if selInfo, found := pass.TypesInfo.Selections[sel]; found {
						if v, ok := selInfo.Obj().(*types.Var); ok {
							if imm[v] {
								pass.Reportf(sel.Pos(), "modifying immutable field %s (inc/dec)", v.Name())
							}
						}
					}
				}
			case *ast.UnaryExpr:
				// e.g., &x.Field is okay; but ++/-- are not unary in ast (they're IncDecStmt).
				_ = stmt
			}
			return true
		})
	}

	return nil, nil
}

func main() {
	singlechecker.Main(Analyzer)
}
