package store

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
)

func (obj *EncodedObject) Encode() (*EncodedObject, error) {
	return obj, nil
}

func (obj *EncodedObject) Value() (any, error) {
	if obj.IsStream {
		return nil, fmt.Errorf("cannot get object directly on stream")
	}
	switch obj.Language {
	case Language_LANGUAGE_JSON:
		var v any
		if err := json.Unmarshal(obj.Data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case Language_LANGUAGE_PYTHON:
		return nil, fmt.Errorf("decoding python obj is not supported in Go runtime")
	case Language_LANGUAGE_GO:
		var v any
		dec := gob.NewDecoder(bytes.NewReader(obj.Data))
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unknown language")
	}
}
