package test

import (
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/arrays"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/arrays --output ./generated/arrays/model.go

func TestArrayHasWordItemsOnlyOnceInName(t *testing.T) {
	assert.NotNil(t, models.Bar1Items{})
	assert.NotNil(t, models.Bar2Items{})
}
