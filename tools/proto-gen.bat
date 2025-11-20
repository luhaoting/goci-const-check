:: 生成 Go 代码
protoc.exe --proto_path=../proto --go_out=paths=source_relative:../pb ../proto/*.proto

:: 生成 descriptor set（包含自定义 options）
protoc.exe --proto_path=../proto --include_imports --descriptor_set_out=../pb/descriptor/all.protos.pb ../proto/*.proto