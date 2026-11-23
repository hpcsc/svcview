package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hpcsc/svcview/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	os.Exit(cli.Run(ctx))
}
