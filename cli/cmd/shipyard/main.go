package main

import (
	"os"

	"github.com/philiprehberger/shipyard/cli/internal/cmdcli"
)

func main() {
	os.Exit(cmdcli.Execute(os.Stdout, os.Stderr, os.Args[1:]))
}
