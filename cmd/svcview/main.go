package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hpcsc/svcview/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(ctx))
}
