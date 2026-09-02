package cli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"locus-link/internal/locus"
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

	runInternalCLI(t, "init", "--user", "--scope-id", "scope.user", "--json")
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
}

func TestCLI02CommandSurfaceAndInference(t *testing.T) {
	base := workspaceTestPath(t, "cli-v02-surface")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "registry")
	writeDeclarationTestRegistry(t, root)
	t.Setenv("LOCUS_HOME", filepath.Join(base, "home"))
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(base, "state", "state.db"))

	code, contextResult, stderr := runCLIResult(t, "context", "--registry", root, "--json")
	if code != 0 || contextResult["active_scope"] != "project.test" {
		t.Fatalf("context without --from failed: code=%d stderr=%s result=%#v", code, stderr, contextResult)
	}
	runtime := contextResult["runtime"].(map[string]any)
	if runtime["current_entity"] != "" {
		t.Fatalf("context guessed current entity: %#v", runtime)
	}
	code, statusResult, stderr := runCLIResult(t, "status", "--registry", root, "--json")
	if code != 0 || statusResult["summary"] == nil {
		t.Fatalf("status without --from failed: code=%d stderr=%s result=%#v", code, stderr, statusResult)
	}
	code, resolved, stderr := runCLIResult(t, "resolve", "server", "shell", "--registry", root, "--json")
	if code != 0 {
		t.Fatalf("resolve inference failed: code=%d stderr=%s", code, stderr)
	}
	route := resolved["route"].(map[string]any)
	if route["from"] != "project.test::workstation" {
		t.Fatalf("resolve omitted inferred origin: %#v", route)
	}
	code, _, stderr = runCLIResult(t, "resolve", "server", "--registry", root, "--json")
	if code != 2 || !strings.Contains(stderr, "accepts 2 argument") {
		t.Fatalf("missing positional capability was not validated: code=%d stderr=%s", code, stderr)
	}
	var completionOut, completionErr bytes.Buffer
	if code := NewCLI(&completionOut, &completionErr).Run([]string{"completion", "bash"}); code != 0 || !strings.Contains(completionOut.String(), "__start_locus") {
		t.Fatalf("bash completion failed: code=%d stderr=%s", code, completionErr.String())
	}
}

func TestHumanResolveSummaryAndAdviceJSON(t *testing.T) {
	base := workspaceTestPath(t, "cli-human-output")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "registry")
	writeDeclarationTestRegistry(t, root)
	t.Setenv("LOCUS_HOME", filepath.Join(base, "home"))
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(base, "state", "state.db"))

	var stdout, stderr bytes.Buffer
	code := NewCLI(&stdout, &stderr).Run([]string{"resolve", "server", "shell", "--registry", root})
	if code != 0 {
		t.Fatalf("human resolve exited %d: %s", code, stderr.String())
	}
	for _, expected := range []string{
		"server → project.test::server",
		"Route: project.test::shell",
		"From: project.test::workstation",
		"Evidence: unknown",
		"1. project.test::connection",
		"Next:\n  locus probe project.test::shell",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("human output omitted %q:\n%s", expected, stdout.String())
		}
	}

	code, result, cliErr := runCLIResult(t, "resolve", "server", "shell", "--registry", root, "--json")
	if code != 0 {
		t.Fatalf("JSON resolve exited %d: %s", code, cliErr)
	}
	diagnostics, ok := result["diagnostics"].([]any)
	if !ok || len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "evidence.unknown" {
		t.Fatalf("unstable diagnostics: %#v", result)
	}
	actions, ok := result["next_actions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("missing next action: %#v", result)
	}
	action := actions[0].(map[string]any)
	if action["effect"] != "append_observation" || action["confirmation"] != "none" {
		t.Fatalf("invalid action side-effect metadata: %#v", action)
	}

	var ambiguous bytes.Buffer
	err := writeHuman(&ambiguous, locus.ResolveResult{
		Status: "ambiguous", InputTarget: "server", Target: "project.test::server", Capability: "shell",
		Candidates: []locus.ResolvedRoute{
			{CanonicalID: "project.test::route-a", From: "project.test::workstation-a"},
			{CanonicalID: "project.test::route-b", From: "project.test::workstation-b"},
		},
	})
	if err != nil || !strings.Contains(ambiguous.String(), "Cannot infer an origin") ||
		!strings.Contains(ambiguous.String(), "locus resolve server shell --from project.test::workstation-a") {
		t.Fatalf("ambiguous human output:\n%s\nerror: %v", ambiguous.String(), err)
	}

	var partial bytes.Buffer
	err = writeHuman(&partial, locus.ResolveResult{
		Status: "incomplete", InputTarget: "server", Capability: "shell", Completeness: locus.Partial,
		BlockedImports: []locus.BlockedImport{{SourceScopeID: "project.test", TargetScopeID: "environment.test", AliasPath: []string{"environment"}, Reason: "missing_active_cache"}},
	})
	if err != nil || !strings.Contains(partial.String(), "Declaration view: partial") ||
		!strings.Contains(partial.String(), "locus refresh environment") {
		t.Fatalf("partial human output:\n%s\nerror: %v", partial.String(), err)
	}
}

func TestDoctorReadsStateWithoutModifyingIt(t *testing.T) {
	base := workspaceTestPath(t, "cli-doctor-read-only")
	if err := os.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "registry")
	writeDeclarationTestRegistry(t, root)
	statePath := filepath.Join(base, "state", "state.db")
	t.Setenv("LOCUS_HOME", filepath.Join(base, "home"))
	t.Setenv("LOCUS_STATE_PATH", statePath)
	runInternalCLI(t, "migrate", "--state", statePath, "--json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	code, result, stderr := runCLIResult(t, "doctor", "--registry", root, "--json")
	if code != 0 {
		t.Fatalf("doctor exited %d: %s", code, stderr)
	}
	checks, ok := result["checks"].([]any)
	if !ok || len(checks) != 5 {
		t.Fatalf("doctor checks = %#v", result)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("doctor modified the State database")
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

	code, resolved, stderr := runCLIResult(t, "resolve", "server", "shell", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json")
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
	code, unresolved, stderr := runCLIResult(t, "resolve", "remote::missing", "shell", "--registry", root, "--from", "workstation", "--vantage", "test:vantage", "--json")
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
	for _, directory := range []string{"entities", "links", "routes"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"scope.yaml": "api_version: locus/v1\nscope_id: project.test\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v1\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
		filepath.Join("entities", "server.yaml"):      "api_version: locus/v1\ntype: entity\nid: server\nkind: server\nname: Server\n",
		filepath.Join("links", "connection.yaml"):     "api_version: locus/v1\ntype: link\nid: connection\nfrom: workstation\nto: server\nprovider: ssh\nprovides:\n  - shell\nprovider_data:\n  user: root\n  host: example.test\n  port: 22\n",
		filepath.Join("routes", "shell.yaml"):         "api_version: locus/v1\ntype: route\nid: shell\nsteps:\n  - link: connection\n",
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
