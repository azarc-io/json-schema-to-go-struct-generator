package test

import (
	"encoding/json"
	model "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/marshal"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate go run ../cmd/main.go --input ./samples/marshal --output ./generated/marshal/model.go

func TestThatJSONCanBeRoundtrippedUsingGeneratedStructs(t *testing.T) {
	j := `{"address":{"county":"countyValue"},"name":"nameValue"}`

	e := &model.Example{}
	err := json.Unmarshal([]byte(j), e)

	if err != nil {
		t.Fatal("Failed to unmarshall JSON with error ", err)
	}

	if e.Address.County != "countyValue" {
		t.Errorf("the county value was not found, expected 'countyValue' got '%s'", e.Address.County)
	}

	op, err := json.Marshal(e)

	if err != nil {
		t.Error("Failed to marshal JSON with error ", err)
	}

	if string(op) != j {
		t.Errorf("expected %s, got %s", j, string(op))
	}
}

func TestUnmarshalWithoutErrorButContainValidationErrors(t *testing.T) {
	j := `{"address":{"line2":"foo line 2"},"name":"bar name", "zulu": "africa"}`

	e := &model.Home{}
	err := json.Unmarshal([]byte(j), e)
	assert.Nil(t, err)
	assert.Equal(t, "africa", e.Zulu)

	errs := e.Address.Validate()
	assert.Equal(t, 2, len(errs))
	assert.Equal(t, "\"Line1\" is required but was not present: field required validation failed", errs[0].Error())
	assert.ErrorIs(t, errs[0], model.ErrFieldRequired)
	assert.ErrorIs(t, errs[1], model.ErrFieldRequired)
}
