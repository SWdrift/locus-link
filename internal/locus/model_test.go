package locus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRegistryWalksUpWithPlatformPaths(t *testing.T) {
	root := workspaceTestPath(t, "registry-discovery")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, ".locus", "registry")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registry, "scope.yaml"), []byte("api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "nested", "package")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	discovered, err := DiscoverRegistry(nested)
	if err != nil {
		t.Fatal(err)
	}
	if discovered != registry {
		t.Fatalf("expected %s, got %s", registry, discovered)
	}
}
