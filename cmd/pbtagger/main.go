// cmd/pbtagger/main.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	descPath = flag.String("desc", "out/all.protos.pb", "FileDescriptorSet path")
	pbDir    = flag.String("pbdir", "pb", "directory of generated pb .go files")
)

func main() {
	flag.Parse()

	b, err := ioutil.ReadFile(*descPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read desc: %v\n", err)
		os.Exit(2)
	}
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(b, &fds); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal descriptor set: %v\n", err)
		os.Exit(2)
	}

	// Map of fully-qualified message name -> set of field names that are immutable
	// message name example: "example.Person"
	imm := map[string]map[string]bool{}

	for _, fd := range fds.File {
		pkg := fd.GetPackage()
		for _, md := range fd.MessageType {
			msgName := md.GetName()
			fullMsg := pkg
			if fullMsg != "" {
				fullMsg = fullMsg + "." + msgName
			} else {
				fullMsg = msgName
			}
			for _, fld := range md.Field {
				opts := fld.GetOptions()
				if opts == nil {
					continue
				}
				// The custom option number we used: 51234 ; but we should check via protobuf reflection?
				// Simpler: the descriptor encodes uninterpreted_options when option is unknown.
				// We'll check UninterpretedOption for name "immutable"
				if opts.GetImmutable_() { // Not available — can't rely on generated helpers here.
					// NOTE: this line won't compile; we'll instead inspect UninterpretedOption.
				}
			}
		}
	}

	// Because we don't have generated helper for custom extension in descriptorpb,
	// inspect UninterpretedOption entries.
	for _, fd := range fds.File {
		pkg := fd.GetPackage()
		for _, md := range fd.MessageType {
			msgName := md.GetName()
			fullMsg := pkg
			if fullMsg != "" {
				fullMsg = fullMsg + "." + msgName
			} else {
				fullMsg = msgName
			}
			for _, fld := range md.Field {
				hasImmutable := false
				for _, uo := range fld.GetOptions().GetUninterpretedOption() {
					// uninterpreted option contains a name_parts slice, last part is the option name
					// name example: "protooptions.immutable"
					parts := []string{}
					for _, np := range uo.GetName() {
						parts = append(parts, np.GetNamePart())
					}
					n := strings.Join(parts, ".")
					// For boolean option set to true, the "identifier_value" may be "true" or a positive string_value; also there is string_value or positive/negative/identifier
					if strings.HasSuffix(n, "immutable") {
						// check its value representation: identifier_value or positive_int_value etc.
						if uo.GetIdentifierValue() == "true" || uo.GetStringValue() != nil && string(uo.GetStringValue()) == "true" {
							hasImmutable = true
						}
					}
				}
				if hasImmutable {
					if imm[fullMsg] == nil {
						imm[fullMsg] = map[string]bool{}
					}
					imm[fullMsg][fld.GetName()] = true
				}
			}
		}
	}

	// DEBUG
	// fmt.Printf("immutable map: %#v\n", imm)

	// Now iterate pb go files and add comments to struct fields
	fset := token.NewFileSet()
	err = filepath.Walk(*pbDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}
		modified := false

		ast.Inspect(f, func(n ast.Node) bool {
			// find type declarations
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			// The generated protobuf struct type name is same as message name (CamelCase).
			// Need to map package+Type => check messages in imm map for match.
			typeName := ts.Name.Name // e.g., "Person"
			// There is no direct package name in descriptor (full package + message).
			// We'll try matching by suffix: any entry in imm whose last part equals typeName
			for fullMsg, fields := range imm {
				if !strings.HasSuffix(fullMsg, "."+typeName) && fullMsg != typeName {
					continue
				}
				// Found candidate: iterate fields in AST struct and mark ones whose JSON/protobuf name matches.
				for _, field := range st.Fields.List {
					// get AST field name (Go exported name)
					if len(field.Names) == 0 {
						continue
					}
					goName := field.Names[0].Name // e.g., Id, Name, Age
					// Convert goName to proto field name: protoc uses CamelCase <-> snake_case mapping.
					// Easiest: check tag `protobuf` or `json` present in struct tag to find original proto name.
					protoName := ""
					if field.Tag != nil {
						tag := strings.Trim(field.Tag.Value, "`")
						// search for `protobuf:"...name=xxx..."`
						// or `json:"name,omitempty"`
						// Try json first
						for _, part := range strings.Split(tag, " ") {
							if strings.HasPrefix(part, "json:") {
								v := strings.Trim(strings.TrimPrefix(part, "json:"), "\"")
								// json tag might be "id,omitempty" -> take before comma
								protoName = strings.Split(v, ",")[0]
								break
							}
						}
						if protoName == "" {
							// try protobuf tag
							for _, part := range strings.Split(tag, " ") {
								if strings.HasPrefix(part, "protobuf:") {
									v := strings.Trim(strings.TrimPrefix(part, "protobuf:"), "\"")
									// format may include name=xxx
									for _, seg := range strings.Split(v, ",") {
										if strings.HasPrefix(seg, "name=") {
											protoName = strings.TrimPrefix(seg, "name=")
											break
										}
									}
								}
							}
						}
					}
					if protoName == "" {
						// fallback: guess by lowercasing first letter (not reliable). Skip if unknown.
						continue
					}
					if fields[protoName] {
						// add comment `// immutable` if not already present
						exists := false
						if field.Comment != nil {
							for _, c := range field.Comment.List {
								if strings.Contains(strings.ToLower(c.Text), "immutable") {
									exists = true
								}
							}
						}
						if !exists {
							if field.Comment == nil {
								field.Comment = &ast.CommentGroup{}
							}
							field.Comment.List = append(field.Comment.List, &ast.Comment{Text: "// immutable"})
							modified = true
						}
					}
				}
			}
			return true
		})

		if modified {
			var out bytes.Buffer
			if err := printer.Fprint(&out, fset, f); err != nil {
				return err
			}
			if err := ioutil.WriteFile(path, out.Bytes(), 0644); err != nil {
				return err
			}
			fmt.Printf("patched %s\n", path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk pb dir: %v\n", err)
		os.Exit(2)
	}
}
