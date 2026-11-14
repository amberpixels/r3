package enginefile

import (
	"encoding/json"
	"strconv"
)

// jsonFieldValue reads a key path out of a JSON column's Go value, mirroring
// what the SQL flavors extract: a missing key (or a non-object on the way) is
// nil; text mode yields the scalar as a string (an object or array as its JSON
// text), number mode a float64 or nil when the key is not a number.
func jsonFieldValue(colVal any, path []string, numeric bool) any {
	raw, err := json.Marshal(colVal)
	if err != nil {
		return nil
	}
	var cur any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil
	}
	for _, seg := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		if cur, ok = obj[seg]; !ok {
			return nil
		}
	}

	switch v := cur.(type) {
	case nil:
		return nil
	case float64:
		if numeric {
			return v
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		if numeric {
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				return n
			}
			return nil
		}
		return v
	case bool:
		if numeric {
			return nil
		}
		return strconv.FormatBool(v)
	default:
		if numeric {
			return nil
		}
		text, _ := json.Marshal(v)
		return string(text)
	}
}
