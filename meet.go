package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// meeting-room manager tcp-link
type meeting struct {
	Time    time.Time
	Creater tcp4Tuple
	/*Flag1*/ Flag2 bool
	Flag3, Flag4    bool
	Flag5, Flag6    uint32

	one      sync.Once
	recv     chan *h2Packet
	doneResp chan struct{}
}

func newMeeting(tuple tcp4Tuple, t time.Time) (m *meeting) {
	m = &meeting{
		Time:    t,
		Creater: tuple,
		// Flag1:   1,
	}
	return
}

func (m *meeting) Wait() {
	if m.doneResp != nil {
		<-m.doneResp
	}
}

func (m *meeting) RecvPacket(p *h2Packet) {
	m.one.Do(func() {
		m.recv = make(chan *h2Packet, 1024)
		m.doneResp = make(chan struct{})
		go m.process()
	})

	m.recv <- p
}

type h2Stream struct {
	Path     string // current grpc stream path
	ReqTuple tcp4Tuple
	Packets  []*h2Packet // temp save if no path
	Pass     bool
}

func (s *h2Stream) Add(p *h2Packet) {
	if s.Path == "" {
		if path, ok := p.Headers[":path"]; ok {
			s.Path = path[0]
			s.ReqTuple = p.Tuple
		}
	}
	if s.Path == "" {
		if s.Pass {
			if !conf.HideNoPath {
				p.DefPrint()
			}
			return
		}
		s.Packets = append(s.Packets, p)
		if len(s.Packets) == 3 {
			s.Pass = true
			if !conf.HideNoPath {
				for _, v := range s.Packets {
					v.DefPrint()
				}
			}
			s.Packets = nil
		}
		return
	}
	packPrint := func(p *h2Packet) {
		isReq := p.Tuple == s.ReqTuple
		if isReq {
			// may be no work
			if _, ok := p.Headers[":path"]; !ok {
				p.Headers[":path"] = []string{s.Path}
			}
		} else {
			// <path (for response) diff :path
			p.Headers["<path"] = []string{s.Path}
		}
		p.Print(s.Path, isReq)
	}
	if len(s.Packets) > 0 {
		for _, v := range s.Packets {
			packPrint(v)
		}
		s.Packets = nil
	}
	packPrint(p)
}

func (m *meeting) process() {
	defer close(m.doneResp)

	var flag1, flag2 uint32
	streams := make(map[uint32]*h2Stream)

	for p := range m.recv {
		if p.Empty() {
			continue
		}
		if p.End {
			if m.Creater == p.Tuple {
				flag1 = 1
			} else {
				flag2 = 1
			}
			if conf.Verbose {
				log.Println(p.Tuple, "got packet end")
			}
			if atomic.LoadUint32(&m.Flag5) == flag1 && atomic.LoadUint32(&m.Flag6) == flag2 {
				return
			}
		}

		s, ok := streams[p.Sid]
		if !ok {
			s = &h2Stream{}
			streams[p.Sid] = s
		}
		s.Add(p)
	}
}

type meetingRooms map[tupleKey]*meeting

func (rs meetingRooms) Enter(tuple tcp4Tuple, t time.Time) {
	key := tuple.Key()
	if m, ok := rs[key]; !ok {
		rs[key] = newMeeting(tuple, t)
		if conf.Verbose {
			log.Println(tuple, "create meeting room")
		}
	} else if !m.Flag2 && m.Creater != tuple {
		m.Flag2 = true
		if conf.Verbose {
			log.Println(tuple, "set meeting flag2")
		}
	}
}

func (rs meetingRooms) Exit(tuple tcp4Tuple) {
	key := tuple.Key()
	if m, ok := rs[key]; ok {
		if m.Creater == tuple {
			m.Flag3 = true
		} else {
			m.Flag4 = true
		}
		if m.Flag3 == true && m.Flag2 == m.Flag4 {
			delete(rs, key)
			if conf.Verbose {
				log.Println(tuple, "delete meeting room")
			}
		}
	}
}

func (m *meeting) Talk(tuple tcp4Tuple) {
	if atomic.LoadUint32(&m.Flag5) == 0 && m.Creater == tuple {
		atomic.StoreUint32(&m.Flag5, 1)
		if conf.Verbose {
			log.Println(tuple, "set meeting flag5")
		}
	} else if atomic.LoadUint32(&m.Flag6) == 0 && m.Creater != tuple {
		atomic.StoreUint32(&m.Flag6, 1)
		if conf.Verbose {
			log.Println(tuple, "set meeting flag6")
		}
	}
}

func (rs meetingRooms) Wait() {
	for _, v := range rs {
		v.Wait()
	}
}
