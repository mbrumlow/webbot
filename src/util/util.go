package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/net/websocket"
)

func ReadMessage(ws *websocket.Conn, max uint32) ([]byte, error) {

	var msgSize uint32
	if err := binary.Read(ws, binary.BigEndian, &msgSize); err != nil {
		return nil, err
	}

	if msgSize > max {
		return nil, fmt.Errorf("Message size %v > max %v", msgSize, max)
	}

	msg := make([]byte, msgSize)
	if _, err := io.ReadFull(ws, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func WriteMessage(ws *websocket.Conn, msg []byte) error {

	if buf, err := Encode32HeadBuf(uint32(len(msg)), msg); err != nil {
		return err
	} else {
		if err := websocket.Message.Send(ws, buf); err != nil {
			return fmt.Errorf("Failed to write %v bytes to network: %v", len(buf), err)
		}
	}
	return nil

}

func Encode32TimeHeadBuf(u uint32, t uint64, buf []byte) ([]byte, error) {

	bb := bytes.NewBuffer(make([]byte, 0, len(buf)+4+8)) // +4+8 bytes for type header

	if err := binary.Write(bb, binary.BigEndian, u); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	if err := binary.Write(bb, binary.BigEndian, t); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	if n, err := bb.Write(buf); err != nil {
		return nil, fmt.Errorf("Failed to write %v bytes to buffer: %v", n, err)
	}
	return bb.Bytes(), nil

}

func EncodeCapDef(t, i, v, g uint32, r uint64, buf []byte) ([]byte, error) {

	bb := bytes.NewBuffer(make([]byte, 0, len(buf)+4+4+4)) // +4+4+4 bytes for type header

	// Type
	if err := binary.Write(bb, binary.BigEndian, t); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	// ID
	if err := binary.Write(bb, binary.BigEndian, i); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	// Version
	if err := binary.Write(bb, binary.BigEndian, v); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	// Group
	if err := binary.Write(bb, binary.BigEndian, g); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	// Revision
	if err := binary.Write(bb, binary.BigEndian, r); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	if n, err := bb.Write(buf); err != nil {
		return nil, fmt.Errorf("Failed to write %v bytes to buffer: %v", n, err)
	}
	return bb.Bytes(), nil

}

func Encode32HeadBuf(u uint32, buf []byte) ([]byte, error) {

	bb := bytes.NewBuffer(make([]byte, 0, len(buf)+4)) // +4 bytes for type header
	if err := binary.Write(bb, binary.BigEndian, u); err != nil {
		return nil, fmt.Errorf("Failed to write message header to buffer: %v", err)
	}

	if n, err := bb.Write(buf); err != nil {
		return nil, fmt.Errorf("Failed to write %v bytes to buffer: %v", n, err)
	}
	return bb.Bytes(), nil

}

func DrainChan(msgChan <-chan []byte, f func()) {
	for {
		_, ok := <-msgChan
		if !ok {
			f()
			return
		}
	}
}

func DecodeUTF16(b []byte) (string, error) {

	if len(b)%2 != 0 {
		return "", fmt.Errorf("Must have even length byte slice")
	}

	u16s := make([]uint16, 1)

	ret := &bytes.Buffer{}

	b8buf := make([]byte, 4)

	lb := len(b)
	for i := 0; i < lb; i += 2 {
		u16s[0] = uint16(b[i]) + (uint16(b[i+1]) << 8)
		r := utf16.Decode(u16s)
		n := utf8.EncodeRune(b8buf, r[0])
		ret.Write(b8buf[:n])
	}

	return ret.String(), nil
}
