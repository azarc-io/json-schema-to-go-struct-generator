package inputs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/azarc-io/json-schema-to-go-struct-generator/pkg/utils"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Generator will produce structs from the JSON schema.
type Generator struct {
	schemas  []*Schema
	resolver *RefResolver
	Structs  map[string]*Struct
	Aliases  map[string]*Field
	// cache for reference types; k=url v=type
	refs        map[string]string
	anonCount   int
	structCache map[string][]*Struct
}

// New creates an instance of a generator which will produce structs.
func New(schemas ...*Schema) *Generator {
	return &Generator{
		schemas:     schemas,
		resolver:    NewRefResolver(schemas),
		Structs:     make(map[string]*Struct),
		Aliases:     make(map[string]*Field),
		refs:        make(map[string]string),
		structCache: make(map[string][]*Struct),
	}
}

// CreateTypes creates types from the JSON schemas, keyed by the golang name.
func (g *Generator) CreateTypes() (err error) {
	if err := g.resolver.Init(); err != nil {
		return err
	}

	// extract the types
	for _, schema := range g.schemas {
		name := g.getSchemaName("", schema)
		rootType, err := g.processSchema(name, schema)
		rootType.isRootType = true
		if err != nil {
			return err
		}
		// ugh: if it was anything but a struct the type will not be the name...
		primType, err := rootType.getPrimitiveTypeName()
		if err != nil {
			return err
		}

		if primType != "*"+name {
			a := NewField(
				name,
				"",
				rootType,
				false,
				[]string{schema.Description},
			)
			g.Aliases[a.Name] = a
		}
	}

	//consolidate structs and types
	return g.consolidateStructsAndTypes()
}

