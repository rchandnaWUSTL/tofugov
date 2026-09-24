package main

import (
	"os"

	"github.com/rchandnaWUSTL/tofugov/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
