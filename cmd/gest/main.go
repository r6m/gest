package main

import (
	"context"
	"os"

	"github.com/r6m/gest/internal/cli"
)

func main() {
	command := cli.New()
	code := command.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}
