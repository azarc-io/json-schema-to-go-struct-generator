package test

import (
	_ "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/duplicate-struct-name-overrides"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/duplicate-struct-name-overrides --output ./generated/duplicate-struct-name-overrides/model.go

func TestHasMultipleStructWhenDuplicateNames(t *testing.T) {
	pkg := GetPackageStructs("github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/duplicate-struct-name-overrides")

	assert.NotNil(t, pkg)
	assert.True(t, pkg.HasField("Foo"))
	assert.True(t, pkg.HasField("Root"))
	assert.True(t, pkg.HasFieldWithPrefix("Foo_"))
}