func (g *Generator) consolidateStructsAndTypes() error {
	var allStructs []*Struct
	for shortKey, cacheItem := range g.structCache {
		var structs []*Struct
		for _, s := range cacheItem {
			sCache := s
			structs = append(structs, sCache)
		}

		//run consolidation of structs
		for i := 0; i < len(structs); i++ {
			if structs[i] == nil {
				continue
			}
			for j := i + 1; j < len(structs); j++ {
				if structs[j] == nil {
					continue
				}
				if out := structs[i].unifiedWith(structs[j]); out != nil {
					structs[i] = nil
					structs[j] = out
					break
				}
			}
		}

		//filter out unused structs and add to full list
		_, hasAlias := g.Aliases[shortKey]
		count := 0
		for _, item := range structs {
			if item != nil {
				count++
				item.TypeInfo.hasSameNames = hasAlias || count > 1
				allStructs = append(allStructs, item)
			}
		}
	}

	//Add all structs to list
	for _, item := range allStructs {
		if err := g.addStruct(item); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) getStructKeys() []string {
	keys := make([]string, len(g.Structs))

	i := 0
	for k := range g.Structs {
		keys[i] = k
		i++
	}

	sort.Strings(keys)
	return keys
}

func (g *Generator) addStruct(item *Struct) error {
	//check if this can be aliased
	for _, key := range g.getStructKeys() {
		strct := g.Structs[key]

		//Only alias and merge root types
		if !strct.TypeInfo.isRootType || !item.TypeInfo.isRootType {
			continue
		}

		strctIsAlias := strct.TypeInfo.IsAlias()

		//if this can be merged, remove existing struct, create and add both as aliasFor of new Struct
		if merged := strct.unifiedWith(item); merged != nil {
			f1FieldName := strct.TypeInfo.String()
			f2FieldName := item.TypeInfo.String()

			//add aliasFor
			if !strctIsAlias {
				//only add as alias if this type is not an alias itself
				g.Aliases[f1FieldName] = NewField(f1FieldName, "", merged.TypeInfo, false, []string{strct.Description})
				merged.TypeInfo.AddAliasFor(f1FieldName)
			}
			g.Aliases[f2FieldName] = NewField(f2FieldName, "", merged.TypeInfo, false, []string{item.Description})
			merged.TypeInfo.AddAliasFor(f2FieldName)

			merged.TypeInfo.Name = strings.Join(merged.TypeInfo.aliasFor, "_")
			merged.Description = fmt.Sprintf("\nAliased for: %s", strings.Join(merged.TypeInfo.aliasFor, ", "))

			mergedFieldName := merged.TypeInfo.String()
			g.Structs[mergedFieldName] = merged
			delete(g.Structs, key)

			return nil
		}
	}

	//no aliasing has occured, add struct
	key := item.TypeInfo.String()
	if _, ok := g.Structs[key]; ok {
		return fmt.Errorf("struct with the name '%s' already exists", key)
	}
	g.Structs[key] = item
	return nil
}

// process a block of definitions
func (g *Generator) processDefinitions(schema *Schema) error {
	for key, subSchema := range schema.Definitions {
		if _, err := g.processSchema(GetGolangName(key), subSchema); err != nil {
			return err
		}
	}
	return nil
}

// process a reference string
func (g *Generator) processReference(schema *Schema) (*TypeInfo, error) {
	schemaPath := g.resolver.GetPath(schema)
	if schema.Reference == "" {
		return nil, errors.New("processReference empty reference: " + schemaPath)
	}
	refSchema, err := g.resolver.GetSchemaByReference(schema)
	if err != nil {
		return nil, errors.New("processReference: reference \"" + schema.Reference + "\" not found at \"" + schemaPath + "\"")
	}
	if refSchema.GeneratedType == nil {
		// reference is not resolved yet. Do that now.
		refSchemaName := g.getSchemaName("", refSchema)
		typ, err := g.processSchema(refSchemaName, refSchema)
		if err != nil {
			return nil, err
		}
		return typ, nil
	}
	return refSchema.GeneratedType, nil
}

// returns the type refered to by schema after resolving all dependencies
func (g *Generator) processSchema(schemaName string, schema *Schema) (typ *TypeInfo, err error) {
	if len(schema.Definitions) > 0 {
		err = g.processDefinitions(schema)
		if err != nil {
			return
		}
	}

	schema.FixMissingTypeValue()
	// if we have multiple schema types, the golang type will be interface{}
	typ = NewTypeInfo("interface{}", "interface", false, nil)
	types, isMultiType := schema.MultiType()
	if len(types) > 0 {
		for _, schemaType := range types {
			name := schemaName
			if isMultiType {
				name = name + "_" + schemaType
			}
			switch schemaType {
			case "object":
				rv, err := g.processObject(name, schema)
				if err != nil {
					return nil, err
				}
				if !isMultiType {
					return rv, nil
				}
			case "array":
				rv, err := g.processArray(name, schema)
				if err != nil {
					return nil, err
				}
				if !isMultiType {
					return rv, nil
				}
			default:
				rv := NewTypeInfo(schemaType, schemaType, false, nil)
				rv.Format = schema.Format
				if !isMultiType {
					return rv, nil
				}
			}
		}
	} else {
		if schema.Reference != "" {
			return g.processReference(schema)
		}
	}
	return // return interface{}
}

// name: name of this array, usually the js key
// schema: items element
func (g *Generator) processArray(name string, schema *Schema) (typ *TypeInfo, err error) {
	if schema.Items != nil {
		propName := name
		if !strings.HasSuffix(propName, "Items") {
			propName += "Items"
		}

		// subType: fallback name in case this array Contains inline object without a title
		subName := g.getSchemaName(propName, schema.Items)
		subTyp, err := g.processSchema(subName, schema.Items)
		if err != nil {
			return nil, err
		}
		finalType := NewTypeInfo("", "array", false, subTyp)
		if err != nil {
			return nil, err
		}
		// only alias root arrays
		if schema.Parent == nil {
			array := NewField(
				name,
				"",
				finalType,
				Contains(schema.Required, name),
				[]string{schema.Description},
			)
			g.Aliases[array.Name] = array
		}
		return finalType, nil
	}
	//type: []interface{}
	return NewTypeInfo("", "array", false, NewTypeInfo("", "interface", false, nil)), nil
}

// name: name of the struct (calculated by caller)
// schema: detail incl properties & child objects
// returns: generated type
func (g *Generator) processObject(name string, schema *Schema) (typ *TypeInfo, err error) {
	strct := &Struct{
		ID:          schema.ID(),
		TypeInfo:    NewTypeInfo(name, "object", true, nil),
		Description: schema.Description,
		Fields:      make(map[string]*Field, len(schema.Properties)),
	}
	// cache the object name in case any sub-schemas recursively reference it
	schema.GeneratedType = strct.TypeInfo

	// regular properties
	for propKey, prop := range schema.Properties {
		fieldName := GetGolangName(propKey)
		// calculate sub-schema name here, may not actually be used depending on type of schema!
		subSchemaName := g.getSchemaName(fieldName, prop)
		fieldType, err := g.processSchema(subSchemaName, prop)
		if err != nil {
			return nil, err
		}
		f := NewField(
			fieldName,
			propKey,
			fieldType,
			Contains(schema.Required, propKey),
			[]string{prop.Description},
		)
		if f.Required {
			strct.GenerateCode = true
		}
		strct.Fields[f.Name] = f
	}
	// additionalProperties with typed sub-schema
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.AdditionalPropertiesBool == nil {
		ap := (*Schema)(schema.AdditionalProperties)
		apName := g.getSchemaName("", ap)
		subTyp, err := g.processSchema(apName, ap)
		if err != nil {
			return nil, err
		}
		mapTyp := NewTypeInfo("string", "map", false, subTyp)
		// If this object is inline property for another object, and only Contains additional properties, we can
		// collapse the structure down to a map.
		//
		// If this object is a definition and only Contains additional properties, we can't do that or we end up with
		// no struct
		isDefinitionObject := strings.HasPrefix(schema.PathElement, "definitions")
		if len(schema.Properties) == 0 && !isDefinitionObject {
			// since there are no regular properties, we don't need to emit a struct for this object - return the
			// additionalProperties map type.
			return mapTyp, nil
		}
		// this struct will have both regular and additional properties
		f := NewField(
			"AdditionalProperties",
			"-",
			mapTyp,
			false,
			[]string{},
		)
		strct.Fields[f.Name] = f
		// setting this will cause marshal code to be emitted in Output()
		strct.GenerateCode = true
		strct.AdditionalType = subTyp
	}
	// additionalProperties as either true (everything) or false (nothing)
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.AdditionalPropertiesBool != nil {
		if *schema.AdditionalProperties.AdditionalPropertiesBool {
			// everything is valid additional
			subTyp := NewTypeInfo("string", "map", false, NewTypeInfo("", "interface", false, nil))
			f := NewField(
				"AdditionalProperties",
				"-",
				subTyp,
				false,
				[]string{},
			)
			strct.Fields[f.Name] = f
			// setting this will cause marshal code to be emitted in Output()
			strct.GenerateCode = true
			strct.AdditionalType = NewTypeInfo("", "interface", false, nil)
		} else {
			// nothing
			strct.GenerateCode = true
			strct.AdditionalType = NewTypeInfo("false", "boolean", false, nil)
		}
	}

	//store all structs based on unique signature for struct
	g.structCache[strct.TypeInfo.ShortName()] = append(g.structCache[strct.TypeInfo.ShortName()], strct)

	// objects are always a pointer
	return strct.TypeInfo, nil
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// return a name for this (sub-)schema.
func (g *Generator) getSchemaName(keyName string, schema *Schema) string {
	if len(schema.Title) > 0 {
		return GetGolangName(schema.Title)
	}
	if keyName != "" {
		return GetGolangName(keyName)
	}
	if schema.Parent == nil {
		return "Root"
	}
	if schema.JSONKey != "" {
		return GetGolangName(schema.JSONKey)
	}
	if schema.Parent != nil && schema.Parent.JSONKey != "" {
		return GetGolangName(schema.Parent.JSONKey + "Item")
	}
	g.anonCount++
	return fmt.Sprintf("Anonymous%d", g.anonCount)
}

// GetGolangName strips invalid characters out of golang struct or field names.
var mustContainLowercaseRegex = regexp.MustCompile("[a-z]")

func GetGolangName(s string) string {
	//Always convert to lower case to avoid all capital letters in the name.
	// eg. stop `title: "MY FOO BAR"`  becoming `type MYFOOBAR struct {...`
	if !mustContainLowercaseRegex.MatchString(s) {
		s = strings.ToLower(s)
	}

	buf := bytes.NewBuffer([]byte{})
	for i, v := range splitOnAll(s, IsNotAGoNameCharacter) {
		if i == 0 && strings.IndexAny(v, "0123456789") == 0 {
			// Go types are not allowed to start with a number, lets prefix with an underscore.
			buf.WriteRune('_')
		}
		buf.WriteString(CapitaliseFirstLetter(v))
	}
	return buf.String()
}

func splitOnAll(s string, shouldSplit func(r rune) bool) []string {
	rv := []string{}
	buf := bytes.NewBuffer([]byte{})
	for _, c := range s {
		if shouldSplit(c) {
			rv = append(rv, buf.String())
			buf.Reset()
		} else {
			buf.WriteRune(c)
		}
	}
	if buf.Len() > 0 {
		rv = append(rv, buf.String())
	}
	return rv
}

func IsNotAGoNameCharacter(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return true
}

func CapitaliseFirstLetter(s string) string {
	if s == "" {
		return s
	}
	prefix := s[0:1]
	suffix := s[1:]
	return strings.ToUpper(prefix) + suffix
}

// Struct defines the data required to generate a struct in Go.
type Struct struct {
	// The ID within the JSON schema, e.g. #/definitions/address
	ID string
	// The golang type information, e.g. "Address" or "*Address"
	TypeInfo *TypeInfo

	// Description of the struct
	Description string
	Fields      map[string]*Field

	GenerateCode   bool
	AdditionalType *TypeInfo
}

func (s *Struct) unifiedWith(other *Struct) *Struct {
	leastFieldsStruct := s
	mostFieldsStruct := other
	if len(s.Fields) > len(other.Fields) {
		leastFieldsStruct = other
		mostFieldsStruct = s
	}

	//check if all fields from the "leastFields" are available in "mostFields"
	for _, leastField := range leastFieldsStruct.Fields {
		if mostField, ok := mostFieldsStruct.Fields[leastField.Name]; ok {
			if mostField.Type.Name != leastField.Type.Name {
				return nil
			}
		} else {
			return nil
		}
	}

	//all fields are matchable, merge them together
	for _, notKeptField := range leastFieldsStruct.Fields {
		if keptField, ok := mostFieldsStruct.Fields[notKeptField.Name]; ok {
			keptField.Descriptions = utils.UniqueStrings(append(keptField.Descriptions, notKeptField.Descriptions...))
		}
	}

	fmt.Printf("TypeInfo: %s: Replacing: %s\n", mostFieldsStruct.TypeInfo.Id, leastFieldsStruct.TypeInfo.Id)
	mostFieldsStruct.TypeInfo.Replaces(leastFieldsStruct.TypeInfo)

	fmt.Printf("TypeInfo: %s: Replaced: %s\n", mostFieldsStruct.TypeInfo.Id, leastFieldsStruct.TypeInfo.Id)
	for _, f := range mostFieldsStruct.TypeInfo.referencedFields {
		fmt.Printf("TypeInfo: %s: Replaced: %s: Field Name: %s: Field Id: %s: Field Type: %s\n", mostFieldsStruct.TypeInfo.Id, leastFieldsStruct.TypeInfo.Id, f.Name, f.Id, f.Type.Id)
	}

	return mostFieldsStruct
}

// Field defines the data required to generate a field in Go.
type Field struct {
	Id string
	// The golang name, e.g. "Address1"
	Name string
	// The JSON name, e.g. "address1"
	JSONName string
	// The golang type of the field, e.g. a built-in type like "string" or the name of a struct generated
	// from the JSON schema.
	Type *TypeInfo
	// Required is set to true when the field is required.
	Required     bool
	Descriptions []string
}

func NewField(name, jsonName string, info *TypeInfo, required bool, descriptions []string) *Field {
	f := &Field{
		Id:           utils.RandomString(20),
		Name:         name,
		JSONName:     jsonName,
		Required:     required,
		Descriptions: descriptions,
	}
	info.AddFieldReference(f)
	return f
}

type TypeInfo struct {
	Id               string
	Name             string
	PrimitiveType    string
	SubType          *TypeInfo
	IsPointer        bool
	hasSameNames     bool
	isRootType       bool
	referencedFields map[string]*Field
	aliasFor         []string
	Format           Format
}

func (p *TypeInfo) ShortName() string {
	return p.Name
}
func (p *TypeInfo) LongName() string {
	return fmt.Sprintf("%s_%s", p.Name, p.Id)
}
func (p *TypeInfo) AddFieldReference(f *Field) {
	if f.Type != nil {
		//de-reference this field first
		_ = f.Type.RemoveFieldReference(f)
	}
	f.Type = p
	p.referencedFields[f.Id] = f
	fmt.Printf("TypeInfo: %s: Added Field: %s\n", p.Id, f.Id)
}
func (p *TypeInfo) RemoveFieldReference(f *Field) bool {
	if field, ok := p.referencedFields[f.Id]; ok {
		if field == f {
			fmt.Printf("TypeInfo: %s: Removed Field: %s: Old Type: %s:%s:same object\n", p.Id, f.Id, f.Type.Id, field.Type.Id)
		} else {
			fmt.Printf("TypeInfo: %s: Removed Field: %s: Old Type: %s:%s:diff object\n", p.Id, f.Id, f.Type.Id, field.Type.Id)
		}
		field.Type = nil
		delete(p.referencedFields, f.Id)
		return true
	}
	return false
}
func (p *TypeInfo) Replaces(old *TypeInfo) {
	for _, f := range old.referencedFields {
		p.AddFieldReference(f)
	}

	for _, f := range old.aliasFor {
		p.AddAliasFor(f)
	}
}
func (p *TypeInfo) String() string {
	if p.hasSameNames {
		return p.LongName()
	}
	return p.ShortName()
}
func (p *TypeInfo) IsAlias() bool {
	return len(p.aliasFor) > 0
}
func (p *TypeInfo) AddAliasFor(name string) {
	p.aliasFor = utils.UniqueStrings(append(p.aliasFor, name))
	sort.Strings(p.aliasFor)
}

func (p *TypeInfo) getPrimitiveTypeName() (name string, err error) {
	switch p.PrimitiveType {
	case "array":
		if p.SubType == nil {
			return "error_creating_array", errors.New("can't create an array of an empty subtype")
		}
		if name, err = p.SubType.getPrimitiveTypeName(); err != nil {
			return "", err
		} else {
			return "[]" + name, nil
		}
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "number":
		return "float64", nil
	case "null":
		return "nil", nil
	case "object":
		if p.SubType != nil {
			return "error_creating_object", errors.New("object cannot contain subtype")
		}
		if p.IsPointer {
			return "*" + p.String(), nil
		}
		return p.String(), nil
	case "string":
		switch p.Format {
		case FormatDatetime:
			return "*time.Time", nil
		case FormatUUID:
			return "*uuid.UUID", err
		default:
			return "string", nil
		}
	case "interface":
		return "interface{}", nil
	case "map":
		if p.Name == "" || p.SubType == nil {
			return "error_creating_map", fmt.Errorf("map type requires both a name and a subtype: %v", p)
		}
		if subName, err := p.SubType.getPrimitiveTypeName(); err != nil {
			return "", err
		} else {
			return fmt.Sprintf("map[%s]%s", p.Name, subName), nil
		}
	}

	return "undefined", fmt.Errorf("failed to get a primitive type for schemaType '%s' and subtype '%s'",
		p.Name, p.SubType)
}

func (p *TypeInfo) GetTypeAsString() string {
	pt, err := p.getPrimitiveTypeName()
	if err != nil {
		panic(err)
	}
	return pt
}

func NewTypeInfo(name string, primitiveType string, isPointer bool, subType *TypeInfo) *TypeInfo {
	st := TypeInfo{
		Id:               utils.RandomString(10),
		Name:             name,
		PrimitiveType:    primitiveType,
		IsPointer:        isPointer,
		SubType:          subType,
		referencedFields: map[string]*Field{},
	}
	return &st
}
