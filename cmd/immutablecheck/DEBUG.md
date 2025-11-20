# Immutablecheck 分析器调试指南

## 快速开始

### 1. 运行单个测试

```bash
# 测试 Descriptor Set 加载
go test -v -run TestLoadDescriptorSet

# 测试 snake_case 转换
go test -v -run TestSnakeToCamelCase

# 测试主文件分析
go test -v -run TestAnalyzeMainFile

# 测试类型解析
go test -v -run TestGetReceiverTypeName

# 运行所有测试
go test -v
```

### 2. 运行分析器

```bash
# 编译分析器
go install ./cmd/immitablecheck

# 分析 main.go
immitablecheck ../main.go

# 分析整个项目（排除 pbtagger 错误）
immitablecheck ./... 2>&1 | grep -v pbtagger
```

### 3. 运行完整调试脚本

```bash
pwsh -File debug.ps1
```

## 测试结果

### Descriptor Set 加载测试
- ✓ 成功加载 13873 字节的 descriptor set
- ✓ 检测到 Person 消息的不可变字段: [id, age]
- ✓ 检测到 School 消息的不可变字段: [teachers]

### Snake Case 转换测试
- ✓ id → Id
- ✓ name → Name
- ✓ age → Age
- ✓ full_name → FullName
- ✓ user_id → UserId

### 分析结果
```
G:\test\goci-const-check\main.go:9:2: assignment to immutable field Id
G:\test\goci-const-check\main.go:11:2: assignment to immutable field Age
```

## 调试输出

### 第 1 步：验证 Descriptor Set

```
✓ Descriptor set found at: pb/descriptor/all.protos.pb
  Size: 13873 bytes
```

### 第 2 步：加载 Proto 不可变字段信息

测试代码输出：
```
=== Loaded Proto Immutable Fields ===
Message: Person
  Immutable fields: [id, age]
Message: School
  Immutable fields: [teachers]
```

### 第 3 步：分析代码中的赋值语句

| 行号 | 字段 | 接收者类型 | 包名 | 状态 |
|-----|-----|---------|-----|-----|
| 9 | Id | Person | pb | ❌ 错误 |
| 10 | Name | Person | pb | ✓ 正常 |
| 11 | Age | Person | pb | ❌ 错误 |

## 工作原理

### 1. 类型解析

使用 `types.Selection.Recv()` 获取字段所属的结构体：
```
Line 9: Field=Id, TypeName=Person, RecvType=*goci-const-check/pb.Person
```

### 2. 不可变字段检查

1. 从 proto descriptor set 读取选项 59527 的编码 `[184 136 29 1]`
2. 提取标记为不可变的字段名称
3. 将 proto 字段名（snake_case）转换为 Go 字段名（CamelCase）
4. 检查每个赋值语句是否违反不可变性约束

### 3. 跨包检查

对于使用 pb 包中的类型（如 `pb.Person`）的赋值，分析器会：
1. 识别字段属于 pb 包
2. 查找该消息在 proto descriptor 中的不可变字段定义
3. 报告违反不可变性的赋值

## 常见问题

### Q: 分析器没有检测到不可变字段？
A: 检查以下几点：
1. Descriptor set 文件是否存在于 `pb/descriptor/all.protos.pb`
2. Proto 文件中是否正确标记了 `[(example.immutable) = true]`
3. 在 proto-gen.bat 中是否使用了 `--descriptor_set_out` 生成 descriptor set

### Q: 类型解析失败？
A: 使用 `TestGetReceiverTypeName` 测试来检查类型解析是否工作正常

### Q: snake_case 转换出错？
A: 运行 `TestSnakeToCamelCase` 验证转换逻辑

## 自定义测试

可以在 `debug_test.go` 中添加新的测试用例：

```go
func TestCustomCase(t *testing.T) {
    // 你的测试代码
}
```

然后运行：
```bash
go test -v -run TestCustomCase
```

## 调试技巧

### 1. 启用详细输出
修改 `main.go` 中的分析器，添加调试打印：
```go
fmt.Printf("DEBUG: Checking field %s in type %s\n", v.Name(), typeName)
```

### 2. 验证 Descriptor Set 内容
```go
info, _ := loadDescriptorSet("pb/descriptor/all.protos.pb")
for name, fields := range info {
    fmt.Printf("%s: %v\n", name, fields.FieldNames)
}
```

### 3. 检查选择信息
```go
if selInfo, found := pass.TypesInfo.Selections[sel]; found {
    fmt.Printf("Selection found: %v\n", selInfo)
}
```

## 相关文件

- `main.go` - 分析器核心代码
- `debug_test.go` - 调试测试用例
- `debug.ps1` - 自动化调试脚本
- `../main.go` - 测试目标文件
- `pb/descriptor/all.protos.pb` - Proto descriptor set
