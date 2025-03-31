package gen

import (
	_ "embed"
	"errors"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"golang.org/x/exp/maps"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type structType struct {
	name    string
	methods []string

	asGetters map[string]struct{}
}

var wellKnownTypes = map[string]string{
	"int64":   "Int()",
	"uint64":  "Uint()",
	"float64": "Float()",
	"bool":    "Bool()",
	"string":  "String()",
}

func (s *structType) MakeStore(typ, defaultJson string) {
	s.MakeStoreWith(typ, defaultJson, false)
}

func (s *structType) MakeStoreWith(typ, defaultJson string, mergeSet bool) {
	s.methods = append(s.methods, fmt.Sprintf(`
	func (r %[1]v) Copy() *%[1]v {
			return &%[1]v{ 
				__node: r.copy(),
			}
	}
	`, s.name))

	s.methods = append(s.methods, fmt.Sprintf(`
	func (r %[1]v) WithSafe(safe bool) *%[1]v {
			return &%[1]v{ 
				__node: r.__node.withSafe(safe),
			}
	}
	`, s.name))

	s.methods = append(s.methods, fmt.Sprintf(`
	func (r %v) typeDefaultJson() []byte {
		return []byte("%v")
	}
	`, s.name, defaultJson))

	if typ != "" {
		if accessor, ok := wellKnownTypes[typ]; ok {
			if accessor == "String()" {
				s.methods = append(s.methods, fmt.Sprintf(`
				func (r %v) Value() %v {
					return node_value_string(r.__node)
				}`, s.name, typ))
			} else {
				s.methods = append(s.methods, fmt.Sprintf(`
				func (r %v) Value() %v {
					return r.result().%v
				}`, s.name, typ, accessor))
			}

		} else {
			s.methods = append(s.methods, fmt.Sprintf(`
			func (r %v) Value() %v {
				return node_value_struct[%v](r.__node)
			}`, s.name, typ, typ))
		}

		s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) Set(v %v) error {
			b, err := json.Marshal(v)
			if err != nil { return err }

			return r.setb(b)
		}
		`, s.name, typ))
	} else {
		if mergeSet {
			s.methods = append(s.methods, fmt.Sprintf(`
			func (r %v) Set(v *%v) error {
				incoming := v.currentJson()

				return r.setMerge(incoming)
			}
			`, s.name, s.name))
		} else {
			s.methods = append(s.methods, fmt.Sprintf(`
			func (r %v) Set(v *%v) error {
				incoming := v.currentJson()

				return r.set(incoming)
			}
			`, s.name, s.name))
		}
	}
}

func (s *structType) AddGetter(name, path, styp string) {
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) Get%v() *%v {
			return &%v{
				__node: node_get[%v, %v](&r.__node, %q),
			}
		}
	`, s.name, name, styp, styp, s.name, styp, path))
}

func (s *structType) AddIndexGetter(styp string, dtype string) {
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) At(i int) *%v {
			return &%v{
				__node: node_get[%v, %v](&r.__node, strconv.Itoa(i)),
			}
		}
	`, s.name, styp, styp, s.name, styp))
	appendType := "*" + styp
	if dtype != "" {
		appendType = dtype
	}
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) Append(v %v) error {
			return r.At(-1).Set(v)
		}
	`, s.name, appendType))
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Len() int {
			return node_array_len(r.__node)
		}
	`, s.name))
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Clear() error {
			return r.set("[]")
		}
	`, s.name))
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Range() func(yield func(int, *%v) bool) {
			return node_array_range(&r)
		}
	`, s.name, styp))
}

func (s *structType) AddAsGetter(name, styp string) {
	if name == "" {
		name = styp
	}
	name = titleCase(name)

	if s.asGetters == nil {
		s.asGetters = map[string]struct{}{}
	}
	if _, ok := s.asGetters[name]; ok {
		return
	}
	s.asGetters[name] = struct{}{}

	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) As%v() *%v {
			return &%v{ 
				__node: node_get_as[%v, %v](&r.__node),
			}
		}
	`, s.name, name, styp, styp, s.name, styp))
}

