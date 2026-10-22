// Package jsondoc reads JSON without silently discarding duplicate keys or number precision.
package jsondoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ErrLimit = errors.New("JSON resource limit exceeded")

func Decode(data []byte, maxDepth int) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := read(d, 0, maxDepth)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("expected exactly one JSON value")
	}
	return v, nil
}

func read(d *json.Decoder, depth, limit int) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, errors.New("invalid JSON")
	}
	if n, ok := token.(json.Number); ok {
		if len(n) > 1024 {
			return nil, ErrLimit
		}
		if i := strings.IndexAny(string(n), "eE"); i >= 0 {
			exp, err := strconv.Atoi(string(n)[i+1:])
			if err != nil || exp < -10000 || exp > 10000 {
				return nil, ErrLimit
			}
		}
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	if depth >= limit {
		return nil, ErrLimit
	}
	switch delim {
	case '{':
		obj := make(map[string]any)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, errors.New("invalid object key")
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("invalid object key")
			}
			if _, exists := obj[name]; exists {
				return nil, errors.New("duplicate JSON object key")
			}
			value, err := read(d, depth+1, limit)
			if err != nil {
				return nil, err
			}
			obj[name] = value
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return nil, errors.New("invalid JSON object")
		}
		return obj, nil
	case '[':
		items := make([]any, 0)
		for d.More() {
			v, err := read(d, depth+1, limit)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return nil, errors.New("invalid JSON array")
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}
