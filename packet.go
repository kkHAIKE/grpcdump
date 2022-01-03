package main

import (
	"encoding/binary"
	"log"
	"net/http"

	"golang.org/x/net/http2"
)

type h2Packet struct {
	Sid     uint32
	Tuple   tcp4Tuple
	Headers http.Header
	Body    []byte
	// tcp RST/FIN
	End bool
	// RstStream
	Rst http2.ErrCode
}

func (p h2Packet) Empty() bool {
	return len(p.Headers) == 0 && len(p.Body) == 0 && !p.End && p.Rst == 0
}

// TrySplit use for grpc streaming
func (p *h2Packet) TrySplit() (np *h2Packet) {
	if len(p.Body) < 5 {
		return
	}

	length := binary.BigEndian.Uint32(p.Body[1:5])
	if length+5 >= uint32(len(p.Body)) {
		return
	}

	np = &h2Packet{
		Sid:     p.Sid,
		Tuple:   p.Tuple,
		Headers: p.Headers,
		Body:    p.Body[:length+5],
		End:     p.End,
		Rst:     p.Rst,
	}
	p.Headers = make(http.Header)
	p.Body = p.Body[length+5:]
	p.End = false
	p.Rst = 0
	return
}

type h2PacketBuilder map[uint32]*h2Packet

func (hh h2PacketBuilder) Process(f http2.Frame, tuple tcp4Tuple, room *meeting) {
	switch f := f.(type) {
	case *http2.MetaHeadersFrame:
		// is ok for trailer
		p, ok := hh[f.StreamID]
		if !ok {
			p = &h2Packet{
				Sid:     f.StreamID,
				Tuple:   tuple,
				Headers: make(http.Header),
			}
			hh[f.StreamID] = p
		}

		// add dummy/lost header for on-the-fly
		var hasDummy bool
		for _, v := range f.Fields {
			if v.Value != "" {
				p.Headers[v.Name] = append(p.Headers[v.Name], v.Value)
			} else if v.Name == "dummy" {
				hasDummy = true
			}
		}
		if hasDummy {
			p.Headers["lost"] = []string{"lost"}
		}

		if f.Flags.Has(http2.FlagHeadersEndStream) {
			room.RecvPacket(p)
			delete(hh, f.StreamID)
		}
		if conf.Verbose {
			log.Println(tuple, "got MetaHeadersFrame", len(f.Fields))
		}
	case *http2.DataFrame:
		p, ok := hh[f.StreamID]
		if ok {
			if len(f.Data()) > 0 {
				p.Body = append(p.Body, f.Data()...)

				// split one-data
				for {
					np := p.TrySplit()
					if np == nil {
						break
					}

					room.RecvPacket(np)
				}
			}
		}
		if ok && f.Flags.Has(http2.FlagDataEndStream) {
			room.RecvPacket(p)
			delete(hh, f.StreamID)
		}
		if conf.Verbose {
			log.Println(tuple, "got DataFrame", len(f.Data()))
		}
	case *http2.RSTStreamFrame:
		p, ok := hh[f.StreamID]
		if ok {
			p.Rst = f.ErrCode

			room.RecvPacket(p)
			delete(hh, f.StreamID)
		}
	case *http2.GoAwayFrame:
		//TODO:
		return
	default:
		// ignore
	}
}
