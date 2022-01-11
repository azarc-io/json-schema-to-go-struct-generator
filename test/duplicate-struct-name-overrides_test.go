package test

import (
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/duplicate-struct-name-overrides"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/duplicate-struct-name-overrides --output ./generated/duplicate-struct-name-overrides/model.go

func TestHasMultipleStructWhenDuplicateNames(t *testing.T) {
	assert.NotNil(t, models.Foo{})
	assert.NotNil(t, models.Foo_2eb5aef7cf{})
	assert.NotNil(t, models.Root{})
}

