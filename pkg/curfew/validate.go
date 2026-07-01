package curfew

// Validate reports whether content is syntactically valid .codecurfew DSL.
// It is a thin wrapper around Parse for callers that only care about
// syntax-checking (e.g. a future standalone validator tool).
func Validate(content string) error {
	_, err := Parse(content)
	return err
}
