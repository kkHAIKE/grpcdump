package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"log"
	"sync"
	"unsafe"

	_ "github.com/gogo/protobuf/gogoproto"
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/grpcreflect"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// copy from grpcreflect
type reflectClient struct {
	ctx  context.Context
	stub rpb.ServerReflectionClient

	connMu sync.Mutex
	cancel context.CancelFunc
	stream rpb.ServerReflection_ServerReflectionInfoClient

	cacheMu      sync.RWMutex
	protosByName map[string]*dpb.FileDescriptorProto
	// filesByName      map[string]*desc.FileDescriptor
	// filesBySymbol    map[string]*desc.FileDescriptor
	// filesByExtension map[extDesc]*desc.FileDescriptor
}

// fix gogo.proto include
func modifyClient(cli *grpcreflect.Client) {
	rcli := (*reflectClient)(unsafe.Pointer(cli))

	fd := getGogo()
	// fix "gogo.proto" to full path
	*fd.Name = "github.com/gogo/protobuf/gogoproto/gogo.proto"
	rcli.protosByName["github.com/gogo/protobuf/gogoproto/gogo.proto"] = fd
}

func getGogo() *dpb.FileDescriptorProto {
	data := gogoproto.FileDescriptor("gogo.proto")
	if len(data) == 0 {
		return nil
	}

	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		log.Println("getGogo NewReader failed", err)
		return nil
	}

	data, err = ioutil.ReadAll(r)
	if err != nil {
		log.Println("getGogo ReadAll failed", err)
		return nil
	}

	ret := &dpb.FileDescriptorProto{}
	if err := proto.Unmarshal(data, ret); err != nil {
		log.Println("getGogo Unmarshal failed", err)
		return nil
	}
	return ret
}
