package common

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
)

func (obj *EncodedObject) Value() (any, error) {
	if obj.IsStream {
		return nil, errors.New("cannot get object directly on stream")
	}
	switch obj.Language {
	case Language_LANG_JSON:
		var v any
		if err := json.Unmarshal(obj.Data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case Language_LANG_PYTHON:
		return nil, errors.New("decoding python obj is not supported in Go runtime")
	case Language_LANG_GO:
		var v any
		dec := gob.NewDecoder(bytes.NewReader(obj.Data))
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, errors.New("unknown language")
	}
}

func (obj *EncodedObject) Encode() (*EncodedObject, error) {
	return obj, nil
}
