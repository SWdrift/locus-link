package test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestScopeGraphEndToEnd(t *testing.T) {
	repository := repositoryRoot(t)
	root := filepath.Join(repository, "temp", "e2e-run", "scope")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(repository, "test", "e2e", "case", "scope-graph")
	bin := filepath.Join(root, "bin")
	mustMkdir(t, bin)
	locusExe := filepath.Join(bin, executableName("locus"))
	probeHelper := filepath.Join(bin, executableName("probe-helper"))
	sourceHelper := filepath.Join(bin, executableName("source-helper"))
	goBuild(t, repository, locusExe, "./cmd/locus")
	goBuild(t, repository, probeHelper, "./test/helper")
	goBuild(t, repository, sourceHelper, "./test/source-helper")
	copyFile(t, probeHelper, filepath.Join(bin, executableName("ssh")))

	listener, port := startEndpoint(t)
	defer listener.Close()
	localRoot := filepath.Join(root, "local")
	materializeFixture(t, filepath.Join(caseRoot, "local"), localRoot, map[string]string{"{{PORT}}": stringInt(port)})
	deviceRoot := filepath.Join(root, "sim", "device")
	mustMkdir(t, deviceRoot)
	mustWrite(t, filepath.Join(deviceRoot, "ssh-up"), "up\n")
	localState := filepath.Join(root, "state", "local.db")
	localEnv := append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LOCUS_HOME="+filepath.Join(localRoot, "user"),
		"LOCUS_STATE_PATH="+localState,
		"LOCUS_SIM_ROOT="+deviceRoot,
		"LOCUS_SIM_LOG="+filepath.Join(root, "sim", "probe.log"),
	)
	project := filepath.Join(localRoot, "project")
	projectRegistry := filepath.Join(project, ".locus", "registry")
	sharedManifest := filepath.Join(localRoot, "shared", "registry", "scope.yaml")
	cycleManifest, err := os.ReadFile(sharedManifest)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, sharedManifest, "api_version: locus/v1\nscope_id: shared.scope-e2e\n")
	runCLI(t, locusExe, project, localEnv, "project", "register", "--registry", projectRegistry, "--json")
	mustWrite(t, sharedManifest, string(cycleManifest))

	nested := filepath.Join(project, "nested", "work")
	mustMkdir(t, nested)
	projectContext := runCLI(t, locusExe, nested, localEnv, "context", "--from", "workstation", "--vantage", "scope:e2e", "--json")
	assertStringAt(t, projectContext, "active_scope", "project.scope-e2e")
	rootContext := mustObjectAt(t, projectContext, "root")
	assertStringAt(t, rootContext, "root_origin", "project")
	assertBoolAt(t, rootContext, "registered", true)
	assertBoolAt(t, rootContext, "has_user_import", true)
	assertPartialGraph(t, runCLI(t, locusExe, nested, localEnv, "graph", "--json"))

	resolved := runCLIExpectExitJSON(t, locusExe, project, localEnv, 3,
		"resolve", "shared-host", "--capability", "shell", "--from", "workstation", "--vantage", "scope:e2e", "--json")
	assertStringAt(t, resolved, "status", "incomplete")
	if candidates, ok := resolved["candidates"].([]any); !ok || len(candidates) != 1 || resolved["route"] != nil {
		t.Fatalf("partial Resolve claimed an invalid unique result: %#v", resolved)
	}
	probe := runCLI(t, locusExe, project, localEnv,
		"probe", "shared-shell", "--from", "workstation", "--vantage", "scope:e2e", "--timeout", "2s", "--json")
	assertStringAt(t, probe, "status", "success")
	assertStringAt(t, probe, "completeness", "partial")
	assertObservationCount(t, localState, 1)

	outside := filepath.Join(localRoot, "outside")
	mustMkdir(t, outside)
	userContext := runCLI(t, locusExe, outside, localEnv, "context", "--from", "user-service", "--vantage", "scope:user", "--json")
	assertStringAt(t, userContext, "active_scope", "user.scope-e2e")
	assertStringAt(t, mustObjectAt(t, userContext, "root"), "root_origin", "user")
	userGraph := runCLI(t, locusExe, outside, localEnv, "graph", "--json")
	assertStringAt(t, userGraph, "completeness", "complete")
	assertGraphEntities(t, userGraph, "user.scope-e2e::user-service")
	if graphHasEntity(userGraph, "project.scope-e2e::workstation") {
		t.Fatalf("project registration merged declarations into user graph: %#v", userGraph)
	}
	projects := runCLI(t, locusExe, outside, localEnv, "project", "list", "--json")
	if values, ok := projects["projects"].([]any); !ok || len(values) != 1 {
		t.Fatalf("registered project missing: %#v", projects)
	}

	testRemoteScopeGraph(t, repository, root, caseRoot, locusExe, sourceHelper)
}

