package surveillance

import "encoding/json"

// mustJSON matches the host's utils.MustJSON: marshal errors yield nil.
func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
