package utils

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func FileNameCreation(fileName string) string {
	return fmt.Sprintf("%s%s", fileName[:len(fileName)-len(filepath.Ext(fileName))], ".go")
}

// ReadFiles Reads file or files From Directories
func ReadFiles(inputPath string) ([]string, error) {
	stat, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		//single file entry
		fullPath, err := filepath.Abs(inputPath)
		if err != nil {
			return nil, err
		}

		return []string{fullPath}, nil
	}

	//read a directory
	files, err := os.ReadDir(inputPath)
	if err != nil {
		return nil, err
	}

	inputPath, err = filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}

	filePaths := make([]string, len(files))
	for i, file := range files {
		filePaths[i] = filepath.Join(inputPath, file.Name())
	}

	return filePaths, nil
}

type Flags struct {
	InputDir   string
	OutputPath string
}

func ParseFlags() Flags {
	inputDir := flag.String("input", "../schemas", "Please enter the input directory")
	outputPath := flag.String("output", "../output.go", "Please enter the target output go file")
	flag.Parse()

	return Flags{
		InputDir:  *inputDir,
		OutputPath: *outputPath,
	}
}

func Unique(slice []string) []string {
	// create a map with all the values as key
	uniqMap := make(map[string]struct{})
	for _, v := range slice {
		uniqMap[v] = struct{}{}
	}

	// turn the map keys into a slice
	uniqSlice := make([]string, 0, len(uniqMap))
	for v := range uniqMap {
		uniqSlice = append(uniqSlice, v)
	}
	return uniqSlice
}
