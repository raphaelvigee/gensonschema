package gen

import (
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
	fields  []StructField
	methods []string

	asGetters map[string]struct{}
}

func (s *structType) MakeStore(typ, jsonDefault string) {
	s.AddField(StructField{
		Name: "_path",
		Type: "string",
	})
	s.AddField(StructField{
		Name: "_json",
		Type: "*[]byte",
	})

	s.methods = append(s.methods, fmt.Sprintf(`
	func (r %[1]v) MarshalJSON() ([]byte, error) {
		return r.json(), nil
	}
	func (r *%[1]v) UnmarshalJSON(b []byte) error {
		if r._json != nil {
			njson, err := sjson.SetRawBytes(r.json(), r.path(), b)
			if err != nil {
				return err
			}
			r.setJson(njson)
			return nil
		}

		bcopy := make([]byte, len(b))
		copy(bcopy, b)

		*r = %[1]v{_json: &bcopy}
		return nil
	}
	func (r %[1]v) json() []byte {
		if r._json == nil {
			return []byte(%[2]v)
		}
		
		return *r._json
	}
	func (r %[1]v) path() string {
		return r._path
	}
	func (r %[1]v) setJson(v []byte) {
		*r._json = v
	}
	func (r *%[1]v) ensureJson() {
		if r._json != nil {
			return
		}

		b := r.json()
		r._json = &b
	}
	func (r %[1]v) result() gjson.Result {
		if r._path == "" {
			return gjson.ParseBytes(r.json())
		}
		return gjson.GetBytes(r.json(), r.path())
	}
	func (r %[1]v) Exists() bool {
		return r.result().Exists()
	}
	func (r %[1]v) Delete() error {
		res, err := sjson.DeleteBytes(r.json(), r.path())
		if err != nil {
			return err
		}
		r.setJson(res)
		return nil
	}
	`, s.name, strconv.Quote(jsonDefault)))

	if typ != "" {
		s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Value() %v {
			res := r.result()
			var v %v
			_ = json.Unmarshal([]byte(res.Raw), &v)
			return v
		}

		func (r *%v) Set(v %v) error {
			r.ensureJson()
			if r._path == "" {
				b, err := json.Marshal(v)
				if err != nil { return err }
				r.setJson(b)
				return nil	
			}
			res, err := sjson.SetBytes(r.json(), r.path(), v)
			if err != nil {
				return err
			}
			r.setJson(res)
			return nil
		}
		`, s.name, typ, typ, s.name, typ))
	} else {
		s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Set(v %v) error {
			if r._path == "" {
				r.setJson(v.json())
				return nil	
			}
			res, err := sjson.SetRawBytes(r.json(), r.path(), v.json())
			if err != nil {
				return err
			}
			r.setJson(res)
			return nil
		}
		`, s.name, s.name))
	}
}

func (s *structType) AddField(f StructField) {
	if f.IsEmbedded() {
		for _, field := range s.fields {
			if field.IsEmbedded() && field.Type == f.Type {
				return
			}
		}
	}

	s.fields = append(s.fields, f)
}

func (s *structType) AddGetter(name, path, styp string) {
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) Get%v() *%v {
			r.ensureJson()
			return &%v{ 
				_path: pathJoin(r._path, "%v"),
				_json: r._json,
			}
		}
	`, s.name, name, styp, styp, path))
}

func (s *structType) AddIndexGetter(styp string) {
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) At(i int) *%v {
			r.ensureJson()
			return &%v{ 
				_path: pathJoin(r._path, fmt.Sprint(i)),
				_json: r._json,
			}
		}
	`, s.name, styp, styp))
	s.methods = append(s.methods, fmt.Sprintf(`
		func (r %v) Len() int {
			res := r.result()
			if !res.IsArray() { return 0 }
			return int(res.Get("#").Value().(float64))
		}
	`, s.name))
}

func (s *structType) AddAsGetter(name, styp string) {
	if name == "" {
		name = styp
	}
	name = strings.Title(name)

	if s.asGetters == nil {
		s.asGetters = map[string]struct{}{}
	}
	if _, ok := s.asGetters[name]; ok {
		return
	}
	s.asGetters[name] = struct{}{}

	s.methods = append(s.methods, fmt.Sprintf(`
		func (r *%v) As%v() *%v {
			r.ensureJson()
			return &%v{ 
				_path: r._path,
				_json: r._json,
			}
		}
	`, s.name, name, styp, styp))
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
	goType, jdefault := "", ""
	switch sch.Types[0] {
	case "string":
		goType = "string"
		jdefault = `""`
	case "integer":
		goType = "int64"
		jdefault = `0`
	case "number":
		goType = "float64"
		jdefault = `0.0`
	case "boolean":
		goType = "bool"
		jdefault = `false`
	default:
		return "", "", fmt.Errorf("unsupported type %s", sch.Types[0])
	}

	return g.genStoreForTypeName(goType, jdefault)
}

