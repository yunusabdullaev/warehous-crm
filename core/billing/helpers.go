package billing

import "encoding/json"

// decodeJSON unmarshals the raw JSON message into the target struct.
func decodeJSON(raw json.RawMessage, target interface{}) error {
	return json.Unmarshal(raw, target)
}