func testRemoteScopeGraph(t *testing.T, repository, root, caseRoot, locusExe, sourceHelper string) {
	t.Helper()
	remoteRoot := filepath.Join(root, "remote")
	materializeFixture(t, filepath.Join(caseRoot, "remote"), remoteRoot, nil)
	urlSource := filepath.Join(remoteRoot, "url-source")
	var payload atomic.Value
	payload.Store(zipRegistryDirectory(t, urlSource))
	var requests atomic.Int64
	var failActive atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path == "/failure.zip" || failActive.Load() {
			http.Error(response, "unavailable", http.StatusServiceUnavailable)
			return
		}
		response.Header().Set("Content-Type", "application/zip")
		_, _ = response.Write(payload.Load().([]byte))
	}))
	defer server.Close()
	gitSource := filepath.Join(remoteRoot, "git-source")
	remoteRegistry := filepath.Join(remoteRoot, "root")
	failureRegistry := filepath.Join(remoteRoot, "failure-root")
	replaceFileTokens(t, filepath.Join(remoteRegistry, "scope.yaml"), map[string]string{
		"{{GIT_URI}}": fileURLForE2E(gitSource), "{{URL_URI}}": server.URL + "/registry.zip",
	})
	replaceFileTokens(t, filepath.Join(failureRegistry, "scope.yaml"), map[string]string{"{{FAIL_URL}}": server.URL + "/failure.zip"})
	helperLog := filepath.Join(root, "sim", "source-helper.log")
	remoteEnv := append(os.Environ(),
		"LOCUS_HOME="+filepath.Join(root, "remote-home"),
		"LOCUS_STATE_PATH="+filepath.Join(root, "state", "remote.db"),
		"LOCUS_GIT_EXECUTABLE="+sourceHelper,
		"LOCUS_SOURCE_HELPER_LOG="+helperLog,
	)

	firstGraph := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", remoteRegistry, "--json")
	assertStringAt(t, firstGraph, "completeness", "partial")
	assertBlockedCountAndReason(t, firstGraph, 2, "missing_active_cache")
	if requests.Load() != 0 || fileLineCount(t, helperLog) != 0 {
		t.Fatalf("ordinary graph fetched remote Sources: HTTP=%d helper=%d", requests.Load(), fileLineCount(t, helperLog))
	}
	refreshed := runCLI(t, locusExe, repository, remoteEnv, "refresh", "--registry", remoteRegistry, "--json")
	assertStringAt(t, refreshed, "status", "success")
	if activated, ok := refreshed["activated"].([]any); !ok || len(activated) != 2 {
		t.Fatalf("remote closure was not activated: %#v", refreshed)
	}
	if requests.Load() != 1 || fileLineCount(t, helperLog) != 3 {
		t.Fatalf("unexpected refresh fetch counts: HTTP=%d helper=%d", requests.Load(), fileLineCount(t, helperLog))
	}
	activeGraph := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", remoteRegistry, "--json")
	assertStringAt(t, activeGraph, "completeness", "complete")
	assertGraphEntities(t, activeGraph, "remote.git::git-v1", "remote.url::url-v1")
	gitScope := findGraphScope(t, activeGraph, "remote.git")
	assertStringAt(t, gitScope, "resolved_revision", strings.Repeat("1", 40))
	if requests.Load() != 1 || fileLineCount(t, helperLog) != 3 {
		t.Fatal("normal graph re-fetched active remote Sources")
	}

	mustWrite(t, filepath.Join(gitSource, "entities", "git-v2.yaml"), "api_version: locus/v1\ntype: entity\nid: git-v2\nkind: service\nname: Git V2\n")
	mustWrite(t, gitSource+".commit", strings.Repeat("2", 40)+"\n")
	mustWrite(t, filepath.Join(urlSource, "entities", "url-v2.yaml"), "api_version: locus/v1\ntype: entity\nid: url-v2\nkind: service\nname: URL V2\n")
	payload.Store(zipRegistryDirectory(t, urlSource))
	beforeRefresh := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", remoteRegistry, "--json")
	if graphHasEntity(beforeRefresh, "remote.git::git-v2") || graphHasEntity(beforeRefresh, "remote.url::url-v2") || requests.Load() != 1 || fileLineCount(t, helperLog) != 3 {
		t.Fatalf("updated Sources became visible without refresh: %#v", beforeRefresh)
	}
	secondRefresh := runCLI(t, locusExe, repository, remoteEnv, "refresh", "--registry", remoteRegistry, "--json")
	assertStringAt(t, secondRefresh, "status", "success")
	updatedGraph := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", remoteRegistry, "--json")
	assertGraphEntities(t, updatedGraph, "remote.git::git-v2", "remote.url::url-v2")

	mustWrite(t, gitSource+".commit", "invalid\n")
	failActive.Store(true)
	failedRefresh := runCLIExpectExitJSON(t, locusExe, repository, remoteEnv, 5, "refresh", "--registry", remoteRegistry, "--json")
	assertStringAt(t, failedRefresh, "status", "partial")
	if retained, ok := failedRefresh["retained"].([]any); !ok || len(retained) != 2 {
		t.Fatalf("failed refresh did not retain both active objects: %#v", failedRefresh)
	}
	retainedGraph := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", remoteRegistry, "--json")
	assertStringAt(t, retainedGraph, "completeness", "complete")
	assertGraphEntities(t, retainedGraph, "remote.git::git-v2", "remote.url::url-v2")
	firstFailure := runCLIExpectExitJSON(t, locusExe, repository, remoteEnv, 5, "refresh", "unavailable", "--registry", failureRegistry, "--json")
	assertStringAt(t, firstFailure, "status", "failure")
	failureGraph := runCLI(t, locusExe, repository, remoteEnv, "graph", "--registry", failureRegistry, "--json")
	assertBlockedCountAndReason(t, failureGraph, 1, "missing_active_cache")
}