func (g *generator) genStoreForTypeName(goType, jtype string) (string, string, error) {
	storeType := &structType{name: g.typeToName(goType)}
	storeType.MakeStore(goType, jtype)

	return g.registerStruct(storeType), goType, nil
}

func (g *generator) genTypeFor(name string, sch *jsonschema.Schema) (string, string, error) {
	if name == "" {
		name = g.schemaToTypeName(sch)
	}

	if sch.Ref != nil {
		return g.genTypeFor(name, sch.Ref)
	}

	if sch.OneOf != nil {
		name := "OneOf" + name
		return g.asTypeFor(name, sch.OneOf)
	}

	if sch.AnyOf != nil {
		name := "AnyOf" + name
		return g.asTypeFor(name, sch.AnyOf)
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

		itemStyp, itemDtype, err := g.genTypeFor(name, itemSchema)
		if err != nil {
			return "", "", err
		}

		styp := &structType{name: name}
		styp.MakeStore("", `[]`)

		styp.AddIndexGetter(itemStyp)

		g.registerStruct(styp)

		if itemDtype == "" {
			return styp.name, "", nil
		}

		return styp.name, "[]" + itemDtype, nil
	}

	if sch.Types[0] != "object" {
		return g.genTypeForPrimitive(sch)
	}

	return g.buildTypeFor(sch.Location, name, []*jsonschema.Schema{sch})
}

func (g *generator) buildTypeFor(location, name string, schs []*jsonschema.Schema) (string, string, error) {
	allObjects := true
	for _, sch := range schs {
		if len(sch.Types) < 1 || sch.Types[0] != "object" {
			allObjects = false
			break
		}
	}
	jsonDefault := ""
	if allObjects {
		jsonDefault = `{}`
	}
	storeType := &structType{name: name}
	storeType.MakeStore("", jsonDefault)

	for _, sch := range schs {
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

			name := g.pathToFieldName(propName)

			storeType.AddGetter(name, propName, styp)
		}
	}

	return g.registerStruct(storeType), "", nil
}

func (g *generator) registerStruct(t *structType) string {
	if t.name == "" {
		panic("struct does not have a name")
	}
	g.types[t.name] = t

	return t.name
}

func (g *generator) pathToFieldName(s string) string {
	return strings.Title(s)
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
	_, name, _ := strings.Cut(s, "#/")
	if name == "" {
		s = strings.TrimSuffix(s, "#")
		s = filepath.Base(s)
		name = strings.ReplaceAll(s, filepath.Ext(s), "")
	}

	var parts []string
	for _, part := range reg.Split(name, -1) {
		if part == "properties" || part == "items" {
			continue
		}
		parts = append(parts, strings.Title(part))
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

var reg = regexp.MustCompile("[^a-zA-Z0-9]+")

func (g *generator) titleToName(s string) string {
	parts := reg.Split(s, -1)
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	return g.sanitizeSymbol(strings.Join(parts, ""))
}

func (g *generator) typeToName(s string) string {
	return g.titleToName(s)
}

func (g *generator) asTypeFor(name string, schs []*jsonschema.Schema) (string, string, error) {
	stype := &structType{name: name}
	stype.MakeStore("", "")

	for _, sch := range schs {
		chStype, chDtype, err := g.genTypeFor("", sch)
		if err != nil {
			return "", "", err
		}

		if chStype == "" {
			continue
		}

		stype.AddAsGetter(chDtype, chStype)
	}

	return g.registerStruct(stype), "", nil

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

	return g.buildTypeFor(sch.Location, name, schs)
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
	g.imports["fmt"] = struct{}{}

	w := writer{g.config.Out}
	w.w.Write([]byte("// Code generated by github.com/raphaelvigee/gensonschema. DO NOT EDIT!\n"))
	w.Package(g.config.PackageName)

	for importStr := range g.imports {
		w.Import("", importStr)
	}

	w.w.Write([]byte(`
	func pathJoin(p1, p2 string) string {
		if p1 == "" {
			return p2
		}

		return p1+"."+p2
	}
	`))

	for iname, method := range g.interfaces {
		w.Interface(iname, method)
	}

	typeNames := maps.Keys(g.types)
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		str := g.types[typeName]
		w.StructStart(str.name)
		for _, field := range str.fields {
			w.Field(field.Name, field.Type, field.Tags)
		}
		w.StructEnd()

		sort.Strings(str.methods)

		for _, method := range str.methods {
			w.w.Write([]byte(method))
		}
	}

	return nil
}
