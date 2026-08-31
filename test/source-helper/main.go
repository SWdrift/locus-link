package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const markerName = ".locus-source-helper-commit"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "source helper failed")
		os.Exit(1)
	}
}

func run(args []string) error {
	if logPath := os.Getenv("LOCUS_SOURCE_HELPER_LOG"); logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(file, strings.Join(args, " "))
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if len(args) == 5 && args[0] == "clone" && args[1] == "--no-checkout" && args[2] == "--" {
		return clone(args[3], args[4])
	}
	if len(args) == 5 && args[0] == "-C" && args[2] == "rev-parse" && args[3] == "--verify" {
		commit, err := os.ReadFile(filepath.Join(args[1], markerName))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, strings.TrimSpace(string(commit)))
		return err
	}
	if len(args) == 6 && args[0] == "-C" && args[2] == "checkout" && args[3] == "--detach" && args[4] == "--force" {
		commit, err := os.ReadFile(filepath.Join(args[1], markerName))
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(commit)) != args[5] {
			return errors.New("unknown commit")
		}
		return os.Remove(filepath.Join(args[1], markerName))
	}
	return errors.New("unsupported invocation")
}

func clone(rawURI, destination string) error {
	source, err := fileURIPath(rawURI)
	if err != nil {
		return err
	}
	commit, err := os.ReadFile(source + ".commit")
	if err != nil {
		return err
	}
	if err := copyTree(source, destination); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destination, markerName), commit, 0o644)
}

func fileURIPath(rawURI string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "file" {
		return "", errors.New("source helper accepts only file URLs")
	}
	path := filepath.FromSlash(parsed.Path)
	if runtime.GOOS == "windows" && len(path) >= 3 && (path[0] == '\\' || path[0] == '/') && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(path), nil
}

func copyTree(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlinks are not supported")
		}
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return outputCloseErr
	})
}
