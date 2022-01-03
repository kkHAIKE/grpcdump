package main

import (
	"context"
	"flag"
	"net"

	"grpcdump/test/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func cli() {
	c, err := grpc.Dial("127.0.0.1:9000", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	cli := pb.NewFooClient(c)
	cli.Bar(context.Background(), &pb.BarReq{Q: "alice"})
}

var ref = flag.Bool("ref", true, "use reflection")
var client = flag.Bool("client", false, "client mode")

func srv() {
	srv := grpc.NewServer()
	pb.RegisterFooServer(srv, &server{})

	// reflection service
	if *ref {
		reflection.Register(srv)
	}

	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		panic(err)
	}
	srv.Serve(ln)
}

type server struct{}

func (*server) Bar(ctx context.Context, req *pb.BarReq) (resp *pb.BarResp, err error) {
	resp = &pb.BarResp{R: "bob"}
	return
}

func main() {
	flag.Parse()
	if *client {
		cli()
		return
	}
	srv()
}
