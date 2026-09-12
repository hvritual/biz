// Package commercial embeds the generated, build-checked commercial policy.
package commercial

import _ "embed"

//go:embed generated/catalog.json
var catalog []byte

// Catalog returns a copy; callers cannot mutate the process policy.
func Catalog() []byte { return append([]byte(nil), catalog...) }