type StructField struct {
	Name string
	Type string
	Tags string
}

func (f StructField) IsEmbedded() bool {
	return f.Name == ""
}

type generator struct {
	config Config

	types      map[string]*structType
	interfaces map[string]string
	imports    map[string]struct{}
}

type writer struct {
	w io.Writer
}

func (w writer) Package(name string) {
	w.w.Write([]byte(fmt.Sprintf("package %v\n\n", name)))
}

func (w writer) Import(alias, imprt string) {
	w.w.Write([]byte("import "))

	if alias != "" {
		w.w.Write([]byte(alias))
	}
	w.w.Write([]byte(strconv.Quote(imprt)))
	w.w.Write([]byte("\n"))
}

func (w writer) StructStart(name string) {
	w.w.Write([]byte(fmt.Sprintf("type %v struct {\n", name)))
}

func (w writer) StructEnd() {
	w.w.Write([]byte("}\n\n"))
}

func (w writer) Field(name, typ, tags string) {
	w.w.Write([]byte(fmt.Sprintf("  %v %v", name, typ)))
	if tags != "" {
		w.w.Write([]byte(fmt.Sprintf(" `%v`", tags)))
	}
	w.w.Write([]byte("\n"))
}

func (w writer) Method(typ, name string) {
	w.w.Write([]byte(fmt.Sprintf("func (%v) %v() {}\n", typ, name)))
}

func (w writer) Interface(name, method string) {
	w.w.Write([]byte(fmt.Sprintf("type %v interface { %v() }\n\n", name, method)))
}

func (g *generator) gen() error {
	for _, sch := range g.config.Schemas {
		_, _, err := g.genTypeFor("", sch)
		if err != nil {
			return fmt.Errorf("%v: %w", sch.Location, err)
		}
	}

	return nil
}

func (g *generator) genTypeForPrimitive(sch *jsonschema.Schema) (string, string, error) {
	goType := ""
	switch sch.Types[0] {
	case "string":
		goType = "string"
	case "integer":
		goType = "int64"
	case "number":
		goType = "float64"
	case "boolean":
		goType = "bool"
	default:
		return "", "", fmt.Errorf("unsupported type %s", sch.Types[0])
	}

	return g.genStoreForTypeName(goType)
}

func (g *generator) genStoreForTypeName(goType string) (string, string, error) {
	storeType := &structType{name: g.typeToName(goType)}
	storeType.MakeStore(goType, "")

	return g.registerStruct(storeType), goType, nil
}

func (g *generator) genTypeFor(name string, sch *jsonschema.Schema) (string, string, error) {
	if name == "" {
		name = g.schemaToTypeName(sch)
	}
	if sch.Title != "" {
		name = g.titleToName(sch.Title)
	}

	if sch.Ref != nil {
		return g.genTypeFor(name, sch.Ref)
	}

	if sch.OneOf != nil {
		return g.asTypeFor(name, "OneOf", sch.OneOf)
	}

	if sch.AnyOf != nil {
		return g.asTypeFor(name, "AnyOf", sch.AnyOf)
	}

	if sch.AllOf != nil {
		return g.namedAllOfTypeFor(sch)
	}

	if len(sch.Types) == 0 {
		return "", "", errors.New("no types found in schema")
	}

	if sch.Types[0] == "array" {
		var itemSchema *jsonschema.Schema
		if sch.Items2020 != nil {
			itemSchema = sch.Items2020
		} else if sch, ok := sch.Items.(*jsonschema.Schema); ok {
			itemSchema = sch
		} else {
			return "", "", fmt.Errorf("unsupported items type %T", sch.Types[0])
		}

		itemStyp, itemDtype, err := g.genTypeFor("", itemSchema)
		if err != nil {
			return "", "", err
		}

		dType := ""
		if itemDtype != "" {
			dType = "[]" + itemDtype
		}

		styp := &structType{name: name}
		styp.MakeStore(dType, "[]")

		styp.AddIndexGetter(itemStyp, itemDtype)

		g.registerStruct(styp)

		return styp.name, dType, nil
	}

	if sch.Types[0] != "object" {
		return g.genTypeForPrimitive(sch)
	}

	return g.buildTypeFor(name, []*jsonschema.Schema{sch}, false)
}