func assertPartialGraph(t *testing.T, graph map[string]any) {
	t.Helper()
	assertStringAt(t, graph, "completeness", "partial")
	assertBlockedCountAndReason(t, graph, 1, "cycle")
	if scopes, ok := graph["scopes"].([]any); !ok || len(scopes) != 5 {
		t.Fatalf("unexpected Scope graph nodes: %#v", graph["scopes"])
	}
	assertGraphEntities(t, graph,
		"project.scope-e2e::workstation", "user.scope-e2e::user-service", "customer.scope-e2e::customer",
		"platform.scope-e2e::platform", "shared.scope-e2e::host.shared",
	)
	shared := findGraphScope(t, graph, "shared.scope-e2e")
	paths, ok := shared["alias_paths"].([]any)
	if !ok || len(paths) != 2 {
		t.Fatalf("same Scope digest was not deduplicated with both alias paths: %#v", shared)
	}
}

func assertBlockedCountAndReason(t *testing.T, result map[string]any, want int, reason string) {
	t.Helper()
	blocked, ok := result["blocked_imports"].([]any)
	if !ok || len(blocked) != want {
		t.Fatalf("blocked imports = %#v, want %d", result["blocked_imports"], want)
	}
	for _, value := range blocked {
		diagnostic, ok := value.(map[string]any)
		if !ok || diagnostic["reason"] != reason {
			t.Fatalf("unexpected blocked import: %#v", value)
		}
	}
}

func assertGraphEntities(t *testing.T, graph map[string]any, expected ...string) {
	t.Helper()
	for _, canonicalID := range expected {
		if !graphHasEntity(graph, canonicalID) {
			t.Fatalf("graph entity %s missing: %#v", canonicalID, graph["entities"])
		}
	}
}

func graphHasEntity(graph map[string]any, canonicalID string) bool {
	entities, _ := graph["entities"].([]any)
	for _, value := range entities {
		entity, _ := value.(map[string]any)
		if entity["canonical_id"] == canonicalID {
			return true
		}
	}
	return false
}

func findGraphScope(t *testing.T, graph map[string]any, scopeID string) map[string]any {
	t.Helper()
	scopes, _ := graph["scopes"].([]any)
	for _, value := range scopes {
		scope, _ := value.(map[string]any)
		if scope["id"] == scopeID {
			return scope
		}
	}
	t.Fatalf("Scope %s missing: %#v", scopeID, graph["scopes"])
	return nil
}

func replaceFileTokens(t *testing.T, path string, replacements map[string]string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value := string(contents)
	for old, replacement := range replacements {
		value = strings.ReplaceAll(value, old, replacement)
	}
	mustWrite(t, path, value)
}

func zipRegistryDirectory(t *testing.T, root string) []byte {
	t.Helper()
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entry, err := writer.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = entry.Write(contents)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

func fileURLForE2E(path string) string {
	value := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return (&url.URL{Scheme: "file", Path: value}).String()
}

func fileLineCount(t *testing.T, path string) int {
	t.Helper()
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.FieldsFunc(string(contents), func(value rune) bool { return value == '\n' }))
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}
