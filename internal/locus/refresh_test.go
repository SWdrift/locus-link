package locus

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestStoreCreatesCacheAndAuthorityTables(t *testing.T) {
	base := workspaceTestPath(t, "refresh", "schema")
	os.RemoveAll(base)
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, table := range []string{"project_registrations", "source_cache_entries", "scope_authorities"} {
		var name string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
}

func TestURLRefreshCachesWithoutImplicitFetchAndRetainsOnFailure(t *testing.T) {
	base := workspaceTestPath(t, "refresh", "url")
	os.RemoveAll(base)
	home := filepath.Join(base, "home")
	t.Setenv("LOCUS_HOME", home)
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var mu sync.Mutex
	payload := registryZIP(t, "scope.remote", "first")
	etag := `"first"`
	fail := false
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		if fail {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if request.Header.Get("If-None-Match") == etag {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		writer.Header().Set("ETag", etag)
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	root := writeURLImportRoot(t, base, server.URL+"/registry.zip")

	view, err := CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if view.Completeness != Partial || len(view.BlockedImports) != 1 || view.BlockedImports[0].Reason != "missing_active_cache" || requests != 0 {
		t.Fatalf("ordinary collection fetched or returned wrong state: requests=%d view=%+v", requests, view)
	}
	first, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "success" || len(first.Activated) != 1 || first.Completeness != Complete || requests != 1 {
		t.Fatalf("unexpected first refresh: requests=%d result=%+v", requests, first)
	}
	firstDigest := first.Activated[0].ContentDigest
	entry, err := store.SourceCacheEntry("scope.root", "remote")
	if err != nil || entry == nil {
		t.Fatalf("missing cache entry: %+v %v", entry, err)
	}
	if entry.ActiveContentDigest != firstDigest || entry.ObjectPath != filepath.Join(home, "cache", "objects", strings.TrimPrefix(firstDigest, "sha256:")) {
		t.Fatalf("unexpected active cache: %+v", entry)
	}
	if _, err := os.Stat(entry.ObjectPath); err != nil {
		t.Fatalf("immutable object missing: %v", err)
	}
	view, err = CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if view.Entities["scope.remote::first"] == nil || requests != 1 {
		t.Fatalf("ordinary collection did not use cache: requests=%d entities=%+v", requests, view.Entities)
	}
	mismatchedManifest := strings.Replace(urlImportManifest(server.URL+"/registry.zip"), "scope.remote", "scope.other", 1)
	writeModelFile(t, filepath.Join(root, "scope.yaml"), mismatchedManifest)
	mismatched, err := CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if mismatched.Completeness != Partial || len(mismatched.BlockedImports) != 1 || mismatched.BlockedImports[0].Reason != "scope_id_mismatch" || requests != 1 {
		t.Fatalf("changed expected scope_id accepted stale cache: requests=%d view=%+v", requests, mismatched)
	}
	writeModelFile(t, filepath.Join(root, "scope.yaml"), urlImportManifest(server.URL+"/registry.zip"))
	unchanged, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "success" || len(unchanged.Retained) != 1 || len(unchanged.Activated) != 0 || requests != 2 {
		t.Fatalf("304 did not retain active cache: requests=%d result=%+v", requests, unchanged)
	}

	mu.Lock()
	payload = registryZIP(t, "scope.remote", "second")
	etag = `"second"`
	mu.Unlock()
	writeModelFile(t, filepath.Join(root, "scope.yaml"), urlImportManifest(server.URL+"/updated.zip"))
	view, err = CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if view.Entities["scope.remote::first"] == nil || view.Entities["scope.remote::second"] != nil || requests != 2 {
		t.Fatalf("source change fetched before refresh: requests=%d entities=%+v", requests, view.Entities)
	}
	_, rootContext, err := LoadRegistryContext(root, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(rootContext.SourceCache) != 1 || !rootContext.SourceCache[0].ConfigurationChanged ||
		rootContext.SourceCache[0].CurrentSourceDigest == rootContext.SourceCache[0].ConfiguredSourceDigest {
		t.Fatalf("context did not report pending Source configuration: %+v", rootContext.SourceCache)
	}
	second, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "success" || len(second.Activated) != 1 || second.Activated[0].ContentDigest == firstDigest || requests != 3 {
		t.Fatalf("updated source was not activated: requests=%d result=%+v", requests, second)
	}
	secondDigest := second.Activated[0].ContentDigest

	mu.Lock()
	fail = true
	mu.Unlock()
	failed, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "partial" || len(failed.RefreshErrors) != 1 || len(failed.Retained) != 1 || failed.Retained[0].ContentDigest != secondDigest {
		t.Fatalf("failed refresh did not retain active object: %+v", failed)
	}
	entry, err = store.SourceCacheEntry("scope.root", "remote")
	if err != nil || entry.ActiveContentDigest != secondDigest || entry.LastRefreshStatus != "failure" {
		t.Fatalf("failed refresh changed active pointer: %+v %v", entry, err)
	}
	if _, err := RefreshRegistry(context.Background(), root, "unknown", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()}); err == nil {
		t.Fatal("unknown refresh target was accepted")
	}
}

func TestGitRefreshPinsCommitAndRetainsOnInvalidRevision(t *testing.T) {
	base := workspaceTestPath(t, "refresh", "git")
	os.RemoveAll(base)
	home := filepath.Join(base, "home")
	helper := buildSourceHelper(t, base)
	logPath := filepath.Join(base, "helper.log")
	t.Setenv("LOCUS_SOURCE_HELPER_LOG", logPath)
	remote := writeNamedRegistry(t, base, "remote", manifestYAML("scope.remote", ""), map[string]string{"entities/first.yaml": entityYAML("first")})
	firstCommit := strings.Repeat("1", 40)
	writeModelFile(t, remote+".commit", firstCommit+"\n")
	root := writeNamedRegistry(t, base, "root", fmt.Sprintf("api_version: locus/v1\nscope_id: scope.root\nimports:\n  remote:\n    scope_id: scope.remote\n    source:\n      kind: git\n      uri: %s\n      revision: main\n", fileURL(remote)), nil)
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	view, err := CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil || view.Completeness != Partial || helperCalls(t, logPath) != 0 {
		t.Fatalf("ordinary Git collection invoked helper: calls=%d err=%v", helperCalls(t, logPath), err)
	}
	first, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, GitExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "success" || len(first.Activated) != 1 || first.Activated[0].ResolvedRevision != firstCommit || helperCalls(t, logPath) != 3 {
		t.Fatalf("unexpected Git refresh: calls=%d result=%+v", helperCalls(t, logPath), first)
	}
	firstDigest := first.Activated[0].ContentDigest
	writeModelFile(t, filepath.Join(remote, "entities", "second.yaml"), entityYAML("second"))
	writeModelFile(t, remote+".commit", strings.Repeat("2", 40)+"\n")
	view, err = CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil || view.Entities["scope.remote::second"] != nil || helperCalls(t, logPath) != 3 {
		t.Fatalf("ordinary Git collection did not retain old revision: calls=%d err=%v", helperCalls(t, logPath), err)
	}
	second, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, GitExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "success" || second.Activated[0].ContentDigest == firstDigest || second.Activated[0].ResolvedRevision != strings.Repeat("2", 40) {
		t.Fatalf("Git update was not activated: %+v", second)
	}
	secondDigest := second.Activated[0].ContentDigest
	writeModelFile(t, remote+".commit", "not-a-commit\n")
	failed, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, GitExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "partial" || len(failed.Retained) != 1 || failed.Retained[0].ContentDigest != secondDigest {
		t.Fatalf("invalid revision did not retain active Git object: %+v", failed)
	}
}

func TestURLRefreshRejectsInvalidCandidatesWithoutActivation(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		reason  string
	}{
		{name: "invalid registry", payload: zipPayload(t, map[string]string{
			"scope.yaml": "api_version: locus/v1\nscope_id: scope.remote\nunknown: true\n",
		}), reason: "invalid_registry"},
		{name: "scope mismatch", payload: registryZIP(t, "scope.other", "entity"), reason: "scope_id_mismatch"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			base := workspaceTestPath(t, "refresh", "invalid-"+strings.ReplaceAll(testCase.name, " ", "-"))
			os.RemoveAll(base)
			home := filepath.Join(base, "home")
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write(testCase.payload)
			}))
			defer server.Close()
			root := writeURLImportRoot(t, base, server.URL+"/registry.zip")
			store, err := OpenStore(filepath.Join(base, "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			result, err := RefreshRegistry(context.Background(), root, "remote", RefreshOptions{Home: home, Store: store, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "failure" || len(result.RefreshErrors) != 1 || result.RefreshErrors[0].Reason != testCase.reason ||
				len(result.Activated) != 0 || len(result.Retained) != 0 {
				t.Fatalf("invalid candidate was not rejected: %+v", result)
			}
			entry, err := store.SourceCacheEntry("scope.root", "remote")
			if err != nil || entry == nil || entry.ActiveContentDigest != "" || entry.LastRefreshStatus != "failure" {
				t.Fatalf("invalid candidate changed active pointer: %+v %v", entry, err)
			}
		})
	}
}

