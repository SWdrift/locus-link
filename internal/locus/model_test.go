package locus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRegistryWalksParents(t *testing.T) {
	root := workspaceTestPath(t, "model-v1", "discover")
	os.RemoveAll(root)
	registry := filepath.Join(root, ".locus", "registry")
	writeModelFile(t, filepath.Join(registry, "scope.yaml"), manifestYAML("project.test", ""))
	nested := filepath.Join(root, "src", "nested", "package")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := DiscoverRegistry(nested)
	if err != nil {
		t.Fatal(err)
	}
	if found != registry {
		t.Fatalf("got %s, want %s", found, registry)
	}
}

func TestScopeV1RejectsLegacyAndUnknownManifestShapes(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{"v0", "api_version: locus/v0\nscope_id: project.test\n", "api_version must be locus/v1"},
		{"scope kind", "api_version: locus/v1\nscope_id: project.test\nscope:\n  kind: project\n", "field scope not found"},
		{"list imports", "api_version: locus/v1\nscope_id: project.test\nimports:\n  - alias: child\n    path: ../child\n", "cannot construct"},
		{"unknown source field", "api_version: locus/v1\nscope_id: project.test\nimports:\n  child:\n    source:\n      kind: directory\n      uri: ../child\n      typo: rejected\n", "field typo not found"},
		{"unknown variable", "api_version: locus/v1\nscope_id: project.test\nimports:\n  child: ${OTHER_HOME}/registry\n", "unsupported source variable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeModelRegistry(t, "manifest-"+test.name, test.manifest, nil)
			_, err := LoadScopeRegistry(root, false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}

func TestScopeV1NormalizesScalarSources(t *testing.T) {
	root := writeModelRegistry(t, "scalar-sources", "api_version: locus/v1\nscope_id: project.test\nimports:\n  local: ../local\n  home: ${LOCUS_HOME}/registry\n  git: git+https://example.test/repo.git\n  url: https://example.test/registry.zip\n", nil)
	registry, err := LoadScopeRegistry(root, false)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]Source{
		"local": {Kind: "directory", URI: "../local"},
		"home":  {Kind: "directory", URI: LocusHomeRegistryURI},
		"git":   {Kind: "git", URI: "https://example.test/repo.git"},
		"url":   {Kind: "url", URI: "https://example.test/registry.zip"},
	}
	for alias, want := range checks {
		if got := registry.Manifest.Imports[alias].Source; got != want {
			t.Errorf("%s: got %+v, want %+v", alias, got, want)
		}
	}
}

func TestObjectDecodingIsStrictAndV1Only(t *testing.T) {
	cases := map[string]string{
		"unknown":   "api_version: locus/v1\ntype: entity\nid: host\nkind: host\nname: Host\ntypo: rejected\n",
		"duplicate": "api_version: locus/v1\ntype: entity\nid: host\nid: duplicate\nkind: host\nname: Host\n",
		"multiple":  "api_version: locus/v1\ntype: entity\nid: host\nkind: host\nname: Host\n---\ntype: entity\nid: second\n",
		"v0":        "api_version: locus/v0\ntype: entity\nid: host\nkind: host\nname: Host\n",
	}
	for name, object := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeModelRegistry(t, "strict-"+name, manifestYAML("project.test", ""), map[string]string{"entities/value.yaml": object})
			if _, err := LoadScopeRegistry(root, false); err == nil {
				t.Fatal("expected strict decode failure")
			}
		})
	}
}

func TestBindingSharesLocalIdentityNamespace(t *testing.T) {
	root := writeModelRegistry(t, "binding-collision", manifestYAML("project.test", "bindings:\n  target: target\n"), map[string]string{
		"entities/target.yaml": entityYAML("target"),
	})
	_, err := LoadScopeRegistry(root, false)
	if err == nil || !strings.Contains(err.Error(), "duplicate local id target") {
		t.Fatalf("got %v", err)
	}
}

