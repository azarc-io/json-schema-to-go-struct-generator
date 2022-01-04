package test

import (
	"github.com/azarc-io/json-schema-to-go-struct-generator/pkg/converter"
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/upper-case-titles/upper-case-schema"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/upper-case-titles --output ./generated/upper-case-titles

func TestConvert(t *testing.T) {
	wd, _ := os.Getwd()
	filePath := filepath.Join(wd, "samples/upper-case-titles/upper-case-schema.json")
	err := converter.Convert([]string{filePath}, "./generated/upper-case-titles")
	assert.Empty(t, err)
}

func TestHasLowerCaseOnUpperCaseTitles(t *testing.T) {
	assert.NotNil(t, models.BarIt{})
	assert.NotNil(t, models.SomeTest{})
}

