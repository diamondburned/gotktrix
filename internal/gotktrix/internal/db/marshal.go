package db

import (
	"bytes"
	"encoding/json"
)

type Marshaler interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
}

type jsonMarshaler struct{}

var JSONMarshaler Marshaler = jsonMarshaler{}

func (jsonMarshaler) Marshal(v interface{}) ([]byte, error) {
	buf := bytes.Buffer{}
	buf.Grow(1 << 15) // 32KB

	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}

	return cpy(buf.Bytes()), nil
}

func (jsonMarshaler) Unmarshal(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}

func cpy(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
