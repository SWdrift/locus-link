package cli

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionAndMigrateCommands(t *testing.T) {
	version := runInternalCLI(t, "version", "--json")
	if version["version"] == "" || version["previous_version"] == "" || version["state_schema_version"] != float64(1) {
		t.Fatalf("unexpected version result: %#v", version)
	}

	root := workspaceTestPath(t, "cli-migration")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state.db")
	backupDir := filepath.Join(root, "backups")
	result := runInternalCLI(t, "migrate", "--state", statePath, "--backup-dir", backupDir, "--json")
	if result["status"] != "initialized" || result["to_version"] != float64(1) {
		t.Fatalf("unexpected migration result: %#v", result)
	}
}

func TestDeclarationCommandsDoNotInvokeExternalTools(t *testing.T) {
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
	t.Setenv("LOCUS_HOME", filepath.Join(root, "home"))
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(root, "state", "state.db"))
	if executable, err := exec.LookPath("ssh"); err == nil {
		t.Fatalf("test PATH unexpectedly contains ssh at %s", executable)
	}
	assertDeclarationCommands(t, registry)

	legacyStdout, legacyStderr := bytes.Buffer{}, bytes.Buffer{}
	if exitCode := NewCLI(&legacyStdout, &legacyStderr).Run([]string{"init", "--scope-kind", "project", "--scope-id", "legacy"}); exitCode != 2 {
		t.Fatalf("legacy --scope-kind exited %d: %s", exitCode, legacyStderr.String())
	}
}

