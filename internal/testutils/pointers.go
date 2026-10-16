// Package testutils holds small helpers shared by test files across the repo
// (unit tests under internal/... and e2e suites under tests/e2e).
package testutils

// StrPtr returns a pointer to v. Used to build optional request fields.
func StrPtr(v string) *string { return &v }

// BoolPtr returns a pointer to v. Used to build optional request fields.
func BoolPtr(v bool) *bool { return &v }

// FloatPtr returns a pointer to v. Used to build optional request fields.
func FloatPtr(v float64) *float64 { return &v }

// Int64Ptr returns a pointer to v. Used to build optional request fields.
func Int64Ptr(v int64) *int64 { return &v }
