package main

import (
	"context"
	"net"
	"os"

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

func srv() {
	srv := grpc.NewServer()
	pb.RegisterFooServer(srv, &server{})

	// reflection service
	reflection.Register(srv)

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
	if len(os.Args) > 1 && os.Args[1] == "client" {
		cli()
		return
	}
	srv()
}
