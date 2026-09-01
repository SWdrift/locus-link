package main

import (
	"os"

	"locus-link/internal/cli"
	"locus-link/internal/web"
	webui "locus-link/internal/web/ui"
)

func main() {
	os.Exit(cli.NewCLI(os.Stdout, os.Stderr, web.Command(os.Stdout, webui.Handler)).Run(os.Args[1:]))
}