func (g *generator) buildTypeFor(name string, schs []*jsonschema.Schema, mergeSet bool) (string, string, error) {
	storeType := &structType{name: name}

	commonGoType := ""

	for i, sch := range schs {
		if sch.OneOf != nil {
			prefix := g.titleToName(sch.Title)
			if prefix == "" {
				prefix = fmt.Sprintf("AllOf%vOneOf", i)
			}
			goType, err := g.asTypeForInto(storeType, prefix, sch.OneOf)
			if err != nil {
				return "", "", err
			}
			if len(schs) == 1 {
				commonGoType = goType
			}
			continue
		}

		if sch.AnyOf != nil {
			prefix := g.titleToName(sch.Title)
			if prefix == "" {
				prefix = fmt.Sprintf("AllOf%vAnyOf", i)
			}
			goType, err := g.asTypeForInto(storeType, prefix, sch.AnyOf)
			if err != nil {
				return "", "", err
			}
			if len(schs) == 1 {
				commonGoType = goType
			}
			continue
		}

		for propName, fieldSchema := range sch.Properties {
			if g.config.ShouldGenerate != nil {
				if !g.config.ShouldGenerate(fieldSchema) {
					continue
				}
			}

			styp, _, err := g.genTypeFor("", fieldSchema)
			if err != nil {
				return "", "", fmt.Errorf("%v: %w", fieldSchema.Location, err)
			}

			if styp == "" {
				continue
			}

			name := g.propertyToFieldName(propName)

			storeType.AddGetter(name, propName, styp)
		}
	}

	storeType.MakeStoreWith(commonGoType, g.goTypeToDefaultJson(commonGoType), mergeSet)

	return g.registerStruct(storeType), "", nil
}

func (g *generator) registerStruct(t *structType) string {
	if t.name == "" {
		panic("struct does not have a name")
	}
	g.types[t.name] = t

	return t.name
}

func (g *generator) propertyToFieldName(s string) string {
	return titleCase(s)
}

func (g *generator) schemaToTypeName(sch *jsonschema.Schema) string {
	if sch.Title != "" {
		return g.titleToName(sch.Title)
	}

	if sch.Ref != nil {
		return g.schemaToTypeName(sch.Ref)
	}

	return g.locationToTypeName(sch.Location)
}

func (g *generator) locationToTypeName(s string) string {
	url, name, _ := strings.Cut(s, "#/")
	if name == "" {
		s = strings.TrimSuffix(s, "#")
		s = filepath.Base(s)
		name = strings.ReplaceAll(s, filepath.Ext(s), "")
	}

	fileName := filepath.Base(url)
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	name = fileName + " " + name

	var parts []string
	for _, part := range reg.Split(name, -1) {
		if part == "" || part == "properties" {
			continue
		}
		parts = append(parts, titleCase(part))
	}
	return g.sanitizeSymbol(strings.Join(parts, ""))
}

func (g *generator) sanitizeSymbol(s string) string {
	if s == "" {
		panic("empty type name")
	}
	if !unicode.IsLetter(rune(s[0])) {
		s = "GEN_" + s
	}
	return s
}

var reg = regexp.MustCompile("[^a-zA-Z0-9_]+")