func TestRegistryContentDigestCoversDocsAndIgnoresLocation(t *testing.T) {
	files := map[string]string{
		"entities/host.yaml": entityYAML("host"),
		"docs/guide.md":      "first\n",
	}
	first := writeModelRegistry(t, "digest-first", manifestYAML("shared.test", ""), files)
	second := writeModelRegistry(t, "digest-second", manifestYAML("shared.test", ""), files)
	firstRegistry, err := LoadScopeRegistry(first, false)
	if err != nil {
		t.Fatal(err)
	}
	secondRegistry, err := LoadScopeRegistry(second, false)
	if err != nil {
		t.Fatal(err)
	}
	if firstRegistry.Digest != secondRegistry.Digest {
		t.Fatalf("digest depends on location: %s != %s", firstRegistry.Digest, secondRegistry.Digest)
	}
	writeModelFile(t, filepath.Join(second, "docs", "guide.md"), "second\n")
	changed, err := LoadScopeRegistry(second, false)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Digest == firstRegistry.Digest {
		t.Fatal("documentation change did not change content digest")
	}
}

func TestDocumentationMustStayWithinDocs(t *testing.T) {
	root := writeModelRegistry(t, "docs-containment", manifestYAML("project.test", ""), map[string]string{
		"entities/host.yaml": "api_version: locus/v1\ntype: entity\nid: host\nkind: host\nname: Host\ndocumentation:\n  - ref: ../../outside.md\n",
		"outside.md":         "outside\n",
	})
	_, err := LoadScopeRegistry(root, false)
	if err == nil || !strings.Contains(err.Error(), "must stay within") {
		t.Fatalf("got %v", err)
	}
}

func TestCollectorLoadsLongAliasPathAndCanonicalBinding(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "long-chain")
	os.RemoveAll(base)
	writeNamedRegistry(t, base, "shared", manifestYAML("scope.shared", ""), map[string]string{"entities/host.yaml": entityYAML("host")})
	writeNamedRegistry(t, base, "platform", manifestYAML("scope.platform", "imports:\n  shared: ../shared\n"), nil)
	writeNamedRegistry(t, base, "customer", manifestYAML("scope.customer", "imports:\n  platform: ../platform\n"), nil)
	root := writeNamedRegistry(t, base, "project", manifestYAML("scope.project", "imports:\n  customer: ../customer\nbindings:\n  production: customer::platform::shared::host\n"), nil)
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Completeness != Complete || len(registry.Scopes) != 4 {
		t.Fatalf("unexpected view: %s, scopes=%d", registry.Completeness, len(registry.Scopes))
	}
	resolved, err := registry.ResolveEntity("production")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "scope.shared::host" {
		t.Fatalf("got %s", resolved)
	}
}

func TestCollectorBlocksOnlyCycleBackEdge(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "cycle")
	os.RemoveAll(base)
	root := writeNamedRegistry(t, base, "a", manifestYAML("scope.a", "imports:\n  b: ../b\n"), map[string]string{"entities/a.yaml": entityYAML("a")})
	writeNamedRegistry(t, base, "b", manifestYAML("scope.b", "imports:\n  c: ../c\n"), map[string]string{"entities/b.yaml": entityYAML("b")})
	writeNamedRegistry(t, base, "c", manifestYAML("scope.c", "imports:\n  a: ../a\n"), map[string]string{"entities/c.yaml": entityYAML("c")})
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Completeness != Partial || len(registry.Scopes) != 3 || len(registry.BlockedImports) != 1 {
		t.Fatalf("unexpected cycle view: %+v", registry.BlockedImports)
	}
	blocked := registry.BlockedImports[0]
	if blocked.Reason != "cycle" || blocked.SourceScopeID != "scope.c" || strings.Join(blocked.CycleScopeIDs, ",") != "scope.c,scope.a,scope.b,scope.c" {
		t.Fatalf("unexpected cycle diagnostic: %+v", blocked)
	}
}

func TestCollectorDeduplicatesSameScopeDigestAcrossAliases(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "same-digest")
	os.RemoveAll(base)
	files := map[string]string{"entities/host.yaml": entityYAML("host")}
	writeNamedRegistry(t, base, "x", manifestYAML("scope.shared", ""), files)
	writeNamedRegistry(t, base, "y", manifestYAML("scope.shared", ""), files)
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  x: ../x\n  y: ../y\n"), nil)
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Scopes) != 2 || len(registry.Entities) != 1 {
		t.Fatalf("dedup failed: scopes=%d entities=%d", len(registry.Scopes), len(registry.Entities))
	}
	if len(registry.AliasPaths["scope.shared"]) != 2 {
		t.Fatalf("alias provenance lost: %+v", registry.AliasPaths["scope.shared"])
	}
}

