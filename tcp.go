package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"sync"

	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

var clientPrefix = []byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")

const clientPrefixLen = 24

// tcpStream just one way
type tcpStream struct {
	tcpreader.ReaderStream
	tuple tcp4Tuple
	room  *meeting

	// on-the-fly, no SYN got
	noSyn    bool
	firstChk bool
}

func newTcpStream(tuple tcp4Tuple, room *meeting) *tcpStream {
	return &tcpStream{
		ReaderStream: tcpreader.NewReaderStream(),
		tuple:        tuple,
		room:         room,
	}
}

func (ts *tcpStream) Process(done func()) {
	defer done()
	var err error
	defer func() {
		// if not EOF error so ignore packet after
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			// ts.ReaderStream.Close() can't use after Read()
			io.Copy(ioutil.Discard, &ts.ReaderStream)
		}
	}()

	var data [clientPrefixLen]byte
	if _, err = io.ReadFull(&ts.ReaderStream, data[:]); err != nil {
		if conf.Verbose && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println(ts.tuple, "io.ReadFull failed")
		}
		return
	}

	if ts.noSyn {
		if conf.ForceHttp2 {
			// try get the HeadersFrame
			if data, err1 := skipToHeadersFrame(data[:], &ts.ReaderStream); err1 != nil {
				err = err1
				if conf.Verbose {
					log.Println(ts.tuple, "skipToHeadersFrame failed")
				}
			} else {
				if conf.Verbose {
					log.Println(ts.tuple, "skipToHeadersFrame pass")
				}
				err = ts.processHttp2(io.MultiReader(bytes.NewReader(data), &ts.ReaderStream))
			}
		}
	} else if bytes.Equal(data[:], clientPrefix) {
		// client need skip h2c prefix
		if conf.Verbose {
			log.Println(ts.tuple, "got client h2c")
		}
		err = ts.processHttp2(&ts.ReaderStream)
	} else if isServerSettingFrame(data[:]) {
		// server first frame must be SettingFrame
		if conf.Verbose {
			log.Println(ts.tuple, "got server h2c")
		}
		err = ts.processHttp2(io.MultiReader(bytes.NewReader(data[:]), &ts.ReaderStream))
	} else if conf.Verbose {
		log.Println(ts.tuple, "not support")
	}
}

var dummyHeader []byte
var dummyOne sync.Once

func getDummyHeader() []byte {
	dummyOne.Do(func() {
		var buf bytes.Buffer
		w := hpack.NewEncoder(&buf)
		w.WriteField(hpack.HeaderField{Name: "dummy"})
		dummyHeader = bytes.Repeat(buf.Bytes(), 100)
	})
	return dummyHeader
}

func (ts *tcpStream) processHttp2(r io.Reader) (err error) {
	// talk in the meeting-room (hahaha)
	ts.room.Talk(ts.tuple)
	defer func() {
		ts.room.RecvPacket(&h2Packet{Tuple: ts.tuple, End: true})
	}()

	framer := http2.NewFramer(ioutil.Discard, r)
	// framer.SetReuseFrames()
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	framer.ReadMetaHeaders.SetEmitEnabled(false)

	if ts.noSyn {
		// on-the-fly need feed dummy header make sure decoder no-error
		if _, err1 := framer.ReadMetaHeaders.Write(getDummyHeader()); err1 != nil {
			panic(err1)
		}
	}

	bulder := make(h2PacketBuilder)

	for {
		f, err1 := framer.ReadFrame()
		if err1 != nil {
			err = err1
			if conf.Verbose {
				log.Println(ts.tuple, "framer.ReadFrame failed", err)
			}
			return
		}

		bulder.Process(f, ts.tuple, ts.room)
	}
}

func (ts *tcpStream) Reassembled(reassembly []tcpassembly.Reassembly) {
	// detect on-the-fly
	if !ts.firstChk {
		ts.firstChk = true
		if !reassembly[0].Start {
			ts.noSyn = true
		}
	}
	ts.ReaderStream.Reassembled(reassembly)
}
