package test

import (
	"github.com/azarc-io/json-schema-to-go-struct-generator/pkg/converter"
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/complex-types"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/complex-types --output ./generated/complex-types/models.go

func TestDoesNotContainDuplicateStructs(t *testing.T) {
	assert.NotNil(t, models.Bar1{Person: &models.Person{
		Child: &models.Person{
			Child: nil,
			First: "",
			Last:  "",
		},
		First: "",
		Last:  "",
	}})

	assert.NotNil(t, models.Bar2{Person: &models.Person{
		Child: &models.Person{
			Child: nil,
			First: "",
			Last:  "",
		},
		First: "",
		Last:  "",
	}})
}

func TestGenerate1(t *testing.T) {
	files := []string{path.Join(os.Getenv("PWD"), "./samples/complex-types/duplicate-structs-for-single-type-schema.json")}
	err := converter.Convert(files, "models", "./generated/complex-types/generate1/duplicate-structs.go", true)
	assert.Nil(t, err)
}

func TestGenerate2(t *testing.T) {
	files := []string{
		path.Join(os.Getenv("PWD"), "./samples/complex-types/duplicate-structs-without-creating-duplicate-models-schema.json"),
	}
	err := converter.Convert(files, "models", "./generated/complex-types/generate2/test2.go", true)
	assert.Nil(t, err)
}