func TestExtractRegistryZIPRejectsUnsafeEntries(t *testing.T) {
	base := workspaceTestPath(t, "refresh", "unsafe-zip")
	os.RemoveAll(base)
	if err := extractRegistryZIP(zipPayload(t, map[string]string{"../escape": "bad"}), filepath.Join(base, "traversal")); err == nil {
		t.Fatal("ZIP traversal entry was accepted")
	}
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	header := &zip.FileHeader{Name: "docs/link"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractRegistryZIP(payload.Bytes(), filepath.Join(base, "symlink")); err == nil {
		t.Fatal("ZIP symlink entry was accepted")
	}
}

func TestCollectorUsesImmutableAuthorityWhenCurrentCandidatesDiffer(t *testing.T) {
	base := workspaceTestPath(t, "refresh", "authority-object")
	os.RemoveAll(base)
	writeNamedRegistry(t, base, "x", manifestYAML("scope.shared", ""), map[string]string{"entities/x.yaml": entityYAML("x")})
	authorityPath := writeNamedRegistry(t, base, "authority", manifestYAML("scope.shared", ""), map[string]string{"entities/active.yaml": entityYAML("active")})
	root := writeNamedRegistry(t, base, "root", manifestYAML("scope.root", "imports:\n  x: ../x\n"), nil)
	authority, err := LoadScopeRegistry(authorityPath, true)
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(base, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.activateSource(context.Background(), SourceCacheEntry{OwnerScopeID: "removed.owner", ImportAlias: "removed", ActualScopeID: "scope.shared", ActiveContentDigest: authority.Digest, ObjectPath: authorityPath}, Source{Kind: "directory", URI: authorityPath}); err != nil {
		t.Fatal(err)
	}
	view, err := CollectRegistry(root, CollectorOptions{Home: filepath.Join(base, "home"), Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if view.Entities["scope.shared::active"] == nil || view.Entities["scope.shared::x"] != nil || view.Entities["scope.shared::y"] != nil || view.Completeness != Partial {
		t.Fatalf("immutable authority was not used: entities=%+v diagnostics=%+v", view.Entities, view.BlockedImports)
	}
}

func registryZIP(t *testing.T, scopeID, entityID string) []byte {
	t.Helper()
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	files := map[string]string{"scope.yaml": manifestYAML(scopeID, ""), "entities/" + entityID + ".yaml": entityYAML(entityID)}
	for _, name := range []string{"scope.yaml", "entities/" + entityID + ".yaml"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func zipPayload(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func writeURLImportRoot(t *testing.T, base, sourceURL string) string {
	t.Helper()
	return writeNamedRegistry(t, base, "root", urlImportManifest(sourceURL), nil)
}

func urlImportManifest(sourceURL string) string {
	return fmt.Sprintf("api_version: locus/v1\nscope_id: scope.root\nimports:\n  remote:\n    scope_id: scope.remote\n    source:\n      kind: url\n      uri: %s\n", sourceURL)
}

func buildSourceHelper(t *testing.T, base string) string {
	t.Helper()
	executable := filepath.Join(base, "bin", "source-helper")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", executable, "../../test/source-helper")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build source helper: %v\n%s", err, output)
	}
	return executable
}

func helperCalls(t *testing.T, path string) int {
	t.Helper()
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.FieldsFunc(string(contents), func(r rune) bool { return r == '\n' }))
}

func fileURL(path string) string {
	value := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return (&url.URL{Scheme: "file", Path: value}).String()
}
