package main

import (
	"os"

	"github.com/fission-ai/detergent/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
