package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// decodeJSON preserves numeric tokens as json.Number so schema annotations
// such as default and const do not lose integer precision before normalization.
func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var extra any
	switch err := decoder.Decode(&extra); {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return err
	default:
		return errors.New("multiple JSON values")
	}
}
