package db

import (
	"encoding/json"
)

type Marshaler interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
}

// JSONMarshaler is the default key-value marshaler.
var JSONMarshaler Marshaler = jsonMarshaler{}

type jsonMarshaler struct{}

func (jsonMarshaler) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonMarshaler) Unmarshal(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}
