package converter

import (
	"github.com/azarc-io/json-schema-to-go-struct-generator/pkg/inputs"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func Convert(inputFiles []string, packageName string, outputFile string) error {
	//ensure that files are aways processed in deterministic order
	sort.Strings(inputFiles)

	schemas, err := inputs.ReadInputFiles(inputFiles, false) // passing true will check for schema key in the file
	if err != nil {
		return errors.Wrapf(err, "error while reading input file")

	}
	generatorInstance := inputs.New(schemas...) // instance of generator which will produce structs
	err = generatorInstance.CreateTypes()
	if err != nil {
		return errors.Wrapf(err, "error while generating instance for  proudcing structs")
	}

	packageDirectory := filepath.Dir(outputFile)
	err = os.MkdirAll(packageDirectory, 0755)
	if err != nil {
		return errors.Wrapf(err, "error while creating directory")
	}

	var w io.Writer
	w, err = os.Create(outputFile)
	if err != nil {
		return errors.Wrapf(err, "error while creating output file")
	}

	return inputs.Output(w, generatorInstance, packageName, inputFiles)
}
