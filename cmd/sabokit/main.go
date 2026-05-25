package main

import (
	"os"

	"github.com/sheyaln/sabokit-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
