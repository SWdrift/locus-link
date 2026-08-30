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

func TestLoadRegistryRejectsNonStrictObjectYAML(t *testing.T) {
	cases := map[string]string{
		"unknown field":      "api_version: locus/v0\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\ntypo: rejected\n",
		"duplicate key":      "api_version: locus/v0\ntype: entity\nid: workstation\nid: duplicate\nkind: workstation\nname: Workstation\n",
		"multiple documents": "api_version: locus/v0\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n---\ntype: entity\nid: second\n",
	}
	for name, object := range cases {
		t.Run(name, func(t *testing.T) {
			root := workspaceTestPath(t, "strict-yaml", name)
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
				t.Fatal(err)
			}
			manifest := []byte("api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\n")
			if err := os.WriteFile(filepath.Join(root, "scope.yaml"), manifest, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "entities", "invalid.yaml"), []byte(object), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadRegistry(root); err == nil {
				t.Fatalf("expected %s to be rejected", name)
			}
		})
	}
}
