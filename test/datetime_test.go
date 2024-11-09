package test

import (
	"encoding/json"
	models "github.com/azarc-io/json-schema-to-go-struct-generator/test/generated/datetime"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

//go:generate go run ../cmd/main.go --input ./samples/datetime --output ./generated/datetime/model.go

func TestDatetimeMarshalValidateSuccess(t *testing.T) {
	param := `{
		"myDate": "2022-09-23T21:45:58Z"
	}`

	prod := models.DatetimeValue{}
	err := json.Unmarshal([]byte(param), &prod)
	assert.Nil(t, err)

	expected := time.Date(2022, 9, 23, 21, 45, 58, 0, time.UTC)
	assert.Equal(t, &expected, prod.MyDate)
}

func TestDatetimeMarshalFailed(t *testing.T) {
	param := `{
		"myDate": "2022-not-valid"
	}`

	prod := models.DatetimeValue{}
	err := json.Unmarshal([]byte(param), &prod)
	assert.Error(t, err)
}