func titleCase(s string) string {
	if s == "" {
		return ""
	}

	if len(s) == 1 {
		return strings.ToUpper(s)
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

func (g *generator) titleToName(s string) string {
	if s == "" {
		return ""
	}

	parts := reg.Split(s, -1)
	for i, part := range parts {
		parts[i] = titleCase(part)
	}
	return g.sanitizeSymbol(strings.Join(parts, ""))
}

func (g *generator) typeToName(s string) string {
	return g.titleToName(s)
}

func (g *generator) compositeName(prefix string, i int, sch *jsonschema.Schema) string {
	if sch.Title != "" {
		return g.titleToName(sch.Title)
	}

	if sch.Ref != nil {
		return g.compositeName(prefix, i, sch.Ref)
	}

	return fmt.Sprintf("%v%v", prefix, i)
}

func (g *generator) asTypeForInto(stype *structType, prefix string, schs []*jsonschema.Schema) (string, error) {
	commonGoType, sameGoType := "", true
	for i, sch := range schs {
		chStype, goType, err := g.genTypeFor("", sch)
		if err != nil {
			return "", err
		}

		if chStype == "" {
			continue
		}

		fieldName := g.compositeName(prefix, i, sch)
		stype.AddAsGetter(fieldName, chStype)

		if sameGoType {
			if commonGoType == "" || commonGoType == goType {
				commonGoType = goType
			} else if commonGoType != goType {
				sameGoType = false
			}
		}
	}

	if sameGoType {
		return commonGoType, nil
	}

	return "", nil
}

func (g *generator) asTypeFor(name, prefix string, schs []*jsonschema.Schema) (string, string, error) {
	stype := &structType{name: name}

	commonGoType, err := g.asTypeForInto(stype, prefix, schs)
	if err != nil {
		return "", "", err
	}

	stype.MakeStore(commonGoType, g.goTypeToDefaultJson(commonGoType))

	return g.registerStruct(stype), commonGoType, nil
}

func (g *generator) goTypeToDefaultJson(goType string) string {
	if strings.HasPrefix(goType, "[]") {
		return "[]"
	}

	if goType == "" {
		return "{}"
	}

	return ""
}

func (g *generator) flattenAllOfs(sch *jsonschema.Schema) []*jsonschema.Schema {
	if sch.AllOf != nil {
		out := make([]*jsonschema.Schema, 0, len(sch.AllOf))
		for _, sch := range sch.AllOf {
			out = append(out, g.flattenAllOfs(sch)...)
		}
		return out
	} else if sch.Ref != nil {
		return g.flattenAllOfs(sch.Ref)
	} else {
		return []*jsonschema.Schema{sch}
	}
}

func (g *generator) namedAllOfTypeFor(sch *jsonschema.Schema) (string, string, error) {
	schs := g.flattenAllOfs(sch)

	name := g.schemaToTypeName(sch)

	return g.buildTypeFor(name, schs, true)
}

//go:embed utils.go.tpl
var utils string

func init() {
	utils = strings.ReplaceAll(utils, "package tpl", "")

	r := regexp.MustCompile(`(?m)^package .*$`)

	utils = r.ReplaceAllString(utils, "")
}

func Gen(config Config) error {
	g := &generator{
		config:     config,
		types:      map[string]*structType{},
		imports:    map[string]struct{}{},
		interfaces: map[string]string{},
	}

	if err := g.gen(); err != nil {
		return err
	}

	g.imports["github.com/tidwall/gjson"] = struct{}{}
	g.imports["github.com/tidwall/sjson"] = struct{}{}
	g.imports["encoding/json"] = struct{}{}
	g.imports["sync/atomic"] = struct{}{}
	g.imports["strconv"] = struct{}{}
	g.imports["fmt"] = struct{}{}

	w := writer{g.config.Out}
	w.Package(g.config.PackageName)

	w.w.Write([]byte("// Code generated by github.com/raphaelvigee/gensonschema. DO NOT EDIT!\n\n"))

	for importStr := range g.imports {
		w.Import("", importStr)
	}

	w.w.Write([]byte(utils))

	for iname, method := range g.interfaces {
		w.Interface(iname, method)
	}

	typeNames := maps.Keys(g.types)
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		str := g.types[typeName]
		w.StructStart(str.name)
		w.Field("", fmt.Sprintf("__node[%v]", str.name), "")
		w.StructEnd()

		sort.Strings(str.methods)

		for _, method := range str.methods {
			w.w.Write([]byte(method))
		}
	}

	return nil
}
