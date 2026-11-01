package main

import (
	"os"

	"github.com/OrenVill/Argus/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
