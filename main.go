package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gookit/color"
	"github.com/urfave/cli/v2"
)

type lockOut struct {
	lk sync.Mutex
}

func (w *lockOut) Write(p []byte) (n int, err error) {
	w.lk.Lock()
	defer w.lk.Unlock()
	return os.Stdout.Write(p)
}

var lkout = &lockOut{}

func main() {
	app := cli.NewApp()
	app.Usage = "grpc protocol capture & decode"
	app.Version = "v1.0.0"
	app.Authors = []*cli.Author{{
		Name:  "kkhaike",
		Email: "kkhaike@gmail.com",
	}}

	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:    "snapshot-length",
			Aliases: []string{"s"},
			Value:   262144,
		},
		&cli.StringFlag{
			Name:     "interface",
			Aliases:  []string{"i"},
			Required: true,
		},
		&cli.StringFlag{
			Name:    "path-regex",
			Aliases: []string{"P"},
			Usage:   "focus to show",
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "enable on-the-fly(use with BPFilter)",
		},
		&cli.BoolFlag{
			Name:  "hide-no-path",
			Usage: "non-path packet can't decode",
		},
		&cli.StringSliceFlag{
			Name:    "proto-include",
			Aliases: []string{"I"},
			Usage:   "use like protoc -I",
		},
		&cli.StringSliceFlag{
			Name:    "proto-file",
			Aliases: []string{"f"},
			Usage:   "proto relative path about proto-include",
		},
		&cli.BoolFlag{
			Name: "verbose",
		},
	}

	app.ArgsUsage = "BPFilter"
	app.Action = capAction

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-c
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil {
		color.Errorln(err)
	}
}
