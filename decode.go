package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type pbManager struct {
	ctx  context.Context
	svrs sync.Map
}

type svrDescRemote struct {
	desc *desc.ServiceDescriptor
	one  sync.Once
}

type svrDescLocal struct {
	desc *desc.ServiceDescriptor
}

var pbMgr *pbManager

func newPbManager(ctx context.Context) (m *pbManager, err error) {
	m = &pbManager{ctx: ctx}
	if len(conf.ProtoFile) > 0 {
		parser := protoparse.Parser{
			ImportPaths: conf.ProtoInc,
			// try
			InferImportPaths: true,
		}
		fds, err1 := parser.ParseFiles(conf.ProtoFile...)
		if err1 != nil {
			err = err1
			return
		}
		for _, v := range fds {
			for _, srv := range v.GetServices() {
				m.svrs.Store(srv.GetFullyQualifiedName(), &svrDescLocal{desc: srv})
			}
		}
	}

	return
}

func (m *pbManager) getSvrDesc(host, svr string) (_ *desc.ServiceDescriptor, err error) {
	val, ok := m.svrs.Load(svr)
	if !ok {
		val, _ = m.svrs.LoadOrStore(svr, &svrDescRemote{})
	}
	if d, ok := val.(*svrDescLocal); ok {
		return d.desc, nil
	}
	d := val.(*svrDescRemote)
	d.one.Do(func() {
		c, err1 := grpc.Dial(host, grpc.WithInsecure())
		if err1 != nil {
			err = err1
			return
		}
		cli := grpcreflect.NewClient(m.ctx, rpb.NewServerReflectionClient(c))
		defer cli.Reset()

		// fix gogo.proto include
		modifyClient(cli)
		d.desc, err = cli.ResolveService(svr)
	})
	if err != nil {
		return
	}
	return d.desc, nil
}

func (m *pbManager) DecodeToJsonString(host, path string, isReq bool, data []byte) (_ []byte, err error) {
	idx := strings.IndexByte(path[1:], '/')
	if idx == -1 {
		err = fmt.Errorf("invaild path: %s", path)
		return
	}
	svr, method := path[1:idx+1], path[idx+2:]

	svrDesc, err := m.getSvrDesc(host, svr)
	if err != nil {
		return
	}
	if svrDesc == nil {
		err = fmt.Errorf("service not found: %s", svr)
		return
	}
	mDesc := svrDesc.FindMethodByName(method)
	if mDesc == nil {
		err = fmt.Errorf("method not found: %s", method)
		return
	}

	var msg *dynamic.Message
	if isReq {
		msg = dynamic.NewMessage(mDesc.GetInputType())
	} else {
		msg = dynamic.NewMessage(mDesc.GetOutputType())
	}
	if err = msg.Unmarshal(data); err != nil {
		return
	}
	return msg.MarshalJSON()
}
