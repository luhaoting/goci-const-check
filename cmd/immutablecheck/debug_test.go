package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestLoadDescriptorSet 测试加载 descriptor set
func TestLoadDescriptorSet(t *testing.T) {
	path := "../../pb/descriptor/all.protos.pb"
	info, err := loadDescriptorSet(path)
	if err != nil {
		t.Fatalf("Failed to load descriptor set: %v", err)
	}

	fmt.Println("=== Loaded Proto Immutable Fields ===")
	for msgName, fieldInfo := range info {
		fmt.Printf("Message: %s\n", msgName)
		fmt.Printf("  Immutable fields: %v\n", fieldInfo.FieldNames)
	}
}

// TestAnalyzeMainFile 测试分析 main.go
func TestAnalyzeMainFile(t *testing.T) {
	fmt.Println("\n=== Analyzing main.go ===")

	// 加载 descriptor set
	descriptorInfo, err := loadDescriptorSet("../../pb/descriptor/all.protos.pb")
	if err != nil {
		t.Fatalf("Failed to load descriptor set: %v", err)
	}

	// 使用 packages 包加载项目
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  "../../",
	}

	pkgs, err := packages.Load(cfg, "goci-const-check")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("No packages loaded")
	}

	pkg := pkgs[0]
	fmt.Printf("Package: %s\n", pkg.PkgPath)

	// 解析 main.go
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, "../../main.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("Failed to parse main.go: %v", err)
	}

	fmt.Println("\n=== Assignment Statements Found ===")
	ast.Inspect(mainFile, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range stmt.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					// 尝试找到选择信息
					if selInfo, found := pkg.TypesInfo.Selections[sel]; found {
						if v, ok := selInfo.Obj().(*types.Var); ok {
							typeName := getReceiverTypeName(selInfo)
							pkgName := v.Pkg().Name()
							pos := fset.Position(sel.Pos())

							fmt.Printf("Line %d: %s.%s (type: %s, pkg: %s)\n",
								pos.Line, typeName, v.Name(), v.Type(), pkgName)

							// 检查是否为不可变字段
							if protoInfo, hasProtoInfo := descriptorInfo[typeName]; hasProtoInfo {
								for _, immField := range protoInfo.FieldNames {
									if v.Name() == snakeToCamelCase(immField) {
										fmt.Printf("  ❌ ERROR: Assignment to immutable field %s\n", v.Name())
										break
									}
								}
							}
						}
					}
				}
				_ = i
			}
		}
		return true
	})
}

// TestSnakeToCamelCase 测试 snake_case 到 CamelCase 的转换
func TestSnakeToCamelCase(t *testing.T) {
	tests := map[string]string{
		"id":        "Id",
		"name":      "Name",
		"age":       "Age",
		"full_name": "FullName",
		"user_id":   "UserId",
	}

	fmt.Println("\n=== Snake Case to Camel Case Conversion ===")
	for input, expected := range tests {
		result := snakeToCamelCase(input)
		status := "✓"
		if result != expected {
			status = "✗"
		}
		fmt.Printf("%s %s -> %s (expected: %s)\n", status, input, result, expected)
	}
}

// TestGetReceiverTypeName 测试提取接收者类型名
func TestGetReceiverTypeName(t *testing.T) {
	fmt.Println("\n=== Testing Type Resolution ===")

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  "../../",
	}

	pkgs, err := packages.Load(cfg, "goci-const-check")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("No packages loaded")
	}

	pkg := pkgs[0]

	// 解析 main.go
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, "../../main.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("Failed to parse main.go: %v", err)
	}

	// 找到赋值语句并打印类型信息
	ast.Inspect(mainFile, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range stmt.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					if selInfo, found := pkg.TypesInfo.Selections[sel]; found {
						if v, ok := selInfo.Obj().(*types.Var); ok {
							typeName := getReceiverTypeName(selInfo)
							recv := selInfo.Recv()
							pos := fset.Position(sel.Pos())

							fmt.Printf("Line %d: Field=%s, TypeName=%s, RecvType=%s\n",
								pos.Line, v.Name(), typeName, recv)
						}
					}
				}
			}
		}
		return true
	})
}

// DebugMain 用于快速调试
func DebugMain() {
	fmt.Println("=== Immutablecheck Debug Tests ===")

	// 检查 descriptor set 是否存在
	path := "../../pb/descriptor/all.protos.pb"
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("❌ Descriptor set not found at: %s\n", path)
		return
	}
	fmt.Printf("✓ Descriptor set found at: %s\n", path)

	// 加载并打印 proto 信息
	info, err := loadDescriptorSet(path)
	if err != nil {
		fmt.Printf("❌ Failed to load descriptor set: %v\n", err)
		return
	}

	fmt.Println("Proto Immutable Fields:")
	for msgName, fieldInfo := range info {
		fmt.Printf("  %s: %v\n", msgName, fieldInfo.FieldNames)
	}
}
