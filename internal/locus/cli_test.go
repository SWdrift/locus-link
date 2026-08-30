package locus

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeclarationCommandsDoNotAssembleRuntimeOrState(t *testing.T) {
	root := workspaceTestPath(t, "cli-declaration-boundary")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, "registry")
	writeDeclarationTestRegistry(t, registry)
	emptyPath := filepath.Join(root, "empty-path")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", emptyPath)
	if executable, err := exec.LookPath("ssh"); err == nil {
		t.Fatalf("test PATH unexpectedly contains ssh at %s", executable)
	}

	t.Run("state path is not resolved", func(t *testing.T) {
		for _, name := range []string{"LOCUS_STATE_PATH", "XDG_STATE_HOME", "HOME", "LOCALAPPDATA", "USERPROFILE", "HOMEDRIVE", "HOMEPATH"} {
			t.Setenv(name, "")
		}
		assertDeclarationCommands(t, registry)
	})

	t.Run("configured state path is untouched", func(t *testing.T) {
		statePath := filepath.Join(root, "configured-state", "state.db")
		t.Setenv("LOCUS_STATE_PATH", statePath)
		assertDeclarationCommands(t, registry)
		if _, err := os.Stat(filepath.Dir(statePath)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("declaration command touched configured state directory: %v", err)
		}
	})
}

func assertDeclarationCommands(t *testing.T, registry string) {
	t.Helper()
	validate := runInternalCLI(t, "validate", "--registry", registry, "--json")
	if valid, ok := validate["valid"].(bool); !ok || !valid {
		t.Fatalf("unexpected validate result: %#v", validate)
	}

	list := runInternalCLI(t, "list", "link", "--registry", registry, "--json")
	objects, ok := list["objects"].([]any)
	if !ok || len(objects) != 1 || objects[0] != "project.test::connection" {
		t.Fatalf("unexpected list result: %#v", list)
	}

	show := runInternalCLI(t, "show", "connection", "--registry", registry, "--json")
	if show["canonical_id"] != "project.test::connection" || show["ref_type"] != "link" {
		t.Fatalf("unexpected show result: %#v", show)
	}
}

func runInternalCLI(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := NewCLI(&stdout, &stderr).Run(args); exitCode != 0 {
		t.Fatalf("locus %v exited %d: %s", args, exitCode, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode locus %v output: %v\n%s", args, err, stdout.String())
	}
	return result
}

func writeDeclarationTestRegistry(t *testing.T, root string) {
	t.Helper()
	for _, directory := range []string{"entities", "links"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"scope.yaml": "api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v0\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
		filepath.Join("entities", "server.yaml"):      "api_version: locus/v0\ntype: entity\nid: server\nkind: server\nname: Server\n",
		filepath.Join("links", "connection.yaml"):     "api_version: locus/v0\ntype: link\nid: connection\nfrom: workstation\nto: server\nprovider: ssh\nprovides:\n  - shell\nprovider_data:\n  user: root\n  host: example.test\n  port: 22\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
