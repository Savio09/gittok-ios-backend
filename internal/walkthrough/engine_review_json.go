package walkthrough

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// exactReviewJSON accepts only the exact review contract before encoding/json's
// case-insensitive field matching. The token walk rejects duplicate keys and
// unexpected nesting; it is bounded to 40 decisions and 1024 tokens.
func exactReviewJSON(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	tokens := 0
	next := func() (json.Token, error) {
		tokens++
		if tokens > 1024 {
			return nil, ErrProviderResponse
		}
		return decoder.Token()
	}
	delimiter := func(want json.Delim) bool { token, err := next(); return err == nil && token == want }
	var object func(bool) bool
	object = func(decision bool) bool {
		if !delimiter('{') {
			return false
		}
		seen := map[string]bool{}
		for decoder.More() {
			token, err := next()
			key, ok := token.(string)
			if err != nil || !ok || seen[key] {
				return false
			}
			seen[key] = true
			switch key {
			case "accepted":
				token, err = next()
				if _, ok := token.(bool); err != nil || !ok {
					return false
				}
			case "reason":
				token, err = next()
				if _, ok := token.(string); err != nil || !ok {
					return false
				}
			case "statement_id":
				if !decision {
					return false
				}
				token, err = next()
				if _, ok := token.(string); err != nil || !ok {
					return false
				}
			case "decisions":
				if decision || !delimiter('[') {
					return false
				}
				count := 0
				for decoder.More() {
					count++
					if count > maxStatements || !object(true) {
						return false
					}
				}
				if !delimiter(']') {
					return false
				}
			default:
				return false
			}
		}
		return len(seen) == 3 && delimiter('}')
	}
	if !object(false) {
		return false
	}
	_, err := next()
	return err == io.EOF
}
