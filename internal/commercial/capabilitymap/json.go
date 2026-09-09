package capabilitymap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Reject duplicate object keys rather than accepting encoding/json's last
// value wins behavior. Duplicate mapping entries are checked separately.
func decode(data []byte, target any, strict bool) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		keys := map[string]bool{}
		for d.More() {
			if delim == '{' {
				token, err = d.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if strict {
					key = strings.ToLower(key)
				}
				if !ok || keys[key] {
					return fmt.Errorf("duplicate/invalid object key %q", token)
				}
				keys[key] = true
			}
			if err := walk(); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if err := walk(); err != nil {
		return problem("JSON", "inputs", err.Error())
	}
	if _, err := d.Token(); err != io.EOF {
		return problem("JSON", "inputs", "trailing JSON value or invalid token")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(target); err != nil {
		return problem("JSON", "inputs", err.Error())
	}
	return nil
}

func DecodeDeclaration(data []byte) (Declaration, error) {
	var declaration Declaration
	err := decode(data, &declaration, true)
	return declaration, err
}

func DecodeDocument(data []byte) (Document, error) {
	var document Document
	err := decode(data, &document, true)
	return document, err
}