func TestCollectorExcludesDifferentDigestWithoutAuthority(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "conflict-no-authority")
	os.RemoveAll(base)
	writeNamedRegistry(t, base, "x", manifestYAML("scope.shared", ""), map[string]string{"entities/x.yaml": entityYAML("x")})
	writeNamedRegistry(t, base, "y", manifestYAML("scope.shared", ""), map[string]string{"entities/y.yaml": entityYAML("y")})
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  x: ../x\n  y: ../y\n"), nil)
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Scopes) != 1 || len(registry.Entities) != 0 || registry.Completeness != Partial {
		t.Fatalf("conflicting scope leaked into view: scopes=%d entities=%d", len(registry.Scopes), len(registry.Entities))
	}
	if len(registry.BlockedImports) != 2 || registry.BlockedImports[0].Reason != "authority_conflict" || registry.BlockedImports[1].Reason != "authority_conflict" {
		t.Fatalf("unexpected diagnostics: %+v", registry.BlockedImports)
	}
}

func TestCollectorUsesActiveAuthorityForDifferentDigest(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "conflict-authority")
	os.RemoveAll(base)
	x := writeNamedRegistry(t, base, "x", manifestYAML("scope.shared", ""), map[string]string{"entities/x.yaml": entityYAML("x")})
	writeNamedRegistry(t, base, "y", manifestYAML("scope.shared", ""), map[string]string{"entities/y.yaml": entityYAML("y")})
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  x: ../x\n  y: ../y\n"), nil)
	xRegistry, err := LoadScopeRegistry(x, false)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.activateSource(context.Background(), SourceCacheEntry{
		OwnerScopeID: "scope.root", ImportAlias: "x", ActualScopeID: "scope.shared",
		ActiveContentDigest: xRegistry.Digest, ObjectPath: x,
	}, Source{Kind: "directory", URI: x}); err != nil {
		t.Fatal(err)
	}
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home"), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Entities["scope.shared::x"] == nil || registry.Entities["scope.shared::y"] != nil {
		t.Fatalf("active authority not used: %+v", registry.Entities)
	}
	if len(registry.BlockedImports) != 1 || registry.BlockedImports[0].Reason != "authority_conflict" {
		t.Fatalf("unexpected conflict diagnostics: %+v", registry.BlockedImports)
	}
}

func TestCollectorKeepsSiblingWhenSourceUnavailable(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "sibling-partial")
	os.RemoveAll(base)
	writeNamedRegistry(t, base, "good", manifestYAML("scope.good", ""), map[string]string{"entities/host.yaml": entityYAML("host")})
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  good: ../good\n  missing: ../missing\n"), nil)
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Entities["scope.good::host"] == nil || len(registry.BlockedImports) != 1 || registry.BlockedImports[0].Reason != "source_unavailable" {
		t.Fatalf("unexpected partial sibling result: %+v", registry.BlockedImports)
	}
}

func TestRemoteSourceDoesNotFetchWithoutActiveCache(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "remote-read")
	os.RemoveAll(base)
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  remote: https://example.test/registry.zip\n"), nil)
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home"), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if registry.Completeness != Partial || len(registry.BlockedImports) != 1 || registry.BlockedImports[0].Reason != "missing_active_cache" {
		t.Fatalf("unexpected remote read: %+v", registry.BlockedImports)
	}
}

func TestBlockedImportSanitizesURLMetadata(t *testing.T) {
	base := workspaceTestPath(t, "model-v1", "blocked-sanitized")
	os.RemoveAll(base)
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  remote: https://example.test/registry.zip?token=secret#fragment\n"), nil)
	registry, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home")})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.BlockedImports[0].Source.URI; got != "https://example.test/registry.zip" {
		t.Fatalf("blocked diagnostic leaked URL metadata: %q", got)
	}
}

func manifestYAML(scopeID, body string) string {
	return "api_version: locus/v1\nscope_id: " + scopeID + "\n" + body
}

func entityYAML(id string) string {
	return "api_version: locus/v1\ntype: entity\nid: " + id + "\nkind: host\nname: " + id + "\n"
}

func writeModelRegistry(t *testing.T, name, manifest string, files map[string]string) string {
	t.Helper()
	root := workspaceTestPath(t, "model-v1", name)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	return writeNamedRegistry(t, filepath.Dir(root), filepath.Base(root), manifest, files)
}

func writeNamedRegistry(t *testing.T, base, name, manifest string, files map[string]string) string {
	t.Helper()
	root := filepath.Join(base, name)
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
