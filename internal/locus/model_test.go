package locus

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDeclarationNameLexicalBoundaries(t *testing.T) {
	for _, value := range []string{"a", "z9._-", "project.customer-a_1"} {
		if !validDeclarationName(value) {
			t.Errorf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"", "UPPER", "has space", "customer::host", "path/part", "客户"} {
		if validDeclarationName(value) {
			t.Errorf("expected %q to be invalid", value)
		}
	}
}

func TestLoadRegistryRejectsInvalidDeclarationNamesAndObjectVersion(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		objects  map[string]string
		want     string
	}{
		{
			name:     "scope id",
			manifest: "api_version: locus/v0\nscope:\n  id: Project.Test\n  kind: project\n",
			want:     "invalid scope",
		},
		{
			name: "import alias",
			manifest: "api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\nimports:\n" +
				"  - alias: Customer\n    path: missing\n",
			want: "invalid import alias \"Customer\"",
		},
		{
			name: "binding role",
			manifest: "api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\nbindings:\n" +
				"  Production Host: host.test\n",
			want: "invalid binding role \"Production Host\"",
		},
		{
			name:     "local id",
			manifest: modelProjectManifest,
			objects: map[string]string{
				"entities/invalid.yaml": "api_version: locus/v0\ntype: entity\nid: Host.Test\nkind: host\nname: Host\n",
			},
			want: "invalid local id \"Host.Test\"",
		},
		{
			name:     "object api version",
			manifest: modelProjectManifest,
			objects: map[string]string{
				"entities/invalid.yaml": "api_version: locus/v1\ntype: entity\nid: host.test\nkind: host\nname: Host\n",
			},
			want: "api_version must be locus/v0",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeModelRegistry(t, "invalid-declaration", test.name, test.manifest, test.objects)
			_, err := LoadRegistry(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadRegistryRejectsEnvironmentImportsAndBindings(t *testing.T) {
	cases := []struct {
		name     string
		imported bool
		field    string
		want     string
	}{
		{name: "active imports", field: "imports:\n  - alias: nested\n    path: missing\n", want: "environment imports are not supported in v0"},
		{name: "active bindings", field: "bindings:\n  production: host.test\n", want: "environment bindings are not supported in v0"},
		{name: "imported imports", imported: true, field: "imports:\n  - alias: nested\n    path: missing\n", want: "import customer: environment imports are not supported in v0"},
		{name: "imported bindings", imported: true, field: "bindings:\n  production: host.test\n", want: "import customer: environment bindings are not supported in v0"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := workspaceTestPath(t, "environment-restrictions", test.name)
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			active := root
			if test.imported {
				active = filepath.Join(root, "project")
				imported := filepath.Join(root, "environment")
				writeModelFile(t, filepath.Join(active, "scope.yaml"),
					"api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\nimports:\n  - alias: customer\n    path: ../environment\n")
				writeModelFile(t, filepath.Join(imported, "scope.yaml"),
					"api_version: locus/v0\nscope:\n  id: environment.test\n  kind: environment\n"+test.field)
			} else {
				writeModelFile(t, filepath.Join(active, "scope.yaml"),
					"api_version: locus/v0\nscope:\n  id: environment.test\n  kind: environment\n"+test.field)
			}
			_, err := LoadRegistry(active)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadRegistryValidatesDocumentationReferences(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want string
	}{
		{name: "empty", ref: "", want: "documentation[0].ref is invalid"},
		{name: "fragment only", ref: "#details", want: "documentation[0].ref is invalid"},
		{name: "missing", ref: "../docs/missing.md", want: "does not reference a file"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			object := "api_version: locus/v0\ntype: entity\nid: host.test\nkind: host\nname: Host\n" +
				"documentation:\n  - ref: \"" + test.ref + "\"\n"
			root := writeModelRegistry(t, "documentation", test.name, modelProjectManifest,
				map[string]string{"entities/host.yaml": object})
			_, err := LoadRegistry(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadRegistryConfinesDocumentationReferencesToScopeDocs(t *testing.T) {
	for _, test := range []struct {
		name string
		ref  func(string) string
		want string
	}{
		{
			name: "registry file traversal",
			ref:  func(string) string { return "../scope.yaml" },
			want: "must stay within the scope docs directory",
		},
		{
			name: "absolute path",
			ref:  func(root string) string { return filepath.ToSlash(filepath.Join(root, "docs", "guide.md")) },
			want: "must be relative",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writeModelRegistry(t, "documentation-boundary", test.name, modelProjectManifest,
				map[string]string{"docs/guide.md": "Guide\n"})
			object := "api_version: locus/v0\ntype: entity\nid: host.test\nkind: host\nname: Host\n" +
				"documentation:\n  - ref: \"" + test.ref(root) + "\"\n"
			writeModelFile(t, filepath.Join(root, "entities", "host.yaml"), object)
			_, err := LoadRegistry(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadRegistryAcceptsValidDeclarationBoundariesAndDocumentation(t *testing.T) {
	root := writeModelRegistry(t, "valid-declaration", "boundaries",
		"api_version: locus/v0\nscope:\n  id: z9._-\n  kind: project\n",
		map[string]string{
			"entities/z9._-.yaml": "api_version: locus/v0\ntype: entity\nid: z9._-\nkind: host\nname: Host\n" +
				"documentation:\n  - ref: ../docs/guide.md#details\n",
			"docs/guide.md": "Guide\n",
		})
	registry, err := LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Entities["z9._-::z9._-"]; !ok {
		t.Fatal("valid boundary ID was not canonicalized")
	}
}

func TestRegistryValidateChecksProviderRegistrationAndDeclaration(t *testing.T) {
	root := writeModelRegistry(t, "provider-validation", "unknown", modelProjectManifest,
		map[string]string{
			"entities/from.yaml": "api_version: locus/v0\ntype: entity\nid: host.from\nkind: host\nname: From\n",
			"entities/to.yaml":   "api_version: locus/v0\ntype: entity\nid: host.to\nkind: host\nname: To\n",
			"links/link.yaml": "api_version: locus/v0\ntype: link\nid: link.test\nfrom: host.from\nto: host.to\n" +
				"provider: unknown\n",
		})
	_, err := LoadRegistry(root)
	if err == nil || !strings.Contains(err.Error(), "project.test::link.test: unsupported provider unknown") {
		t.Fatalf("expected unsupported provider error with canonical link context, got %v", err)
	}

	root = writeModelRegistry(t, "provider-validation", "invalid-data", modelProjectManifest,
		map[string]string{
			"entities/from.yaml": "api_version: locus/v0\ntype: entity\nid: host.from\nkind: host\nname: From\n",
			"entities/to.yaml":   "api_version: locus/v0\ntype: entity\nid: host.to\nkind: host\nname: To\n",
			"links/link.yaml": "api_version: locus/v0\ntype: link\nid: link.test\nfrom: host.from\nto: host.to\n" +
				"provider: ssh\nprovider_data:\n  user: deploy\n",
		})
	_, err = LoadRegistry(root)
	if err == nil || !strings.Contains(err.Error(), "project.test::link.test: provider_data.host") {
		t.Fatalf("expected provider validation error with canonical link context, got %v", err)
	}
}

func TestRegistryValidateRejectsVersionForEveryObjectType(t *testing.T) {
	registry := &Registry{
		Scopes: map[string]Manifest{
			"project.test": {APIVersion: "locus/v0", Scope: Scope{ID: "project.test", Kind: "project"}},
		},
		Entities: map[string]*Entity{
			"project.test::host.from": {
				APIVersion: "locus/v1", ID: "host.from", CanonicalID: "project.test::host.from", ScopeID: "project.test",
			},
			"project.test::host.to": {
				APIVersion: "locus/v0", ID: "host.to", CanonicalID: "project.test::host.to", ScopeID: "project.test",
			},
		},
		Links: map[string]*Link{
			"project.test::link.test": {
				APIVersion: "locus/v1", ID: "link.test", CanonicalID: "project.test::link.test", ScopeID: "project.test",
				From: "host.from", To: "host.to", Provider: "ssh",
				ProviderData: map[string]any{"user": "deploy", "host": "127.0.0.1", "port": 22},
			},
		},
		Routes: map[string]*Route{
			"project.test::route.test": {
				APIVersion: "locus/v1", ID: "route.test", CanonicalID: "project.test::route.test", ScopeID: "project.test",
				Steps: []RouteStep{{Link: "link.test"}},
			},
		},
	}
	issues := strings.Join(registry.Validate(), "\n")
	for _, objectID := range []string{"host.from", "link.test", "route.test"} {
		want := "project.test::" + objectID + ": api_version must be locus/v0"
		if !strings.Contains(issues, want) {
			t.Errorf("expected %q in issues:\n%s", want, issues)
		}
	}
}

func TestRegistryValidateSortsDeclarationIssues(t *testing.T) {
	registry := &Registry{
		Scopes: map[string]Manifest{
			"project.test": {APIVersion: "locus/v0", Scope: Scope{ID: "project.test", Kind: "project"}},
		},
		Entities: map[string]*Entity{
			"project.test::z": {CanonicalID: "project.test::z", ScopeID: "project.test", APIVersion: "locus/v1", ID: "z"},
			"project.test::a": {CanonicalID: "project.test::a", ScopeID: "project.test", APIVersion: "locus/v1", ID: "a"},
		},
		Links:  map[string]*Link{},
		Routes: map[string]*Route{},
	}
	issues := registry.Validate()
	want := "project.test::a: api_version must be locus/v0\nproject.test::z: api_version must be locus/v0"
	if strings.Join(issues, "\n") != want {
		t.Fatalf("unexpected issue order:\n%s", strings.Join(issues, "\n"))
	}
}

const modelProjectManifest = "api_version: locus/v0\nscope:\n  id: project.test\n  kind: project\n"

func writeModelRegistry(t *testing.T, elements1, elements2, manifest string, files map[string]string) string {
	t.Helper()
	root := workspaceTestPath(t, elements1, elements2)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	writeModelFile(t, filepath.Join(root, "scope.yaml"), manifest)
	for relative, content := range files {
		writeModelFile(t, filepath.Join(root, filepath.FromSlash(relative)), content)
	}
	return root
}

func writeModelFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
