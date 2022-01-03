package main

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/urfave/cli/v2"
)

var conf *Config

type Config struct {
	PathFilter *regexp.Regexp
	ForceHttp2 bool
	Verbose    bool
	HideNoPath bool
	ProtoInc   []string
	ProtoFile  []string
}

func newConfig(ctx *cli.Context) (c *Config, err error) {
	c = &Config{
		ForceHttp2: ctx.Bool("force"),
		Verbose:    ctx.Bool("verbose"),
		HideNoPath: ctx.Bool("hide-no-path"),
		ProtoInc:   ctx.StringSlice("proto-include"),
		ProtoFile:  ctx.StringSlice("proto-file"),
	}
	if tmp := ctx.String("path-regex"); tmp != "" {
		if c.PathFilter, err = regexp.Compile(tmp); err != nil {
			return
		}
	}

	// if ctx.Bool("verbose") {
	// 	flag.Set("assembly_debug_log", "true")
	// }
	return
}

func capAction(ctx *cli.Context) (err error) {
	if conf, err = newConfig(ctx); err != nil {
		return
	}

	if pbMgr, err = newPbManager(ctx.Context); err != nil {
		return
	}

	handle, err := pcap.OpenLive(ctx.String("interface"), int32(ctx.Int("snapshot-length")), false, pcap.BlockForever)
	if err != nil {
		return
	}
	defer handle.Close()

	if ctx.Args().Present() {
		if err = handle.SetBPFFilter(strings.Join(ctx.Args().Slice(), " ")); err != nil {
			return
		}
	}
	source := gopacket.NewPacketSource(handle, handle.LinkType())

	sf := newStreamFactory(ctx)
	defer sf.Close()
	streamPool := tcpassembly.NewStreamPool(sf)
	assembler := tcpassembly.NewAssembler(streamPool)

	// use 'flush' to force tcpassembly trigger callback
	// if on-the-fly(tcp has been connected)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	lastFlush := time.Now()
	flush := func() {
		if now := time.Now(); now.Sub(lastFlush) >= 3*time.Second {
			assembler.FlushWithOptions(tcpassembly.FlushOptions{T: lastFlush})
			lastFlush = now
		}
	}

	for {
		select {
		case <-ctx.Done():
			assembler.FlushAll()
			if conf.Verbose {
				log.Println("got ctx.Done")
			}
			return

		case <-tick.C:
			flush()

		case packet, ok := <-source.Packets():
			flush()
			if !ok {
				return
			}

			// ignore other protocol
			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil ||
				packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
				continue
			}

			netFlow := packet.NetworkLayer().NetworkFlow()
			tcp := packet.TransportLayer().(*layers.TCP)
			seen := packet.Metadata().Timestamp

			// use meeting-room to link two way
			tuple := newTcp4Tuple(netFlow, tcp.TransportFlow())
			if tcp.RST || tcp.FIN {
				sf.rooms.Exit(tuple)
			} else {
				sf.rooms.Enter(tuple, seen)
			}

			assembler.AssembleWithTimestamp(netFlow, tcp, seen)
		}
	}
}

type streamFactory struct {
	rooms meetingRooms

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func newStreamFactory(ctx *cli.Context) (sf *streamFactory) {
	sf = &streamFactory{
		rooms: make(meetingRooms),
	}
	sf.ctx, sf.cancel = context.WithCancel(ctx.Context)
	return
}

func (sf *streamFactory) Close() {
	sf.cancel()
	sf.rooms.Wait()
	sf.wg.Wait()
}

func (sf *streamFactory) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	tuple := newTcp4Tuple(netFlow, tcpFlow)
	room, ok := sf.rooms[tuple.Key()]
	if !ok {
		if conf.Verbose {
			log.Println(tuple, "no meeting room")
		}
		// ignore this stream
		ret := tcpreader.NewReaderStream()
		sf.wg.Add(1)
		go func() {
			defer sf.wg.Done()
			ret.Close()
		}()
		return &ret
	}

	s := newTcpStream(tuple, room)
	sf.wg.Add(1)
	go s.Process(sf.wg.Done)
	return s
}
