package locus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocusHomeLayoutUsesWorkspaceOverride(t *testing.T) {
	root := workspaceTestPath(t, "home-layout")
	t.Setenv("LOCUS_HOME", root)
	layout, err := LocusHomeLayout()
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != root || layout.Registry != filepath.Join(root, "registry") || layout.Objects != filepath.Join(root, "cache", "objects") || layout.Candidates != filepath.Join(root, "cache", "candidates") {
		t.Fatalf("unexpected layout: %+v", layout)
	}
}

func TestRootSelectionRegistrationAndUserIsolation(t *testing.T) {
	base := workspaceTestPath(t, "root-selection")
	os.RemoveAll(base)
	home := filepath.Join(base, "home")
	statePath := filepath.Join(base, "state", "state.db")
	t.Setenv("LOCUS_HOME", home)
	t.Setenv("LOCUS_STATE_PATH", statePath)
	writeNamedRegistry(t, home, "registry", manifestYAML("scope.user", ""), map[string]string{"entities/user.yaml": entityYAML("user")})
	projectRoot := filepath.Join(base, "project", ".locus", "registry")
	writeModelFile(t, filepath.Join(projectRoot, "scope.yaml"), manifestYAML("scope.project", "imports:\n  user:\n    scope_id: scope.user\n    source:\n      kind: directory\n      uri: ${LOCUS_HOME}/registry\n"))
	writeModelFile(t, filepath.Join(projectRoot, "entities", "project.yaml"), entityYAML("project"))
	store, err := OpenStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	nested := filepath.Join(base, "project", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	registry, root, err := LoadRegistryContext("", store)
	if err != nil {
		t.Fatal(err)
	}
	if root.RootOrigin != "project" || root.Registered || !root.HasUserImport || registry.Entities["scope.user::user"] == nil {
		t.Fatalf("unexpected project root context: %+v", root)
	}
	if _, err := store.RegisterProject(context.Background(), projectRoot, home); err != nil {
		t.Fatal(err)
	}
	_, root, err = LoadRegistryContext("", store)
	if err != nil {
		t.Fatal(err)
	}
	if !root.Registered {
		t.Fatal("registered project was not reported")
	}
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	registry, root, err = LoadRegistryContext("", store)
	if err != nil {
		t.Fatal(err)
	}
	if root.RootOrigin != "user" || registry.RootScopeID != "scope.user" || registry.Entities["scope.project::project"] != nil {
		t.Fatalf("registration merged project declarations into user root: root=%+v entities=%+v", root, registry.Entities)
	}
}

func TestProjectRegistrationTracksMovedAndDeletedPaths(t *testing.T) {
	base := workspaceTestPath(t, "registration-move")
	os.RemoveAll(base)
	home := filepath.Join(base, "home")
	project := writeNamedRegistry(t, base, "project", manifestYAML("scope.project", ""), nil)
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	registration, err := store.RegisterProject(context.Background(), project, home)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(base, "moved")
	if err := os.Rename(project, moved); err != nil {
		t.Fatal(err)
	}
	projects, err := store.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Available {
		t.Fatalf("moved project still reported available: %+v", projects)
	}
	updated, err := store.RegisterProject(context.Background(), moved, home)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ScopeID != registration.ScopeID || updated.RegistryPath != moved {
		t.Fatalf("registration was not updated: %+v", updated)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
	projects, err = store.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Available {
		t.Fatalf("deleted project still reported available: %+v", projects)
	}
	removed, err := store.UnregisterProject(context.Background(), registration.ScopeID)
	if err != nil || !removed {
		t.Fatalf("unregister failed: removed=%v err=%v", removed, err)
	}
}
