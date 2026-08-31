package main

import (
	"os"

	"locus-link/internal/cli"
	"locus-link/internal/web"
)

func main() {
	os.Exit(cli.NewCLI(os.Stdout, os.Stderr, web.Command(os.Stdout)).Run(os.Args[1:]))
}
