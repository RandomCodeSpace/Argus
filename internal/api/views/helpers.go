package views

import "encoding/json"

// rawToAny unmarshals a json.RawMessage into a generic any value so it is
// re-encoded as structured JSON on the wire. Empty/invalid blobs become nil.
func rawToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}
