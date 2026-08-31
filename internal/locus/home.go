package locus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type HomeLayout struct {
	Root       string `json:"root"`
	Registry   string `json:"registry"`
	Objects    string `json:"objects"`
	Candidates string `json:"candidates"`
}

func DefaultHome() (string, error) {
	if value := os.Getenv("LOCUS_HOME"); value != "" {
		return filepath.Abs(value)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if base == "" {
		return "", errors.New("OS user config directory is unavailable")
	}
	return filepath.Join(base, "locus-link"), nil
}

func LocusHomeLayout() (HomeLayout, error) {
	root, err := DefaultHome()
	if err != nil {
		return HomeLayout{}, err
	}
	return HomeLayout{
		Root:       root,
		Registry:   filepath.Join(root, "registry"),
		Objects:    filepath.Join(root, "cache", "objects"),
		Candidates: filepath.Join(root, "cache", "candidates"),
	}, nil
}

func DiscoverRegistry(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(current, ".locus", "registry")
		if info, statErr := os.Stat(filepath.Join(candidate, "scope.yaml")); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no .locus/registry found from %s", start)
		}
		current = parent
	}
}