func TestUserAndProjectAuthoringCommands(t *testing.T) {
	base := workspaceTestPath(t, "cli-authoring-v1")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCUS_HOME", filepath.Join(base, "home"))
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(base, "state", "state.db"))

	runInternalCLI(t, "user", "init", "--scope-id", "scope.user", "--json")
	created := runInternalCLI(t, "init", "--scope-id", "scope.project", "--import-user", "user", "--register", "--json")
	if created["scope_id"] != "scope.project" {
		t.Fatalf("unexpected init result: %#v", created)
	}
	manifest, err := os.ReadFile(filepath.Join(base, ".locus", "registry", "scope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(manifest)
	for _, required := range []string{"api_version: locus/v1", "scope_id: scope.project", "scope_id: scope.user", "uri: ${LOCUS_HOME}/registry"} {
		if !strings.Contains(text, required) {
			t.Fatalf("manifest missing %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, "kind: project") || strings.Contains(text, "scope:") {
		t.Fatalf("legacy Scope schema leaked into manifest:\n%s", text)
	}
	projects := runInternalCLI(t, "project", "list", "--json")
	values, ok := projects["projects"].([]any)
	if !ok || len(values) != 1 {
		t.Fatalf("unexpected project list: %#v", projects)
	}
	var stdout, stderr bytes.Buffer
	if code := NewCLI(&stdout, &stderr).Run([]string{"user", "init", "--scope-id", "scope.other", "--json"}); code != 2 {
		t.Fatalf("user init overwrite exited %d: %s", code, stderr.String())
	}
}

func TestRefreshCLIReportsFailureResultAndExitCodes(t *testing.T) {
	base := workspaceTestPath(t, "cli-refresh")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	t.Setenv("LOCUS_HOME", home)
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(base, "state", "state.db"))
	payload := cliRegistryZIP(t)
	failing := true
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if failing {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "api_version: locus/v1\nscope_id: scope.root\nimports:\n  remote:\n    scope_id: scope.remote\n    source:\n      kind: url\n      uri: " + server.URL + "/registry.zip\n"
	if err := os.WriteFile(filepath.Join(root, "scope.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := NewCLI(&stdout, &stderr).Run([]string{"refresh", "remote", "--registry", root, "--json"}); code != 5 {
		t.Fatalf("failed refresh exited %d: %s", code, stderr.String())
	}
	var failed map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatal(err)
	}
	if failed["status"] != "failure" {
		t.Fatalf("failed refresh omitted structured result: %#v", failed)
	}

	failing = false
	stdout.Reset()
	stderr.Reset()
	if code := NewCLI(&stdout, &stderr).Run([]string{"refresh", "remote", "--registry", root, "--json"}); code != 0 {
		t.Fatalf("successful refresh exited %d: %s", code, stderr.String())
	}
	var succeeded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &succeeded); err != nil {
		t.Fatal(err)
	}
	if succeeded["status"] != "success" {
		t.Fatalf("unexpected refresh result: %#v", succeeded)
	}
	contextCode, contextResult, contextError := runCLIResult(t, "context", "--registry", root, "--from", "remote::remote", "--vantage", "test:vantage", "--json")
	if contextCode != 0 {
		t.Fatalf("context after refresh exited %d: %s", contextCode, contextError)
	}
	rootContext, ok := contextResult["root"].(map[string]any)
	if !ok {
		t.Fatalf("context omitted root metadata: %#v", contextResult)
	}
	cacheEntries, ok := rootContext["source_cache"].([]any)
	if !ok || len(cacheEntries) != 1 {
		t.Fatalf("context omitted active Source cache: %#v", rootContext)
	}
	cacheEntry, ok := cacheEntries[0].(map[string]any)
	if !ok || cacheEntry["active_content_digest"] == "" || cacheEntry["last_refresh_status"] != "success" ||
		cacheEntry["configured_source_digest"] != cacheEntry["current_source_digest"] || cacheEntry["configuration_changed"] != false {
		t.Fatalf("context returned incomplete cache provenance: %#v", cacheEntry)
	}

	payload = cliRegistryZIPWithManifest(t, "api_version: locus/v1\nscope_id: scope.remote\nimports:\n  self: .\n")
	stdout.Reset()
	stderr.Reset()
	if code := NewCLI(&stdout, &stderr).Run([]string{"refresh", "remote", "--registry", root, "--json"}); code != 6 {
		t.Fatalf("regressing refresh exited %d: %s", code, stderr.String())
	}
	var confirmation map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &confirmation); err != nil {
		t.Fatal(err)
	}
	candidateSnapshot, ok := confirmation["candidate_snapshot"].(map[string]any)
	candidateDigest, digestOK := candidateSnapshot["snapshot_digest"].(string)
	if confirmation["status"] != "confirmation_required" || !ok || !digestOK || candidateDigest == "" {
		t.Fatalf("refresh omitted candidate confirmation result: %#v", confirmation)
	}
	stdout.Reset()
	stderr.Reset()
	if code := NewCLI(&stdout, &stderr).Run([]string{
		"refresh", "remote", "--registry", root, "--allow-regression", "--expected-candidate-digest", candidateDigest, "--json",
	}); code != 0 {
		t.Fatalf("confirmed refresh exited %d: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := NewCLI(&stdout, &stderr).Run([]string{"refresh", "unknown", "--registry", root, "--json"}); code != 2 {
		t.Fatalf("unknown refresh target exited %d: %s", code, stderr.String())
	}
}

func cliRegistryZIP(t *testing.T) []byte {
	t.Helper()
	return cliRegistryZIPWithManifest(t, "api_version: locus/v1\nscope_id: scope.remote\n")
}

func cliRegistryZIPWithManifest(t *testing.T, manifest string) []byte {
	t.Helper()
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	files := map[string]string{
		"scope.yaml":           manifest,
		"entities/remote.yaml": "api_version: locus/v1\ntype: entity\nid: remote\nkind: service\nname: Remote\n",
	}
	for _, name := range []string{"scope.yaml", "entities/remote.yaml"} {
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

func TestPartialCommandsExposeStableTopLevelDiagnostics(t *testing.T) {
	base := workspaceTestPath(t, "cli-partial")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	root := writePartialCLIRegistry(t, base)
	t.Setenv("LOCUS_HOME", filepath.Join(base, "home"))
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(base, "state", "state.db"))

	for _, command := range [][]string{
		{"graph", "--registry", root, "--json"},
		{"list", "entity", "--registry", root, "--json"},
		{"show", "workstation", "--registry", root, "--json"},
		{"context", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json"},
		{"status", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json"},
	} {
		code, result, stderr := runCLIResult(t, command...)
		if code != 0 {
			t.Fatalf("locus %v exited %d: %s", command, code, stderr)
		}
		assertPartialDiagnostics(t, result)
	}

	code, graph, stderr := runCLIResult(t, "graph", "--registry", root, "--json")
	if code != 0 {
		t.Fatalf("graph exited %d: %s", code, stderr)
	}
	if scopes, ok := graph["scopes"].([]any); !ok || len(scopes) != 1 {
		t.Fatalf("partial graph omitted loaded Scope: %#v", graph)
	}
	if entities, ok := graph["entities"].([]any); !ok || len(entities) != 2 {
		t.Fatalf("partial graph omitted loaded entities: %#v", graph)
	}

	code, resolved, stderr := runCLIResult(t, "resolve", "server", "--capability", "shell", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json")
	if code != 3 {
		t.Fatalf("partial resolve exited %d: %s", code, stderr)
	}
	assertPartialDiagnostics(t, resolved)
	if resolved["status"] != "incomplete" || resolved["route"] != nil {
		t.Fatalf("partial resolve claimed a unique route: %#v", resolved)
	}
	if candidates, ok := resolved["candidates"].([]any); !ok || len(candidates) != 1 {
		t.Fatalf("partial resolve omitted discovered candidate: %#v", resolved)
	}
	code, unresolved, stderr := runCLIResult(t, "resolve", "remote::missing", "--capability", "shell", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json")
	if code != 3 {
		t.Fatalf("partial unknown resolve exited %d: %s", code, stderr)
	}
	if unresolved["status"] != "incomplete" {
		t.Fatalf("partial unknown resolve claimed absence: %#v", unresolved)
	}

	code, probed, stderr := runCLIResult(t, "probe", "connection", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--timeout", "250ms", "--json")
	if code != 4 {
		t.Fatalf("expected deterministic connection failure, got %d: %s", code, stderr)
	}
	assertPartialDiagnostics(t, probed)
	if observations, ok := probed["observations"].([]any); !ok || len(observations) != 1 {
		t.Fatalf("partial probe did not measure loaded Link: %#v", probed)
	}
	statePath := filepath.Join(base, "state", "state.db")
	before := observationCount(t, statePath)
	code, _, stderr = runCLIResult(t, "probe", "remote::missing", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--timeout", "250ms", "--json")
	if code != 2 {
		t.Fatalf("blocked probe reference exited %d: %s", code, stderr)
	}
	if after := observationCount(t, statePath); after != before {
		t.Fatalf("blocked probe appended observations: before=%d after=%d", before, after)
	}

	code, validation, _ := runCLIResult(t, "validate", "--registry", root, "--json")
	if code != 2 || validation["valid"] != false {
		t.Fatalf("partial validate returned wrong contract: code=%d result=%#v", code, validation)
	}
	assertPartialDiagnostics(t, validation)
}

func writePartialCLIRegistry(t *testing.T, base string) string {
	t.Helper()
	root := filepath.Join(base, "root")
	for _, directory := range []string{"entities", "links", "routes"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"scope.yaml": "api_version: locus/v1\nscope_id: scope.root\nimports:\n  remote:\n    scope_id: scope.remote\n    source:\n      kind: url\n      uri: https://example.invalid/registry.zip\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v1\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
		filepath.Join("entities", "server.yaml"):      "api_version: locus/v1\ntype: entity\nid: server\nkind: server\nname: Server\n",
		filepath.Join("links", "connection.yaml"):     "api_version: locus/v1\ntype: link\nid: connection\nfrom: workstation\nto: server\nprovider: ssh\nprovides:\n  - shell\nprovider_data:\n  user: root\n  host: 127.0.0.1\n  port: 1\n",
		filepath.Join("routes", "shell.yaml"):         "api_version: locus/v1\ntype: route\nid: shell\nsteps:\n  - link: connection\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runCLIResult(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := NewCLI(&stdout, &stderr).Run(args)
	result := map[string]any{}
	if stdout.Len() != 0 {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode locus %v output: %v\n%s", args, err, stdout.String())
		}
	}
	return code, result, stderr.String()
}

func assertPartialDiagnostics(t *testing.T, result map[string]any) {
	t.Helper()
	if result["completeness"] != "partial" {
		t.Fatalf("missing partial completeness: %#v", result)
	}
	blocked, ok := result["blocked_imports"].([]any)
	if !ok || len(blocked) != 1 {
		t.Fatalf("missing blocked import diagnostics: %#v", result)
	}
	diagnostic, ok := blocked[0].(map[string]any)
	if !ok || diagnostic["reason"] != "missing_active_cache" {
		t.Fatalf("unstable blocked import diagnostic: %#v", blocked)
	}
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
		"scope.yaml": "api_version: locus/v1\nscope_id: project.test\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v1\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
		filepath.Join("entities", "server.yaml"):      "api_version: locus/v1\ntype: entity\nid: server\nkind: server\nname: Server\n",
		filepath.Join("links", "connection.yaml"):     "api_version: locus/v1\ntype: link\nid: connection\nfrom: workstation\nto: server\nprovider: ssh\nprovides:\n  - shell\nprovider_data:\n  user: root\n  host: example.test\n  port: 22\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func workspaceTestPath(t *testing.T, elements ...string) string {
	t.Helper()
	parts := append([]string{"..", "..", "temp", "unit-tests"}, elements...)
	path, err := filepath.Abs(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func observationCount(t *testing.T, statePath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
