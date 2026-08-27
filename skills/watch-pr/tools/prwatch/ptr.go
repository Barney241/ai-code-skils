package main

// ptr returns a pointer to v.
//
// Go 1.26 allows new(expr) with a value, which is what this replaces. Keeping a
// helper instead lets the module build on any toolchain from 1.21, which matters
// more for a tool people copy into their own repos than the newer spelling does.
func ptr[T any](v T) *T {
	return &v
}
