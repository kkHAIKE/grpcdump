package main

import (
	"encoding/binary"
	"errors"
	"io"

	"golang.org/x/net/http2"
)

const (
	minMaxFrameSize         = 1 << 14
	maxFrameSize            = 1<<24 - 1
	defaultMaxStreamsClient = 100

	headerChkBufSize = 4096
)

type h2Frame struct {
	Length uint32
	Type   byte
	Flags  byte
	R      byte
	Sid    uint32
}

func decodeFrame(f *h2Frame, data []byte) {
	f.Length = (uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2]))
	f.Type = data[3]
	f.Flags = data[4]
	f.R = data[5] & (1 << 7)
	f.Sid = binary.BigEndian.Uint32(data[5:]) & (1<<31 - 1)
}

func (f h2Frame) SimpleCheck() bool {
	return f.Length <= minMaxFrameSize && f.R == 0
}

func skipToHeadersFrame(prefix []byte, r io.Reader) (_ []byte, err error) {
	var buf [headerChkBufSize]byte
	copy(buf[:len(prefix)], prefix)
	n, err := io.ReadFull(r, buf[len(prefix):])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return
	}
	data := buf[:n]

	var f h2Frame
	for i := 0; i < len(data)-9; i++ {
		decodeFrame(&f, data[i:])

		// check vaild HeadersFrame
		if f.SimpleCheck() && f.Length > 0 &&
			f.Type == byte(http2.FrameHeaders) &&
			f.Flags & ^byte(http2.FlagHeadersEndStream|http2.FlagHeadersEndHeaders|
				http2.FlagHeadersPadded|http2.FlagHeadersPriority) == 0 &&
			f.Sid > 0 && f.Sid <= defaultMaxStreamsClient &&
			// client sid is Odd
			f.Sid&1 == 1 {
			return data[i:], nil
		}
	}
	err = errors.New("tried my best")
	return
}

func isServerSettingFrame(prefix []byte) bool {
	var f h2Frame
	decodeFrame(&f, prefix)
	if !(f.SimpleCheck() && f.Type == byte(http2.FrameSettings) &&
		f.Flags & ^byte(http2.FlagSettingsAck) == 0 &&
		f.Sid == 0) {
		return false
	}

	// ack
	if f.Flags&byte(http2.FlagSettingsAck) != 0 {
		return f.Length == 0
	}

	// maybe five items?
	return f.Length%6 == 0 && f.Length <= 5*6
}
