package query

import "github.com/vmihailenco/msgpack/v5"

func decodeMsgpack(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
