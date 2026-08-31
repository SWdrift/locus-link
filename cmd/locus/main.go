package main

import (
	"os"

	"locus-link/internal/cli"
)

func main() {
	os.Exit(cli.NewCLI(os.Stdout, os.Stderr).Run(os.Args[1:]))
}
