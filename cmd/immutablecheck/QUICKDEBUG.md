# 快速调试命令

## 测试分析器功能

### 1. 测试 Descriptor Set 加载
```powershell
cd g:\test\goci-const-check\cmd\immitablecheck
go test -v -run TestLoadDescriptorSet
```

输出示例：
```
=== Loaded Proto Immutable Fields ===
Message: Person
  Immutable fields: [id, age]
Message: School
  Immutable fields: [teachers]
```

### 2. 测试 Snake Case 转换
```powershell
go test -v -run TestSnakeToCamelCase
```

### 3. 分析 main.go
```powershell
cd g:\test\goci-const-check
immitablecheck ./main.go
```

预期输出：
```
G:\test\goci-const-check\main.go:9:2: assignment to immutable field Id
G:\test\goci-const-check\main.go:11:2: assignment to immutable field Age
```

### 4. 完整分析（排除 pbtagger）
```powershell
immitablecheck ./... 2>&1 | Select-String -Pattern "main\.go"
```

## 完整调试流程

### 脚本方式
```powershell
cd g:\test\goci-const-check\cmd\immitablecheck
.\debug.ps1
```

### 手动步骤

1. **验证 descriptor set 存在**
```powershell
Test-Path g:\test\goci-const-check\pb\descriptor\all.protos.pb
```

2. **编译分析器**
```powershell
cd g:\test\goci-const-check
go install ./cmd/immitablecheck
```

3. **运行单元测试**
```powershell
cd g:\test\goci-const-check\cmd\immitablecheck
go test -v
```

4. **在 main.go 上运行分析**
```powershell
cd g:\test\goci-const-check
immitablecheck ./main.go 2>&1
```

5. **验证输出**
- 应该看到 2 个错误（第 9 行和第 11 行）
- 不应该看到任何关于第 10 行（Name 字段）的错误

## 调试要点

| 项目 | 检查方式 | 预期结果 |
|-----|--------|--------|
| Descriptor Set | `Test-Path pb/descriptor/all.protos.pb` | $true |
| Proto 加载 | `go test -run TestLoadDescriptorSet` | PASS |
| 类型转换 | `go test -run TestSnakeToCamelCase` | 5/5 ✓ |
| 分析执行 | `immitablecheck ./main.go` | 2 errors |
| 正确的误判 | Line 10 无错误 | 无输出 |

## 文件位置

- 分析器代码: `g:\test\goci-const-check\cmd\immitablecheck\main.go`
- 测试代码: `g:\test\goci-const-check\cmd\immitablecheck\debug_test.go`
- 测试目标: `g:\test\goci-const-check\main.go`
- Proto 定义: `g:\test\goci-const-check\proto\person.proto`
- Descriptor Set: `g:\test\goci-const-check\pb\descriptor\all.protos.pb`
