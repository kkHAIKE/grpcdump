package main

import (
	"encoding/binary"
	"encoding/hex"
	"log"
	"strings"

	"github.com/gookit/color"
)

func (p h2Packet) CanDecode() bool {
	if len(p.Body) <= 5 || p.Body[0] != 0 {
		return false
	}

	length := binary.BigEndian.Uint32(p.Body[1:5])
	if length+5 != uint32(len(p.Body)) {
		return false
	}

	if v, ok := p.Headers["content-type"]; ok {
		return v[0] == "application/grpc" || v[0] == "application/grpc+proto"
	}

	if length >= 2 && p.Body[5] == '{' && (p.Body[6] == '"' || p.Body[6] == '}') {
		return false
	}
	// other pass to decoder
	return true
}

func (p h2Packet) CanJson() bool {
	if len(p.Body) <= 5 || p.Body[0] != 0 {
		return false
	}

	length := binary.BigEndian.Uint32(p.Body[1:5])

	if v, ok := p.Headers["content-type"]; ok {
		return v[0] == "application/grpc+json"
	}

	if length >= 2 && p.Body[5] == '{' && (p.Body[6] == '"' || p.Body[6] == '}') {
		return true
	}
	return false
}

func (p h2Packet) HeaderString() string {
	var buf strings.Builder
	// tcp 4 tuple
	buf.WriteString(color.FgLightMagenta.Sprintf("%v\n", p.Tuple))
	if p.Rst != 0 {
		buf.WriteString(color.Sprintf("%s: %v\n", color.LightRed.Sprint("ERROR"), p.Rst))
	}
	// print headers
	for k, v := range p.Headers {
		// ignore some use-less
		if k == ":method" || k == ":scheme" || k == "te" || k == "user-agent" || k == "accept" || k == "content-length" {
			continue
		}

		for _, vv := range v {
			if k == ":path" || k == "<path" {
				buf.WriteString(color.Sprintf("%s: %s\n", color.LightCyan.Sprint(k), color.LightYellow.Sprint(vv)))
			} else {
				buf.WriteString(color.Sprintf("%s: %s\n", color.LightCyan.Sprint(k), vv))
			}
		}
	}
	return buf.String()
}

func (p h2Packet) HeaderBodyString() string {
	if len(p.Body) == 0 {
		return p.HeaderString()
	}

	if p.CanJson() {

		return p.HeaderString() + color.FgLightYellow.Sprintf("%s\n", p.Body[5:])
	}

	return p.HeaderString() + color.FgLightYellow.Sprintf("%s\n", hex.Dump(p.Body))
}

func (p h2Packet) Print(path string, isReq bool) {
	// ignore reflection service call by myself
	if path == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
		return
	}
	if conf.PathFilter != nil && !conf.PathFilter.MatchString(path) {
		return
	}
	if !p.CanDecode() {
		p.DefPrint()
		return
	}

	var host string
	if isReq {
		host = p.Tuple.Dst()
	} else {
		host = p.Tuple.Src()
	}

	ret, err := pbMgr.DecodeToJsonString(host, path, isReq, p.Body[5:])
	if err != nil {
		if conf.Verbose {
			log.Println(p.Tuple, "decodeToJsonString failed", err)
		}
		p.DefPrint()
		return
	}

	color.Fprint(lkout, p.HeaderString()+color.FgLightYellow.Sprintf("%s\n", ret))
}

func (p h2Packet) DefPrint() {
	color.Fprint(lkout, p.HeaderBodyString())
}
