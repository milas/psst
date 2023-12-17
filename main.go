package main

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cqroot/prompt"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streams := genericiooptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		time.AfterFunc(
			time.Second, func() {
				os.Exit(4)
			},
		)
	}()

	cmd, _ := NewCommand(streams)
	if err := cmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(2)
			return
		}
		if errors.Is(err, context.Canceled) {
			os.Exit(3)
			return
		}
		_, _ = fmt.Fprintf(streams.ErrOut, "Error: %s\n", err)
		os.Exit(1)
	}
}
