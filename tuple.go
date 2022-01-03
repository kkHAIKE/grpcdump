package main

import (
	"fmt"

	"github.com/google/gopacket"
)

type tupleKey [2]gopacket.Flow

type tcp4Tuple struct {
	netSrc, netDst, tcpSrc, tcpDst gopacket.Endpoint
}

func newTcp4Tuple(netFlow, tcpFlow gopacket.Flow) tcp4Tuple {
	netSrc, netDst := netFlow.Endpoints()
	tcpSrc, tcpDst := tcpFlow.Endpoints()
	return tcp4Tuple{netSrc, netDst, tcpSrc, tcpDst}
}

func (tt tcp4Tuple) String() string {
	return fmt.Sprintf("%v:%v->%v:%v", tt.netSrc, tt.tcpSrc, tt.netDst, tt.tcpDst)
}

func (tt tcp4Tuple) Src() string {
	return fmt.Sprintf("%v:%v", tt.netSrc, tt.tcpSrc)
}

func (tt tcp4Tuple) Dst() string {
	return fmt.Sprintf("%v:%v", tt.netDst, tt.tcpDst)
}

func (tt tcp4Tuple) srcKey() tupleKey {
	return tupleKey{
		gopacket.NewFlow(tt.netSrc.EndpointType(), tt.netSrc.Raw(), tt.netDst.Raw()),
		gopacket.NewFlow(tt.tcpSrc.EndpointType(), tt.tcpSrc.Raw(), tt.tcpDst.Raw()),
	}
}

func (tt tcp4Tuple) revKey() tupleKey {
	return tupleKey{
		gopacket.NewFlow(tt.netSrc.EndpointType(), tt.netDst.Raw(), tt.netSrc.Raw()),
		gopacket.NewFlow(tt.tcpSrc.EndpointType(), tt.tcpDst.Raw(), tt.tcpSrc.Raw()),
	}
}

// use lower addr/port for Key
func (tt tcp4Tuple) Key() tupleKey {
	if tt.netSrc.LessThan(tt.netDst) {
		return tt.srcKey()
	}
	if tt.netDst.LessThan(tt.netSrc) {
		return tt.revKey()
	}
	if tt.tcpDst.LessThan(tt.tcpSrc) {
		return tt.revKey()
	}
	return tt.srcKey()
}
