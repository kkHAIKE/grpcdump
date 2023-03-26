# grpcdump
This is a tcpdump-like tool for automatically decoding Protobuf in the gRPC h2c protocol.

## feature
- [x] h2c capture & decode
- [x] auto decoding of Protobuf in gRPC using the [Reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) service
- [x] manual specification of the Proto file if reflection is not registered
- [ ] simple BPF compiler for pure go build (linux only)

## preview
`<path` header is add by this tool
![show](capture.png)

## usage
This tool has one required parameter `-i`, which allows you to choose your interface, similar to *tcpdump*.

However, in production environments, you should use [pcap-filter](https://www.tcpdump.org/manpages/pcap-filter.7.html) to reduce memory consumption, just like *tcpdump*.

By default, this tool does not decode the TCP stream on-the-fly (since HPACK requires the TCP connection to be established before capture).

| parameter | short | description |
|-|-|-|
|interface|i|same as tcpdump
|snapshot-length|s|same as tcpdump
|path-regex|P|focus to show
|force||enable on-the-fly decode(use with pcap-filter)
|hide-no-path||non-path packet can't decode
|proto-include|I|use like protoc -I
|proto-file|f|proto relative path about proto-include

### some case
use with my test cmd

1. `./test` + `./test -client`:
    `grpcdump -i any -P "/test\.Foo/Bar" "host 127.0.0.1 and port 9000"`
2. `./test -ref=false` + `./test -client`:
    `grpcdump -i lo0 -P "/test\.Foo/Bar" -I .. -f grpcdump/test/pb/foo.proto "host 127.0.0.1 and port 9000"`

## require
1. **mac/win**: install [Wireshark](https://www.wireshark.org/download.html)
    or only install *ChmodBPF.pkg*
2. **linux**: need *libpcap-dev*
