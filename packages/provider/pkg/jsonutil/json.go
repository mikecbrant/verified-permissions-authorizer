package jsonutil

import "encoding/json"

// CanonicalizeJSON returns a canonical (minified) JSON string for comparison.
//
// Comparison details:
// - The function normalizes JSON by unmarshaling into an interface{} and
//   re-marshaling. This removes insignificant whitespace and stabilizes key
//   ordering for maps, making string equality resilient to formatting and
//   key-order differences.
// - If input is not valid JSON, it is returned unchanged.
func CanonicalizeJSON(s string) string {
    var v any
    if err := json.Unmarshal([]byte(s), &v); err != nil { return s }
    b, err := json.Marshal(v); if err != nil { return s }
    return string(b)
}
