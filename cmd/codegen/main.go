package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// TL schema parser + Go code generator for TDLib td_api.tl

type Field struct {
	Name    string
	TLType  string
	GoName  string
	GoType  string
	JSONTag string
	Comment string
}

type TypeDef struct {
	Name       string // TL name, e.g. authenticationCodeTypeSms
	GoName     string // Go name, e.g. AuthenticationCodeTypeSms
	ParentType string // TL parent, e.g. AuthenticationCodeType
	Fields     []Field
	Comment    string
	IsFunc     bool   // true if this is a function (method)
	ReturnType string // TL return type for functions
	GoReturn   string // Go return type
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: codegen <td_api.tl> <output_dir>\n")
		os.Exit(1)
	}
	tlPath := os.Args[1]
	outDir := os.Args[2]

	types, funcs, err := parseTL(tlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed %d types, %d functions\n", len(types), len(funcs))

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir error: %v\n", err)
		os.Exit(1)
	}

	if err := generateTypes(types, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "generate types error: %v\n", err)
		os.Exit(1)
	}

	if err := generateFunctions(funcs, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "generate functions error: %v\n", err)
		os.Exit(1)
	}

	if err := generateRegistry(types, funcs, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "generate registry error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Code generation complete.")
}

func parseTL(path string) (types []TypeDef, funcs []TypeDef, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var commentLines []string
	inFunctions := false
	classRegex := regexp.MustCompile(`^//@class\s+(\w+)\s+@description\s+(.*)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "---functions---" {
			inFunctions = true
			continue
		}
		if line == "---types---" {
			inFunctions = false
			continue
		}

		// skip empty lines
		if line == "" {
			commentLines = nil
			continue
		}

		// check for @class (abstract type)
		if m := classRegex.FindStringSubmatch(line); m != nil {
			className := m[1]
			classComment := m[2]
			types = append(types, TypeDef{
				Name:       className,
				GoName:     toGoName(className),
				ParentType: className,
				Comment:    classComment,
			})
			commentLines = nil
			continue
		}

		// collect comments
		if strings.HasPrefix(line, "//@") || strings.HasPrefix(line, "//") {
			commentLines = append(commentLines, line)
			continue
		}

		// skip vector/double/string/int primitives
		if strings.HasPrefix(line, "double ") || strings.HasPrefix(line, "string ") ||
			strings.HasPrefix(line, "int32") || strings.HasPrefix(line, "int53") ||
			strings.HasPrefix(line, "int64") || strings.HasPrefix(line, "bytes") ||
			strings.HasPrefix(line, "boolFalse") || strings.HasPrefix(line, "boolTrue") ||
			strings.HasPrefix(line, "vector ") {
			commentLines = nil
			continue
		}

		// parse type/function definition
		td, ok := parseDefinition(line, commentLines, inFunctions)
		if ok {
			if inFunctions {
				funcs = append(funcs, td)
			} else {
				types = append(types, td)
			}
		}
		commentLines = nil
	}

	return types, funcs, scanner.Err()
}

var defRegex = regexp.MustCompile(`^(\w+)\s*(.*?)\s*=\s*(\w+)\s*;$`)

func parseDefinition(line string, comments []string, isFunc bool) (TypeDef, bool) {
	m := defRegex.FindStringSubmatch(line)
	if m == nil {
		return TypeDef{}, false
	}

	name := m[1]
	fieldsStr := strings.TrimSpace(m[2])
	returnType := m[3]

	td := TypeDef{
		Name:       name,
		GoName:     toGoName(name),
		ParentType: returnType,
		IsFunc:     isFunc,
		ReturnType: returnType,
		GoReturn:   toGoName(returnType),
		Comment:    extractMainComment(comments),
	}

	if fieldsStr != "" {
		td.Fields = parseFields(fieldsStr, comments)
	}

	return td, true
}

func parseFields(fieldsStr string, comments []string) []Field {
	parts := strings.Fields(fieldsStr)
	var fields []Field
	for _, p := range parts {
		idx := strings.Index(p, ":")
		if idx < 0 {
			continue
		}
		fname := p[:idx]
		ftype := p[idx+1:]
		goName := toGoName(fname)
		goType := tlTypeToGo(ftype)
		fields = append(fields, Field{
			Name:    fname,
			TLType:  ftype,
			GoName:  goName,
			GoType:  goType,
			JSONTag: toJSONTag(fname),
			Comment: extractFieldComment(comments, fname),
		})
	}
	return fields
}

func toGoName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "_")
	var result strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	s := result.String()
	// fix common acronyms
	s = strings.ReplaceAll(s, "Id", "ID")
	s = strings.ReplaceAll(s, "Url", "URL")
	s = strings.ReplaceAll(s, "Http", "HTTP")
	s = strings.ReplaceAll(s, "Ip", "IP")
	s = strings.ReplaceAll(s, "Api", "API")
	return s
}

func toJSONTag(name string) string {
	return name
}

func tlTypeToGo(tlType string) string {
	switch tlType {
	case "Bool":
		return "bool"
	case "int32":
		return "int32"
	case "int53":
		return "int64"
	case "int64":
		return "JSONInt64"
	case "double":
		return "float64"
	case "string":
		return "string"
	case "bytes":
		return "[]byte"
	case "Ok":
		return "*Ok"
	case "Error":
		return "*Error"
	}

	// vector<T>
	if strings.HasPrefix(tlType, "vector<") && strings.HasSuffix(tlType, ">") {
		inner := tlType[7 : len(tlType)-1]
		goInner := tlTypeToGo(inner)
		return "[]" + goInner
	}

	// reference type
	return "*" + toGoName(tlType)
}

func extractMainComment(comments []string) string {
	for _, c := range comments {
		if strings.HasPrefix(c, "//@description ") {
			return strings.TrimPrefix(c, "//@description ")
		}
	}
	return ""
}

func extractFieldComment(comments []string, fieldName string) string {
	tag := "//@" + fieldName + " "
	for _, c := range comments {
		if strings.HasPrefix(c, tag) {
			return strings.TrimPrefix(c, tag)
		}
	}
	return ""
}

// --- Code generation ---

const typesTemplate = `// Code generated by go-tdlib codegen. DO NOT EDIT.
package tdapi

import "encoding/json"

// JSONInt64 represents a TDLib int64 that is serialized as a JSON string.
type JSONInt64 int64

func (i JSONInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%d", int64(i)))
}

func (i *JSONInt64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// try as number
		var n int64
		if err2 := json.Unmarshal(data, &n); err2 != nil {
			return err
		}
		*i = JSONInt64(n)
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*i = JSONInt64(n)
	return nil
}

// TDLibClass is the interface for all TDLib types.
type TDLibClass interface {
	GetType() string
}

{{range .}}
{{if .Comment}}// {{.GoName}} — {{.Comment}}{{end}}
type {{.GoName}} struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}"` + "`" + `{{if .Comment}} // {{.Comment}}{{end}}
{{- end}}
}

