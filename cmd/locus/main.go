package main

import (
	"os"

	"locus-link/internal/locus"
)

func main() {
	os.Exit(locus.NewCLI(os.Stdout, os.Stderr).Run(os.Args[1:]))
}
