package main

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var Analyzer = &analysis.Analyzer{
	Name: "immutablefield",
	Doc:  "report assignments to struct fields marked immutable (from proto or Go tags/comments)",
	Run:  run,
}

// ImmutableFieldInfo holds info about immutable fields from proto
type ImmutableFieldInfo struct {
	MessageName string   // e.g., "Person"
	FieldNames  []string // e.g., ["id", "age"]
}

// loadDescriptorSet reads the protobuf descriptor set file
func loadDescriptorSet(path string) (map[string]*ImmutableFieldInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return nil, err
	}

	result := make(map[string]*ImmutableFieldInfo)

	// Look for the custom immutable option (59527)
	for _, fd := range fds.File {
		for _, msg := range fd.MessageType {
			info := &ImmutableFieldInfo{
				MessageName: msg.GetName(),
				FieldNames:  []string{},
			}

			for _, field := range msg.Field {
				if field.Options != nil {
					// Check if field has the immutable option
					// The option is encoded in the raw bytes
					optBytes, _ := proto.Marshal(field.Options)

					// Check for field number 59527 (0xE887 * 8 + 1 = wire format)
					// The encoding for option 59527 with value 1 is: [184 136 29 1]
					// This is wire format: (59527 << 3) | 1 (varint), then varint value 1
					// 59527 = 0xE887
					// 0xE887 << 3 | 1 = 0xE8871 = wire format bytes

					// Simpler approach: check if the bytes contain the expected pattern
					if len(optBytes) >= 4 && optBytes[0] == 184 && optBytes[1] == 136 && optBytes[2] == 29 && optBytes[3] == 1 {
						info.FieldNames = append(info.FieldNames, field.GetName())
					}
				}
			}

			if len(info.FieldNames) > 0 {
				result[info.MessageName] = info
			}
		}
	}

	return result, nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Load protobuf immutable info
	protoImmutableInfo := make(map[string]*ImmutableFieldInfo)
	possiblePaths := []string{
		"pb/descriptor/all.protos.pb",
		"./pb/descriptor/all.protos.pb",
		filepath.Join(pass.Pkg.Path(), "../../pb/descriptor/all.protos.pb"),
		filepath.Join(pass.Pkg.Path(), "../pb/descriptor/all.protos.pb"),
		filepath.Join(pass.Pkg.Path(), "pb/descriptor/all.protos.pb"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			if info, err := loadDescriptorSet(path); err == nil {
				protoImmutableInfo = info
				break
			}
		}
	}

	// Build a map of field -> immutable status
	immutableFields := make(map[*types.Var]bool)

	// Check all defined types in this package
	scope := pass.Pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}

		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		strct, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}

		// Check proto definitions for this struct
		protoInfo, hasProtoInfo := protoImmutableInfo[named.Obj().Name()]
		if hasProtoInfo {
			for i := 0; i < strct.NumFields(); i++ {
				field := strct.Field(i)
				for _, immField := range protoInfo.FieldNames {
					if strings.EqualFold(field.Name(), immField) ||
						strings.EqualFold(field.Name(), snakeToCamelCase(immField)) {
						immutableFields[field] = true
						break
					}
				}
			}
		}
	}

	// Also check struct definitions in current files for Go tags/comments
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

			// Get the type object
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

			// Check Go tags/comments for immutable fields
			for i := 0; i < strct.NumFields(); i++ {
				field := strct.Field(i)
				if len(st.Fields.List) <= i {
					continue
				}
				astField := st.Fields.List[i]

				isImmutable := false

				// Check Go tags
				if astField.Tag != nil {
					tagText := strings.Trim(astField.Tag.Value, "`\"")
					if strings.Contains(tagText, `immutable:"true"`) || strings.Contains(tagText, `immutable:"1"`) {
						isImmutable = true
					}
				}
				// Check trailing comment
				if !isImmutable && astField.Comment != nil {
					for _, c := range astField.Comment.List {
						if strings.Contains(strings.ToLower(c.Text), "immutable") {
							isImmutable = true
							break
						}
					}
				}
				// Check doc comment
				if !isImmutable && astField.Doc != nil {
					for _, c := range astField.Doc.List {
						if strings.Contains(strings.ToLower(c.Text), "immutable") {
							isImmutable = true
							break
						}
					}
				}

				if isImmutable {
					immutableFields[field] = true
				}
			}

			return true
		})
	}

	// Now walk through the code looking for assignments to immutable fields
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range stmt.Lhs {
					// Check direct field assignment:
					if sel, ok := lhs.(*ast.SelectorExpr); ok {
						if selInfo, found := pass.TypesInfo.Selections[sel]; found {
							if v, ok := selInfo.Obj().(*types.Var); ok {
								// Get receiver type name
								typeName := getReceiverTypeName(selInfo)
								pkgName := v.Pkg().Name()

								// Check local immutable fields
								if immutableFields[v] {
									pass.Reportf(sel.Pos(), "assignment to immutable field %s", v.Name())
								} else {
									// Check if this field is from pb package and might be immutable from proto
									protoInfo, hasProtoInfo := protoImmutableInfo[typeName]
									if hasProtoInfo && (pkgName == "pb" || strings.HasSuffix(v.Pkg().Path(), "/pb")) {
										for _, immField := range protoInfo.FieldNames {
											if strings.EqualFold(v.Name(), immField) ||
												strings.EqualFold(v.Name(), snakeToCamelCase(immField)) {
												pass.Reportf(sel.Pos(), "assignment to immutable field %s", v.Name())
												break
											}
										}
									}
								}
							}
						}
					}

					// Check map index assignment:
					if idx, ok := lhs.(*ast.IndexExpr); ok {
						// Extract the X part (the map/slice being indexed)
						if sel, ok := idx.X.(*ast.SelectorExpr); ok {
							if selInfo, found := pass.TypesInfo.Selections[sel]; found {
								if v, ok := selInfo.Obj().(*types.Var); ok {
									// Check if the field being indexed is immutable
									typeName := getReceiverTypeName(selInfo)
									pkgName := v.Pkg().Name()

									// Check local immutable fields
									if immutableFields[v] {
										pass.Reportf(idx.Pos(), "modifying immutable field %s (map/slice index)", v.Name())
									} else {
										// Check if this field is from pb package and might be immutable from proto
										protoInfo, hasProtoInfo := protoImmutableInfo[typeName]
										if hasProtoInfo && (pkgName == "pb" || strings.HasSuffix(v.Pkg().Path(), "/pb")) {
											for _, immField := range protoInfo.FieldNames {
												if strings.EqualFold(v.Name(), immField) ||
													strings.EqualFold(v.Name(), snakeToCamelCase(immField)) {
													pass.Reportf(idx.Pos(), "modifying immutable field %s (map/slice index)", v.Name())
													break
												}
											}
										}
									}
								}
							}
						}
					}
				}
			case *ast.IncDecStmt:
				if sel, ok := stmt.X.(*ast.SelectorExpr); ok {
					if selInfo, found := pass.TypesInfo.Selections[sel]; found {
						if v, ok := selInfo.Obj().(*types.Var); ok {
							// Get receiver type name
							typeName := getReceiverTypeName(selInfo)
							pkgName := v.Pkg().Name()

							// Check local immutable fields
							if immutableFields[v] {
								pass.Reportf(sel.Pos(), "modifying immutable field %s (inc/dec)", v.Name())
							}

							// Check if this field is from pb package and might be immutable from proto
							protoInfo, hasProtoInfo := protoImmutableInfo[typeName]
							if hasProtoInfo && (pkgName == "pb" || strings.HasSuffix(v.Pkg().Path(), "/pb")) {
								for _, immField := range protoInfo.FieldNames {
									if strings.EqualFold(v.Name(), immField) ||
										strings.EqualFold(v.Name(), snakeToCamelCase(immField)) {
										pass.Reportf(sel.Pos(), "modifying immutable field %s (inc/dec)", v.Name())
										break
									}
								}
							}
						}
					}
				}
			}
			return true
		})
	}

	return nil, nil
}

// snakeToCamelCase converts snake_case to CamelCase
func snakeToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// getReceiverTypeName gets the struct name from a field's parent type (stored in selection)
func getReceiverTypeName(selInfo *types.Selection) string {
	recv := selInfo.Recv()
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	if named, ok := recv.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func main() {
	singlechecker.Main(Analyzer)
}
