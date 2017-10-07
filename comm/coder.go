package comm

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
)

// 反序列化操作(uint32)
func GetUint32(body []byte) uint32 {
	return binary.BigEndian.Uint32(body)
}

// 反序列化操作(uint64)
func GetUint64(body []byte) uint64 {
	return binary.BigEndian.Uint64(body)
}

// 序列化操作(uint32)
func PutUint32(value uint32, body []byte) {
	binary.BigEndian.PutUint32(body, value)
}

// 序列化操作(uint64)
func PutUint64(value uint64, body []byte) {
	binary.BigEndian.PutUint64(body, value)
}

// 序列化操作(结构体类型)
func BinaryCoder(req interface{}) ([]byte, error) {
	iobuf := new(bytes.Buffer)
	enc := gob.NewEncoder(iobuf)
	err := enc.Encode(req)
	if err != nil {
		return nil, err
	}
	return iobuf.Bytes(), nil
}

// 反序列化操作(结构体类型)
func BinaryDecoder(buf []byte, rsp interface{}) error {
	iobuf := bytes.NewReader(buf)
	denc := gob.NewDecoder(iobuf)
	err := denc.Decode(rsp)
	return err
}
