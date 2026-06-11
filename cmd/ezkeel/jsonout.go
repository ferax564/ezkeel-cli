package main

import (
	"encoding/json"
	"io"
)

// emitJSON writes v to w as a single JSON document followed by a
// newline. HTML escaping is disabled — the output feeds terminals and
// agent pipes, not browsers, and `&`-escaped URLs would corrupt
// values like DATABASE_URL.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
