package inputs

import (
	"bytes"
	"crypto/sha1"
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
	Structs  map[string]Struct
	Aliases  map[string]Field
	// cache for reference types; k=url v=type
	refs      map[string]string
	anonCount int
}

// New creates an instance of a generator which will produce structs.
func New(schemas ...*Schema) *Generator {
	return &Generator{
		schemas:  schemas,
		resolver: NewRefResolver(schemas),
		Structs:  make(map[string]Struct),
		Aliases:  make(map[string]Field),
		refs:     make(map[string]string),
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
		if err != nil {
			return err
		}
		// ugh: if it was anything but a struct the type will not be the name...
		if rootType != "*"+name {
			a := Field{
				Name:         name,
				JSONName:     "",
				Type:         rootType,
				Required:     false,
				Descriptions: []string{schema.Description},
			}
			g.Aliases[a.Name] = a
		}
	}
	return
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
func (g *Generator) processReference(schema *Schema) (string, error) {
	schemaPath := g.resolver.GetPath(schema)
	if schema.Reference == "" {
		return "", errors.New("processReference empty reference: " + schemaPath)
	}
	refSchema, err := g.resolver.GetSchemaByReference(schema)
	if err != nil {
		return "", errors.New("processReference: reference \"" + schema.Reference + "\" not found at \"" + schemaPath + "\"")
	}
	if refSchema.GeneratedType == "" {
		// reference is not resolved yet. Do that now.
		refSchemaName := g.getSchemaName("", refSchema)
		typeName, err := g.processSchema(refSchemaName, refSchema)
		if err != nil {
			return "", err
		}
		return typeName, nil
	}
	return refSchema.GeneratedType, nil
}

// returns the type refered to by schema after resolving all dependencies
func (g *Generator) processSchema(schemaName string, schema *Schema) (typ string, err error) {
	if len(schema.Definitions) > 0 {
		err = g.processDefinitions(schema)
		if err != nil {
			return
		}
	}

	schema.FixMissingTypeValue()
	// if we have multiple schema types, the golang type will be interface{}
	typ = "interface{}"
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
					return "", err
				}
				if !isMultiType {
					return rv, nil
				}
			case "array":
				rv, err := g.processArray(name, schema)
				if err != nil {
					return "", err
				}
				if !isMultiType {
					return rv, nil
				}
			default:
				rv, err := getPrimitiveTypeName(schemaType, "", false)
				if err != nil {
					return "", err
				}
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
func (g *Generator) processArray(name string, schema *Schema) (typeStr string, err error) {
	if schema.Items != nil {
		propName := name
		if ! strings.HasSuffix(propName, "Items") {
			propName += "Items"
		}

		// subType: fallback name in case this array Contains inline object without a title
		subName := g.getSchemaName(propName, schema.Items)
		subTyp, err := g.processSchema(subName, schema.Items)
		if err != nil {
			return "", err
		}
		finalType, err := getPrimitiveTypeName("array", subTyp, true)
		if err != nil {
			return "", err
		}
		// only alias root arrays
		if schema.Parent == nil {
			array := Field{
				Name:         name,
				JSONName:     "",
				Type:         finalType,
				Required:     Contains(schema.Required, name),
				Descriptions: []string{schema.Description},
			}
			g.Aliases[array.Name] = array
		}
		return finalType, nil
	}
	return "[]interface{}", nil
}

// name: name of the struct (calculated by caller)
// schema: detail incl properties & child objects
// returns: generated type
func (g *Generator) processObject(name string, schema *Schema) (typ string, err error) {
	strct := Struct{
		ID:          schema.ID(),
		Name:        name,
		Description: schema.Description,
		Fields:      make(map[string]Field, len(schema.Properties)),
	}
	// cache the object name in case any sub-schemas recursively reference it
	schema.GeneratedType = "*" + name

	// regular properties
	for propKey, prop := range schema.Properties {
		fieldName := GetGolangName(propKey)
		// calculate sub-schema name here, may not actually be used depending on type of schema!
		subSchemaName := g.getSchemaName(fieldName, prop)
		fieldType, err := g.processSchema(subSchemaName, prop)
		if err != nil {
			return "", err
		}
		f := Field{
			Name:         fieldName,
			JSONName:     propKey,
			Type:         fieldType,
			Required:     Contains(schema.Required, propKey),
			Descriptions: []string{prop.Description},
		}
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
			return "", err
		}
		mapTyp := "map[string]" + subTyp
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
		f := Field{
			Name:         "AdditionalProperties",
			JSONName:     "-",
			Type:         mapTyp,
			Required:     false,
			Descriptions: []string{},
		}
		strct.Fields[f.Name] = f
		// setting this will cause marshal code to be emitted in Output()
		strct.GenerateCode = true
		strct.AdditionalType = subTyp
	}
	// additionalProperties as either true (everything) or false (nothing)
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.AdditionalPropertiesBool != nil {
		if *schema.AdditionalProperties.AdditionalPropertiesBool {
			// everything is valid additional
			subTyp := "map[string]interface{}"
			f := Field{
				Name:         "AdditionalProperties",
				JSONName:     "-",
				Type:         subTyp,
				Required:     false,
				Descriptions: []string{},
			}
			strct.Fields[f.Name] = f
			// setting this will cause marshal code to be emitted in Output()
			strct.GenerateCode = true
			strct.AdditionalType = "interface{}"
		} else {
			// nothing
			strct.GenerateCode = true
			strct.AdditionalType = "false"
		}
	}

	//if this struct already exists, then check to see if it's a different signature
	if prevStruct,ok := g.Structs[strct.Name]; ok {
		if structToUse := strct.unifiedWith(&prevStruct); structToUse != nil {
			//these structs were unified, use the return as the unified version
			g.Structs[structToUse.Name] = *structToUse
		} else {
			//structs cannot be unified, set as different struct
			strct.Name = strct.getUniqueSig()
			g.Structs[strct.Name] = strct
		}
	} else {
		g.Structs[strct.Name] = strct
	}


	// objects are always a pointer
	return getPrimitiveTypeName("object", strct.Name, true)
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func getPrimitiveTypeName(schemaType string, subType string, pointer bool) (name string, err error) {
	switch schemaType {
	case "array":
		if subType == "" {
			return "error_creating_array", errors.New("can't create an array of an empty subtype")
		}
		return "[]" + subType, nil
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "number":
		return "float64", nil
	case "null":
		return "nil", nil
	case "object":
		if subType == "" {
			return "error_creating_object", errors.New("can't create an object of an empty subtype")
		}
		if pointer {
			return "*" + subType, nil
		}
		return subType, nil
	case "string":
		return "string", nil
	}

	return "undefined", fmt.Errorf("failed to get a primitive type for schemaType %s and subtype %s",
		schemaType, subType)
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
	if ! mustContainLowercaseRegex.MatchString(s) {
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
	// The golang name, e.g. "Address"
	Name string
	// Description of the struct
	Description string
	Fields      map[string]Field

	GenerateCode   bool
	AdditionalType string
}

func (s *Struct) getUniqueSig() string {
	var allFieldDescr []string
	for _, field := range s.Fields {
		allFieldDescr = append(allFieldDescr, field.Name + field.Type)
	}

	sort.Strings(allFieldDescr)
	hash := sha1.Sum([]byte(strings.Join(allFieldDescr, "")))
	return fmt.Sprintf("%s_%x", s.Name, hash[0:5])
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
		if mostField,ok := mostFieldsStruct.Fields[leastField.Name]; ok {
			if mostField.Type != leastField.Type {
				return nil
			}
		} else {
			return nil
		}
	}

	//all fields are matchable, merge them together
	for _, field := range leastFieldsStruct.Fields {
		if keptField,ok := mostFieldsStruct.Fields[field.Name]; ok {
			keptField.Descriptions = utils.Unique(append(keptField.Descriptions, field.Descriptions...))
			mostFieldsStruct.Fields[field.Name] = keptField
		}
	}

	return mostFieldsStruct
}

// Field defines the data required to generate a field in Go.
type Field struct {
	// The golang name, e.g. "Address1"
	Name string
	// The JSON name, e.g. "address1"
	JSONName string
	// The golang type of the field, e.g. a built-in type like "string" or the name of a struct generated
	// from the JSON schema.
	Type string
	// Required is set to true when the field is required.
	Required     bool
	Descriptions []string
}