func (t *{{.GoName}}) GetType() string { return "{{.Name}}" }

{{end}}
`

const funcsTemplate = `// Code generated by go-tdlib codegen. DO NOT EDIT.
package tdapi

{{range .}}
{{if .Comment}}// {{.GoName}} — {{.Comment}}{{end}}
type {{.GoName}}Request struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}"` + "`" + `{{if .Comment}} // {{.Comment}}{{end}}
{{- end}}
}

func (r *{{.GoName}}Request) GetType() string { return "{{.Name}}" }

{{end}}
`

const registryTemplate = `// Code generated by go-tdlib codegen. DO NOT EDIT.
package tdapi

import "encoding/json"

// TypeMap maps TDLib type names to Go constructors.
var TypeMap = map[string]func() TDLibClass{
{{- range .Types}}
	"{{.Name}}": func() TDLibClass { return &{{.GoName}}{} },
{{- end}}
}

// UnmarshalType unmarshals a TDLib JSON response into the correct Go type.
func UnmarshalType(data []byte) (TDLibClass, error) {
	var meta struct {
		Type string ` + "`" + `json:"@type"` + "`" + `
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	factory, ok := TypeMap[meta.Type]
	if !ok {
		return nil, fmt.Errorf("unknown type: %s", meta.Type)
	}
	obj := factory()
	if err := json.Unmarshal(data, obj); err != nil {
		return nil, err
	}
	return obj, nil
}
`

type registryData struct {
	Types []TypeDef
	Funcs []TypeDef
}

func generateTypes(types []TypeDef, outDir string) error {
	// split into chunks of ~200 types per file
	chunkSize := 200
	for i := 0; i < len(types); i += chunkSize {
		end := i + chunkSize
		if end > len(types) {
			end = len(types)
		}
		chunk := types[i:end]
		filename := fmt.Sprintf("types_%d.go", i/chunkSize)
		if err := writeTemplate(filepath.Join(outDir, filename), typesTemplate, chunk, i == 0); err != nil {
			return err
		}
	}
	return nil
}

func generateFunctions(funcs []TypeDef, outDir string) error {
	chunkSize := 200
	for i := 0; i < len(funcs); i += chunkSize {
		end := i + chunkSize
		if end > len(funcs) {
			end = len(funcs)
		}
		chunk := funcs[i:end]
		filename := fmt.Sprintf("functions_%d.go", i/chunkSize)
		if err := writeTemplate(filepath.Join(outDir, filename), funcsTemplate, chunk, false); err != nil {
			return err
		}
	}
	return nil
}

func generateRegistry(types []TypeDef, funcs []TypeDef, outDir string) error {
	f, err := os.Create(filepath.Join(outDir, "registry.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, `// Code generated by go-tdlib codegen. DO NOT EDIT.`)
	fmt.Fprintln(f, `package tdapi`)
	fmt.Fprintln(f)
	fmt.Fprintln(f, `import (`)
	fmt.Fprintln(f, `	"encoding/json"`)
	fmt.Fprintln(f, `	"fmt"`)
	fmt.Fprintln(f, `)`)
	fmt.Fprintln(f)
	fmt.Fprintln(f, `// TypeMap maps TDLib type names to Go constructors.`)
	fmt.Fprintln(f, `var TypeMap = map[string]func() TDLibClass{`)
	for _, t := range types {
		fmt.Fprintf(f, "\t%q: func() TDLibClass { return &%s{} },\n", t.Name, t.GoName)
	}
	fmt.Fprintln(f, `}`)
	fmt.Fprintln(f)
	fmt.Fprintln(f, `// UnmarshalType unmarshals a TDLib JSON response into the correct Go type.`)
	fmt.Fprintln(f, `func UnmarshalType(data []byte) (TDLibClass, error) {`)
	fmt.Fprintln(f, `	var meta struct {`)
	fmt.Fprintln(f, "		Type string `json:\"@type\"`")
	fmt.Fprintln(f, `	}`)
	fmt.Fprintln(f, `	if err := json.Unmarshal(data, &meta); err != nil {`)
	fmt.Fprintln(f, `		return nil, err`)
	fmt.Fprintln(f, `	}`)
	fmt.Fprintln(f, `	factory, ok := TypeMap[meta.Type]`)
	fmt.Fprintln(f, `	if !ok {`)
	fmt.Fprintln(f, `		return nil, fmt.Errorf("unknown type: %s", meta.Type)`)
	fmt.Fprintln(f, `	}`)
	fmt.Fprintln(f, `	obj := factory()`)
	fmt.Fprintln(f, `	if err := json.Unmarshal(data, obj); err != nil {`)
	fmt.Fprintln(f, `		return nil, err`)
	fmt.Fprintln(f, `	}`)
	fmt.Fprintln(f, `	return obj, nil`)
	fmt.Fprintln(f, `}`)

	return nil
}

