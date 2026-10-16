package infrastructure

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Probe structs (named types at file level) for the custom validator tags
// registered in GetValidatorInstance.
type s3BucketProbe struct {
	Value string `validate:"s3bucket"`
}

type s3KeyProbe struct {
	Value string `validate:"s3key"`
}

type cacheNamespaceProbe struct {
	Value string `validate:"cachenamespace"`
}

// validatorCase is one row of the validator matrices (plan §5 Phase 1).
// The mutant is documented in a `// Mutant:` comment on each row: it names
// the prod line whose removal must turn that case red (plan §1.1.3).
type validatorCase struct {
	name   string
	value  string
	wantOK bool
}

// runValidatorCasesArgs groups the matrix inputs (plan §1.2 args-struct rule).
type runValidatorCasesArgs struct {
	Cases []validatorCase
	Probe func(value string) any
}

func runValidatorCases(t *testing.T, args runValidatorCasesArgs) {
	t.Helper()
	for _, tc := range args.Cases {
		t.Run(tc.name, func(t *testing.T) {
			err := GetValidatorInstance().Struct(args.Probe(tc.value))
			if tc.wantOK {
				assert.NoError(t, err, "value %q should be valid", tc.value)
			} else {
				assert.Error(t, err, "value %q should be invalid", tc.value)
			}
		})
	}
}

func TestValidateS3Bucket(t *testing.T) {
	bucket63 := strings.Repeat("a", 63)
	bucket64 := strings.Repeat("a", 64)

	runValidatorCases(t, runValidatorCasesArgs{
		Probe: func(value string) any { return s3BucketProbe{Value: value} },
		Cases: []validatorCase{
			// Mutant: if the `len(bucket) < 3` check is removed, this case stops failing.
			{name: "minimum 3 chars valid", value: "abc", wantOK: true},
			// Mutant: if the `len(bucket) > 63` check is removed, this case stops failing.
			{name: "maximum 63 chars valid", value: bucket63, wantOK: true},
			{name: "typical hyphenated name valid", value: "my-bucket-01", wantOK: true}, // Mutant: if s3BucketNamePattern were relaxed (e.g. `.*`), the invalid charset cases would stop failing.
			{name: "dotted name valid", value: "my.bucket.01", wantOK: true},             // Mutant: if the pattern required only alphanumerics, this case would fail.
			// Mutant: if the `len(bucket) < 3` check is removed, this case stops failing.
			{name: "2 chars invalid", value: "ab", wantOK: false},
			// Mutant: if the `len(bucket) > 63` check is removed, this case stops failing.
			{name: "64 chars invalid", value: bucket64, wantOK: false},
			// Mutant: if the `strings.Contains(bucket, "..")` check is removed, this case stops failing.
			{name: "consecutive dots invalid", value: "my..bucket", wantOK: false},
			// Mutant: if the pattern accepted uppercase, this case would stop failing.
			{name: "uppercase invalid", value: "My-Bucket", wantOK: false},
			// Mutant: if the pattern accepted `_`, this case would stop failing.
			{name: "underscore invalid", value: "my_bucket", wantOK: false},
			// Mutant: if the pattern allowed a leading `-`, this case would stop failing.
			{name: "leading hyphen invalid", value: "-mybucket", wantOK: false},
			// Mutant: if the pattern allowed a trailing `.`, this case would stop failing.
			{name: "trailing dot invalid", value: "mybucket.", wantOK: false},
			// Mutant: if the `len(bucket) < 3` check is removed, this case stops failing.
			{name: "empty invalid", value: "", wantOK: false},
		},
	})
}

func TestValidateS3Key(t *testing.T) {
	key1024 := strings.Repeat("k", 1024)
	key1025 := strings.Repeat("k", 1025)

	runValidatorCases(t, runValidatorCasesArgs{
		Probe: func(value string) any { return s3KeyProbe{Value: value} },
		Cases: []validatorCase{
			// Mutant: if the `len(key) < 1` check is removed, this case stops failing.
			{name: "1 char valid", value: "a", wantOK: true},
			{name: "simple key valid", value: "report.pdf", wantOK: true},                    // Mutant: if a prefix were required, this case would fail.
			{name: "intermediate slashes valid", value: "folder/sub/file.pdf", wantOK: true}, // Mutant: if any `/` were rejected, this case would fail.
			// Mutant: if the `len(key) > 1024` check is removed, this case stops failing.
			{name: "1024 chars valid", value: key1024, wantOK: true},
			// Mutant: if `!strings.HasPrefix(key, "/")` is removed, this case stops failing.
			{name: "leading slash invalid", value: "/report.pdf", wantOK: false},
			// Mutant: if the `len(key) < 1` check is removed, this case stops failing.
			{name: "empty invalid", value: "", wantOK: false},
			// Mutant: if the `len(key) > 1024` check is removed, this case stops failing.
			{name: "1025 chars invalid", value: key1025, wantOK: false},
		},
	})
}

func TestValidateCacheNamespace(t *testing.T) {
	ns128 := strings.Repeat("n", 128)
	ns129 := strings.Repeat("n", 129)

	runValidatorCases(t, runValidatorCasesArgs{
		Probe: func(value string) any { return cacheNamespaceProbe{Value: value} },
		Cases: []validatorCase{
			// Mutant: if the `len(namespace) < 1` check is removed, this case stops failing.
			{name: "simple valid", value: "pdf", wantOK: true},
			{name: "colon segments valid", value: "tenant:pdf:render", wantOK: true}, // Mutant: if cacheNamespacePattern did not accept `:`, this case would fail.
			{name: "mixed charset valid", value: "Tenant-1_2.3:ns", wantOK: true},    // Mutant: if the pattern were lowercase only, this case would fail.
			// Mutant: if the `len(namespace) > 128` check is removed, this case stops failing.
			{name: "128 chars valid", value: ns128, wantOK: true},
			// Mutant: if the `len(namespace) < 1` check is removed, this case stops failing.
			{name: "empty invalid", value: "", wantOK: false},
			// Mutant: if the `len(namespace) > 128` check is removed, this case stops failing.
			{name: "129 chars invalid", value: ns129, wantOK: false},
			// Mutant: if the `strings.Contains(namespace, "..")` check is removed, this case stops failing.
			{name: "consecutive dots invalid", value: "a..b", wantOK: false},
			// Mutant: if cacheNamespacePattern accepted a leading empty segment, this case would stop failing.
			{name: "leading colon invalid", value: ":ns", wantOK: false},
			// Mutant: if cacheNamespacePattern accepted a trailing empty segment, this case would stop failing.
			{name: "trailing colon invalid", value: "ns:", wantOK: false},
			// Mutant: if cacheNamespacePattern accepted `::`, this case would stop failing.
			{name: "double colon invalid", value: "a::b", wantOK: false},
			// Mutant: if cacheNamespacePattern accepted `/`, this case would stop failing.
			{name: "slash invalid", value: "a/b", wantOK: false},
			// Mutant: if cacheNamespacePattern accepted spaces, this case would stop failing.
			{name: "space invalid", value: "a b", wantOK: false},
		},
	})
}
