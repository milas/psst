package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cqroot/prompt"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		os.Exit(1)
	}()

	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	cmd, _ := NewCommand(streams)
	if err := cmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, prompt.ErrUserQuit) {
			os.Exit(2)
			return
		}
		_, _ = fmt.Fprintf(streams.ErrOut, "Error: %s\n", err)
		os.Exit(1)
	}
}
