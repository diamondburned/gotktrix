package db

import (
	"github.com/fxamacker/cbor/v2"
)

type Marshaler interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
}

// CBORMarshaler is the default key-value marshaler.
var CBORMarshaler Marshaler = cborMarshaler{}

type cborMarshaler struct{}

func (cborMarshaler) Marshal(v interface{}) ([]byte, error) {
	return cbor.Marshal(v)
}

func (cborMarshaler) Unmarshal(b []byte, v interface{}) error {
	return cbor.Unmarshal(b, v)
}
