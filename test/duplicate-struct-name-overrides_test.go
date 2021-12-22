package test

import (
	"github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/duplicate-struct-name-overrides/duplicate-struct-name-overrides-schema"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/duplicate-struct-name-overrides --output ./generated/duplicate-struct-name-overrides

func TestHasMultipleStructWhenDuplicateNames(t *testing.T) {
	assert.NotNil(t, models.Foo{})
	assert.NotNil(t, models.Foo_01696a3b2c{})
	assert.NotNil(t, models.Root{})
}

