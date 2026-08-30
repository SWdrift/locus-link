package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	root := os.Getenv("LOCUS_SIM_ROOT")
	if logPath := os.Getenv("LOCUS_SIM_LOG"); logPath != "" {
		log, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if _, err := fmt.Fprintln(log, name); err != nil {
			_ = log.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := log.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	state := ""
	switch {
	case strings.HasPrefix(name, "frpc"):
		state = "frp-up"
	case strings.HasPrefix(name, "ssh"):
		state = "ssh-up"
	case strings.HasPrefix(name, "salt"):
		state = "salt-up"
	default:
		fmt.Fprintln(os.Stderr, "unknown simulated tool")
		os.Exit(2)
	}
	if _, err := os.Stat(filepath.Join(root, state)); err != nil {
		fmt.Fprintf(os.Stderr, "%s is unavailable\n", state)
		os.Exit(1)
	}
	fmt.Printf("simulated %s ok\n", state)
}
