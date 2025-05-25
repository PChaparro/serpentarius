package utilities

import (
	"os"
	"path/filepath"
)

// FindProjectRoot finds the project root (where go.mod is located)
func FindProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// ReadFileFromTestsDataDirectory reads a file from the tests/data directory and returns its content as a byte slice.
func ReadFileFromTestsDataDirectory(filename string) ([]byte, error) {
	projectRoot := FindProjectRoot()
	absPath := filepath.Join(projectRoot, "tests", "data", filename)
	return os.ReadFile(absPath)
}
