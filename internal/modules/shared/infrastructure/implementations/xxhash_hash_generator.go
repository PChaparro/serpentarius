package implementations

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/PChaparro/serpentarius/internal/modules/shared/domain/definitions"
	"github.com/PChaparro/serpentarius/internal/modules/shared/infrastructure"
	"github.com/cespare/xxhash/v2"
)

// XxHashGenerator implements the HashGenerator interface using xxHash algorithm
type XxHashGenerator struct{}

var (
	xxHashGenerator *XxHashGenerator
	xxHashOnce      sync.Once
)

// GetXxHashGenerator returns a singleton instance of XxHashGenerator
func GetXxHashGenerator() definitions.HashGenerator {
	xxHashOnce.Do(func() {
		xxHashGenerator = &XxHashGenerator{}
		infrastructure.GetLogger().Info("XXHash generator initialized")
	})

	return xxHashGenerator
}

// GenerateHash generates a hash from the given input string using xxHash algorithm
func (x *XxHashGenerator) GenerateHash(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("input string cannot be empty")
	}

	// Generate a 64-bit xxHash
	hash := xxhash.Sum64String(input)

	// Convert the hash to a hexadecimal string
	hashString := strconv.FormatUint(hash, 16)

	return hashString, nil
}
