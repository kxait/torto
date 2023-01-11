package main

import (
	"os"

	"github.com/kxait/torto/torto"
)

func main() {
	cmd := torto.CreateCommand()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