func writeTemplate(path string, tmplStr string, data interface{}, includeHeader bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, `// Code generated by go-tdlib codegen. DO NOT EDIT.`)
	fmt.Fprintln(f, `package tdapi`)
	fmt.Fprintln(f)

	if includeHeader {
		fmt.Fprintln(f, `import (`)
		fmt.Fprintln(f, `	"encoding/json"`)
		fmt.Fprintln(f, `	"fmt"`)
		fmt.Fprintln(f, `	"strconv"`)
		fmt.Fprintln(f, `)`)
		fmt.Fprintln(f)
		fmt.Fprintln(f, `var _ = json.Marshal`)
		fmt.Fprintln(f, `var _ = fmt.Sprintf`)
		fmt.Fprintln(f, `var _ = strconv.ParseInt`)
		fmt.Fprintln(f)
		fmt.Fprintln(f, `// JSONInt64 represents a TDLib int64 that is serialized as a JSON string.`)
		fmt.Fprintln(f, `type JSONInt64 int64`)
		fmt.Fprintln(f)
		fmt.Fprintln(f, `func (i JSONInt64) MarshalJSON() ([]byte, error) {`)
		fmt.Fprintln(f, `	return json.Marshal(fmt.Sprintf("%d", int64(i)))`)
		fmt.Fprintln(f, `}`)
		fmt.Fprintln(f)
		fmt.Fprintln(f, `func (i *JSONInt64) UnmarshalJSON(data []byte) error {`)
		fmt.Fprintln(f, `	var s string`)
		fmt.Fprintln(f, `	if err := json.Unmarshal(data, &s); err != nil {`)
		fmt.Fprintln(f, `		var n int64`)
		fmt.Fprintln(f, `		if err2 := json.Unmarshal(data, &n); err2 != nil {`)
		fmt.Fprintln(f, `			return err`)
		fmt.Fprintln(f, `		}`)
		fmt.Fprintln(f, `		*i = JSONInt64(n)`)
		fmt.Fprintln(f, `		return nil`)
		fmt.Fprintln(f, `	}`)
		fmt.Fprintln(f, `	n, err := strconv.ParseInt(s, 10, 64)`)
		fmt.Fprintln(f, `	if err != nil {`)
		fmt.Fprintln(f, `		return err`)
		fmt.Fprintln(f, `	}`)
		fmt.Fprintln(f, `	*i = JSONInt64(n)`)
		fmt.Fprintln(f, `	return nil`)
		fmt.Fprintln(f, `}`)
		fmt.Fprintln(f)
		fmt.Fprintln(f, `// TDLibClass is the interface for all TDLib types.`)
		fmt.Fprintln(f, `type TDLibClass interface {`)
		fmt.Fprintln(f, `	GetType() string`)
		fmt.Fprintln(f, `}`)
		fmt.Fprintln(f)
	}

	// write type/function structs
	isFunc := strings.Contains(tmplStr, "Request")
	items := data.([]TypeDef)
	for _, item := range items {
		if item.Comment != "" {
			if isFunc {
				fmt.Fprintf(f, "// %sRequest — %s\n", item.GoName, item.Comment)
			} else {
				fmt.Fprintf(f, "// %s — %s\n", item.GoName, item.Comment)
			}
		}
		if isFunc {
			fmt.Fprintf(f, "type %sRequest struct {\n", item.GoName)
		} else {
			fmt.Fprintf(f, "type %s struct {\n", item.GoName)
		}
		for _, field := range item.Fields {
			comment := ""
			if field.Comment != "" {
				comment = " // " + field.Comment
			}
			fmt.Fprintf(f, "\t%s %s `json:\"%s\"`%s\n", field.GoName, field.GoType, field.JSONTag, comment)
		}
		fmt.Fprintln(f, "}")
		fmt.Fprintln(f)
		if isFunc {
			fmt.Fprintf(f, "func (r *%sRequest) GetType() string { return %q }\n\n", item.GoName, item.Name)
		} else {
			fmt.Fprintf(f, "func (t *%s) GetType() string { return %q }\n\n", item.GoName, item.Name)
		}
	}

	return nil
}
