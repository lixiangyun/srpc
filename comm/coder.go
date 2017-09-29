package comm

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
)

func GetUint32(body []byte) uint32 {
	return binary.BigEndian.Uint32(body)
}

func GetUint64(body []byte) uint64 {
	return binary.BigEndian.Uint64(body)
}

func PutUint32(value uint32, body []byte) {
	binary.BigEndian.PutUint32(body, value)
}

func PutUint64(value uint64, body []byte) {
	binary.BigEndian.PutUint64(body, value)
}

// 报文序列化
func CodePacket(req interface{}) ([]byte, error) {
	iobuf := new(bytes.Buffer)
	enc := gob.NewEncoder(iobuf)
	err := enc.Encode(req)
	if err != nil {
		return nil, err
	}

	return iobuf.Bytes(), nil
}

// 报文反序列化
func DecodePacket(buf []byte, rsp interface{}) error {
	iobuf := bytes.NewReader(buf)
	denc := gob.NewDecoder(iobuf)
	err := denc.Decode(rsp)

	return err
}
