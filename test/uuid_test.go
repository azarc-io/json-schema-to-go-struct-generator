package test

import (
	"encoding/json"
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/uuid"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/uuid --output ./generated/uuid/model.go

func TestUUIDMarshalValidateSuccess(t *testing.T) {
	expected, _ := uuid.NewUUID()
	param := "{ \"myUUID\": \"" + expected.String() + "\" }"

	prod := models.UUIDValue{}
	err := json.Unmarshal([]byte(param), &prod)
	assert.Nil(t, err)
	assert.Equal(t, &expected, prod.MyUUID)
}

func TestUUIDMarshalFailed(t *testing.T) {
	param := `{
		"myUUID": "not-valid"
	}`

	prod := models.UUIDValue{}
	err := json.Unmarshal([]byte(param), &prod)
	assert.Error(t, err)
}
