package definitions

// HashGenerator is an interface for generating hashes from strings.
type HashGenerator interface {
	// GenerateHash generates a hash from the given input string.
	GenerateHash(input string) (string, error)
}
