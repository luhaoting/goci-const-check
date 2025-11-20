# goci-const-check

使用 Go Analyzer 检测对 immutable 字段的修改

## 功能

该项目实现了一个 Go linter (Analyzer)，可以检测对以下方式标记的 immutable 字段的修改：

1. **Protobuf Option** - 在 proto 文件中使用自定义 option：
   ```proto
   syntax = "proto3";
   package example;
   
   import "google/protobuf/descriptor.proto";
   
   extend google.protobuf.FieldOptions {
     bool immutable = 59527;
   }
   
   message Person {
     int64 id = 1 [(example.immutable) = true];
     string name = 2;
     int32 age = 3 [(example.immutable) = true];
   }
   ```

2. **Go Tags** - 在 struct 字段上使用 tag：
   ```go
   type Person struct {
     Id   int64  `immutable:"true"`
     Name string
     Age  int32  `immutable:"1"`
   }
   ```

3. **Go Comments** - 在 struct 字段上使用注释：
   ```go
   type Person struct {
     Id   int64  // immutable
     Name string
     Age  int32  // immutable
   }
   ```

## 使用方法

### 1. 生成 Protobuf 代码和 Descriptor Set

运行 proto 生成脚本：

```bash
cd tools
.\proto-gen.bat
```

这会生成：
- Go 代码在 `pb/` 目录
- Descriptor set 在 `pb/descriptor/all.protos.pb`

### 2. 编译 Analyzer

```bash
go install ./cmd/immitablecheck
```

### 3. 运行 Analyzer

检测单个文件：

```bash
immitablecheck ./main.go
```

检测所有包：

```bash
immitablecheck ./...
```

## 检测示例

当你尝试修改被标记为 immutable 的字段时：

```go
func main() {
	t := &pb.Person{}
	t.Id = 12345    // ❌ 错误：assignment to immutable field Id
	t.Name = "Alice" // ✅ 正常
	t.Age = 30       // ❌ 错误：assignment to immutable field Age
}
```

Analyzer 会报告：

```
main.go:8:2: assignment to immutable field Id
main.go:10:2: assignment to immutable field Age
```

## 项目结构

```
├── cmd/immutablecheck/       # Analyzer 实现
│   └── main.go              # 核心 Analyzer 代码
├── pb/                       # Protobuf 生成的 Go 代码
│   ├── descriptor/
│   │   └── all.protos.pb    # Descriptor set 文件
│   ├── person.pb.go
│   └── school.pb.go
├── proto/                    # Protobuf 定义
│   ├── immutable_options.proto
│   ├── person.proto
│   └── school.proto
├── tools/
│   └── proto-gen.bat        # Proto 生成脚本
└── main.go                  # 示例代码
```

## Analyzer 工作原理

1. **加载 Descriptor Set**：从 `pb/descriptor/all.protos.pb` 读取 protobuf 定义
2. **解析 Immutable 字段**：识别在 proto 文件中标记为 immutable 的字段（option 59527）
3. **扫描 Go 代码**：在所有 Go struct 定义中检测 immutable 标记（tags 或注释）
4. **检测修改**：在代码中查找对 immutable 字段的赋值操作
5. **报告错误**：输出所有违反 immutable 规范的位置

## example
```
 immutablecheck ./main.go 2>&1
goci-const-check\main.go:9:2: assignment to immutable field Id
goci-const-check\main.go:11:2: assignment to immutable field Age
goci-const-check\main.go:24:2: assignment to immutable field Teachers
goci-const-check\main.go:33:2: assignment to immutable field Teachers
goci-const-check\main.go:35:2: modifying immutable field Teachers (map/slice index)
```