package test

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestWorkspaceEndToEnd(t *testing.T) {
	repository := repositoryRoot(t)
	root := filepath.Join(repository, "temp", "e2e-run")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(repository, "test", "e2e", "case")
	bin := filepath.Join(root, "bin")
	mustMkdir(t, bin)

	locusExe := filepath.Join(bin, executableName("locus"))
	helperExe := filepath.Join(bin, executableName("probe-helper"))
	goBuild(t, repository, locusExe, "./cmd/locus")
	goBuild(t, repository, helperExe, "./test/helper")
	for _, name := range []string{"frpc", "ssh", "salt"} {
		copyFile(t, helperExe, filepath.Join(bin, executableName(name)))
	}

	listener, port := startEndpoint(t)
	defer listener.Close()
	materializeFixture(t, filepath.Join(caseRoot, "environments"), filepath.Join(root, "environments"), nil)
	materializeFixture(t, filepath.Join(caseRoot, "devices"), filepath.Join(root, "devices"), nil)

	projectA := filepath.Join(root, "projects", "alpha")
	projectARegistry := filepath.Join(projectA, ".locus", "registry")
	materializeProject(t, caseRoot, projectA, "project.alpha", "workstation.dev-a", port)

	projectB := filepath.Join(root, "projects", "beta")
	mustMkdir(t, projectB)
	projectBRegistry := filepath.Join(projectB, ".locus", "registry")
	deviceA := filepath.Join(root, "devices", "dev-a")

	statePath := filepath.Join(root, "state", "state.db")
	probeLog := filepath.Join(root, "probe-invocations.log")
	env := append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LOCUS_STATE_PATH="+statePath,
		"LOCUS_SIM_ROOT="+deviceA,
		"LOCUS_SIM_LOG="+probeLog,
	)

	initResult := runCLI(t, locusExe, projectB, env, "init", "--scope-kind", "project", "--scope-id", "project.beta", "--registry", projectBRegistry, "--json")
	assertStringAt(t, initResult, "scope", "id", "project.beta")
	materializeProject(t, caseRoot, projectB, "project.beta", "workstation.dev-b", port)

	runtimeArgs := []string{"--from", "workstation.dev-a", "--vantage", "office-lan", "--json"}
	validate := runCLI(t, locusExe, projectA, env, "validate", "--json")
	assertBoolAt(t, validate, "valid", true)
	runCLIExpectExit(t, locusExe, projectA, env, 2, "validate", "--vantage", "office-lan", "--json")

	nested := filepath.Join(projectA, "nested", "working", "directory")
	mustMkdir(t, nested)
	nestedValidation := runCLI(t, locusExe, nested, env, "validate", "--json")
	assertBoolAt(t, nestedValidation, "valid", true)

	contextResult := runCLI(t, locusExe, projectA, env, append([]string{"context"}, runtimeArgs...)...)
	assertStringAt(t, contextResult, "active_scope", "id", "project.alpha")
	assertStringAt(t, contextResult, "bindings", "production-host", "environment.customer-a::host.prod-01")
	assertStringAt(t, contextResult, "runtime", "current_entity", "project.alpha::workstation.dev-a")
	assertStringAt(t, contextResult, "runtime", "vantage", "office-lan")
	assertStringAt(t, contextResult, "runtime", "cwd", projectA)
	runtimeContext := mustObjectAt(t, contextResult, "runtime")
	assertStringAt(t, contextResult, "observation_store", statePath)
	for _, tool := range []string{"frpc", "salt", "ssh"} {
		assertArrayContains(t, runtimeContext["available_tools"], tool)
	}
	assertImport(t, contextResult["imports"], "customer", "environment.customer-a")

	list := runCLI(t, locusExe, nested, env, "list", "route", "--json")
	assertArrayContains(t, list["objects"], "project.alpha::route.prod-shell")
	assertArrayContains(t, list["objects"], "project.alpha::route.prod-salt")

	target := runCLI(t, locusExe, projectA, env, "show", "production-host", "--json")
	assertStringAt(t, target, "input_ref", "production-host")
	assertStringAt(t, target, "ref_type", "binding")
	assertStringAt(t, target, "canonical_target", "environment.customer-a::host.prod-01")
	assertStringAt(t, target, "object", "canonical_id", "environment.customer-a::host.prod-01")
	assertNoRuntimeEvidence(t, target)
	frpLink := runCLI(t, locusExe, projectA, env, "show", "link.prod-frp", "--json")
	assertStringAt(t, frpLink, "ref_type", "link")
	assertStringAt(t, frpLink, "object", "from", "project.alpha::workstation.dev-a")
	assertStringAt(t, frpLink, "object", "to", "environment.customer-a::frps.primary")
	assertNoRuntimeEvidence(t, frpLink)
	sshLink := runCLI(t, locusExe, projectA, env, "show", "link.prod-ssh", "--json")
	assertStringAt(t, sshLink, "object", "from", "project.alpha::workstation.dev-a")
	assertStringAt(t, sshLink, "object", "to", "environment.customer-a::host.prod-01")
	assertNoRuntimeEvidence(t, sshLink)
	shellRoute := runCLI(t, locusExe, projectA, env, "show", "route.prod-shell", "--json")
	assertRouteSteps(t, shellRoute, "project.alpha::link.prod-frp", "project.alpha::link.prod-ssh")
	assertNoRuntimeEvidence(t, shellRoute)

	resolveArgs := append([]string{"resolve", "production-host", "--capability", "shell"}, runtimeArgs...)
	before := runCLI(t, locusExe, projectA, env, resolveArgs...)
	assertStringAt(t, before, "status", "resolved")
	assertStringAt(t, before, "canonical_target", "environment.customer-a::host.prod-01")
	assertStringAt(t, before, "route", "derived_target", "environment.customer-a::host.prod-01")
	assertStringAt(t, before, "route", "evidence_status", "unknown")
	route := mustObjectAt(t, before, "route")
	assertArrayContains(t, route["derived_provides"], "tcp-forward.ssh")
	assertArrayContains(t, route["derived_provides"], "shell")
	assertResolvedProviders(t, route["steps"], "frp-stcp", "ssh")
	assertObservationCount(t, statePath, 0)
	assertProbeInvocations(t, probeLog)

	probe := runCLI(t, locusExe, projectA, env, append([]string{"probe", "route.prod-shell"}, runtimeArgs...)...)
	assertStringAt(t, probe, "status", "success")
	if observations, ok := probe["observations"].([]any); !ok || len(observations) != 2 {
		t.Fatalf("expected two observations, got %#v", probe)
	}
	assertObservationCount(t, statePath, 2)
	assertProbeInvocations(t, probeLog, "frpc", "ssh")
	status := runCLI(t, locusExe, projectA, env, "status", "route.prod-shell", "--vantage", "office-lan", "--json")
	assertStringAt(t, status, "evidence", "status", "success")
	after := runCLI(t, locusExe, projectA, env, resolveArgs...)
	assertStringAt(t, after, "route", "evidence_status", "success")
	observedSSH := runCLI(t, locusExe, projectA, env, "status", "link.prod-ssh", "--vantage", "office-lan", "--json")
	assertStringAt(t, observedSSH, "evidence", "observation", "subject", "project.alpha::link.prod-ssh")
	assertStringAt(t, observedSSH, "evidence", "observation", "vantage", "office-lan")
	assertStringAt(t, observedSSH, "evidence", "observation", "status", "success")
	linkProbe := runCLI(t, locusExe, projectA, env, append([]string{"probe", "link.prod-ssh"}, runtimeArgs...)...)
	assertStringAt(t, linkProbe, "subject_type", "link")
	assertObservation(t, linkProbe["observations"], "project.alpha::link.prod-ssh", "office-lan", "success")
	assertObservationCount(t, statePath, 3)
	assertProbeInvocations(t, probeLog, "frpc", "ssh", "ssh")

	saltResolveArgs := append([]string{"resolve", "production-host", "--capability", "salt.ping"}, runtimeArgs...)
	saltBefore := runCLI(t, locusExe, projectA, env, saltResolveArgs...)
	assertStringAt(t, saltBefore, "route", "evidence_status", "unknown")
	assertProbeInvocations(t, probeLog, "frpc", "ssh", "ssh")
	saltProbe := runCLI(t, locusExe, projectA, env, append([]string{"probe", "route.prod-salt"}, runtimeArgs...)...)
	if observations, ok := saltProbe["observations"].([]any); !ok || len(observations) != 1 {
		t.Fatalf("expected one Salt observation, got %#v", saltProbe)
	}
	saltAfter := runCLI(t, locusExe, projectA, env, saltResolveArgs...)
	assertStringAt(t, saltAfter, "route", "evidence_status", "success")
	assertResolvedProviders(t, mustObjectAt(t, saltAfter, "route")["steps"], "salt")
	assertNativeHint(t, saltAfter, "salt", "customer-a-prod-01", "test.ping", "--out=json")
	saltStatus := runCLI(t, locusExe, projectA, env, "status", "route.prod-salt", "--vantage", "office-lan", "--json")
	assertStringAt(t, saltStatus, "evidence", "status", "success")

	saltUp := filepath.Join(deviceA, "salt-up")
	saltDown := filepath.Join(deviceA, "salt-down")
	if err := os.Rename(saltUp, saltDown); err != nil {
		t.Fatal(err)
	}
	restoreSalt := func() {
		if _, err := os.Stat(saltDown); err == nil {
			_ = os.Rename(saltDown, saltUp)
		}
	}
	defer restoreSalt()
	failedProbe := runCLIExpectExitJSON(t, locusExe, projectA, env, 4, append([]string{"probe", "route.prod-salt"}, runtimeArgs...)...)
	assertStringAt(t, failedProbe, "status", "failure")
	assertObservation(t, failedProbe["observations"], "project.alpha::link.prod-salt", "office-lan", "failure")
	failedSaltStatus := runCLI(t, locusExe, projectA, env, "status", "route.prod-salt", "--vantage", "office-lan", "--json")
	assertStringAt(t, failedSaltStatus, "evidence", "status", "failure")
	failedSaltResolve := runCLI(t, locusExe, projectA, env, saltResolveArgs...)
	assertStringAt(t, failedSaltResolve, "route", "evidence_status", "failure")

	restoreSalt()
	runCLI(t, locusExe, projectA, env, append([]string{"probe", "route.prod-salt"}, runtimeArgs...)...)
	recoveredSalt := runCLI(t, locusExe, projectA, env, saltResolveArgs...)
	assertStringAt(t, recoveredSalt, "route", "evidence_status", "success")

	betaArgs := []string{"--from", "workstation.dev-b", "--vantage", "device-b", "--json"}
	betaContext := runCLI(t, locusExe, projectB, env, append([]string{"context"}, betaArgs...)...)
	assertStringAt(t, betaContext, "active_scope", "id", "project.beta")
	assertStringAt(t, betaContext, "bindings", "production-host", "environment.customer-a::host.prod-01")
	assertStringAt(t, betaContext, "runtime", "vantage", "device-b")
	betaResolve := runCLI(t, locusExe, projectB, env, append([]string{"resolve", "production-host", "--capability", "shell"}, betaArgs...)...)
	assertStringAt(t, betaResolve, "canonical_target", "environment.customer-a::host.prod-01")
	assertStringAt(t, betaResolve, "route", "evidence_status", "unknown")
	betaStatus := runCLI(t, locusExe, projectB, env, "status", "route.prod-shell", "--vantage", "device-b", "--json")
	assertStringAt(t, betaStatus, "evidence", "status", "unknown")

	assertWebEndToEnd(t, locusExe, projectA, env, deviceA)

	unresolved := runCLIExpectExitJSON(t, locusExe, projectA, env, 3, append([]string{"resolve", "production-host", "--capability", "missing.capability"}, runtimeArgs...)...)
	assertStringAt(t, unresolved, "status", "unresolved")

	duplicateRoute := filepath.Join(projectARegistry, "routes", "alternate-shell.yaml")
	mustWrite(t, duplicateRoute, "api_version: locus/v0\ntype: route\nid: route.alternate-shell\nsteps:\n  - link: link.prod-frp\n  - link: link.prod-ssh\n")
	ambiguous := runCLIExpectExitJSON(t, locusExe, projectA, env, 3, resolveArgs...)
	assertStringAt(t, ambiguous, "status", "ambiguous")
	assertCandidateRoutes(t, ambiguous["candidates"], "project.alpha::route.alternate-shell", "project.alpha::route.prod-shell")
	if err := os.Remove(duplicateRoute); err != nil {
		t.Fatal(err)
	}
}

func assertWebEndToEnd(t *testing.T, executable, project string, env []string, deviceRoot string) {
	t.Helper()
	webBase, webClient := startWeb(t, executable, project, env)
	webContext, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/context", "", http.StatusOK)
	assertStringAt(t, webContext, "active_scope", "id", "project.alpha")
	assertStringAt(t, webContext, "runtime", "current_entity", "project.alpha::workstation.dev-a")

	webGraph, graphBody := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/graph", "", http.StatusOK)
	if strings.Contains(string(graphBody), "credential_ref") || strings.Contains(string(graphBody), "secret://") || strings.Contains(string(graphBody), "provider_data") {
		t.Fatalf("graph leaked provider or secret data: %s", graphBody)
	}
	if entities, ok := webGraph["entities"].([]any); !ok || len(entities) != 3 {
		t.Fatalf("unexpected Web graph entities: %#v", webGraph)
	}
	if routes, ok := webGraph["routes"].([]any); !ok || len(routes) != 2 {
		t.Fatalf("unexpected Web graph routes: %#v", webGraph)
	}

	knowledge, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/knowledge", "", http.StatusOK)
	documentID := assertKnowledgeIndex(t, knowledge["documents"])
	document, documentBody := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/knowledge/"+documentID, "", http.StatusOK)
	assertStringAt(t, document, "format", "markdown")
	if !strings.Contains(string(documentBody), "The shell route uses the existing FRP endpoint") {
		t.Fatalf("project documentation body missing: %s", documentBody)
	}
	webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/knowledge/not-a-document-path", "", http.StatusNotFound)

	webResolve, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/resolve?target=production-host&capability=shell&from=workstation.dev-a&vantage=office-lan", "", http.StatusOK)
	assertStringAt(t, webResolve, "status", "resolved")
	assertStringAt(t, webResolve, "route", "canonical_id", "project.alpha::route.prod-shell")

	isolatedStatus, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/status?vantage=device-b", "", http.StatusOK)
	assertWebRouteStatus(t, isolatedStatus, "project.alpha::route.prod-salt", "unknown")

	saltUp := filepath.Join(deviceRoot, "salt-up")
	saltDown := filepath.Join(deviceRoot, "salt-down")
	restoreSalt := func() {
		if _, err := os.Stat(saltDown); err == nil {
			_ = os.Rename(saltDown, saltUp)
		}
	}
	t.Cleanup(restoreSalt)
	if err := os.Rename(saltUp, saltDown); err != nil {
		t.Fatal(err)
	}
	webFailure, _ := webJSON(t, webClient, http.MethodPost, webBase+"/api/v0/probes", `{"subject":"route.prod-salt","from":"workstation.dev-a","vantage":"office-lan","timeout_ms":5000}`, http.StatusOK)
	assertStringAt(t, webFailure, "status", "failure")
	failedWebStatus, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/status?vantage=office-lan", "", http.StatusOK)
	assertWebRouteStatus(t, failedWebStatus, "project.alpha::route.prod-salt", "failure")

	restoreSalt()
	webRecovery, _ := webJSON(t, webClient, http.MethodPost, webBase+"/api/v0/probes", `{"subject":"route.prod-salt","from":"workstation.dev-a","vantage":"office-lan","timeout_ms":5000}`, http.StatusOK)
	assertStringAt(t, webRecovery, "status", "success")
	recoveredWebStatus, _ := webJSON(t, webClient, http.MethodGet, webBase+"/api/v0/status?vantage=office-lan", "", http.StatusOK)
	assertWebRouteStatus(t, recoveredWebStatus, "project.alpha::route.prod-salt", "success")

	_, uiBody := webJSON(t, webClient, http.MethodGet, webBase+"/graph", "", http.StatusOK)
	if !bytes.Contains(uiBody, []byte(`<div id="app"></div>`)) {
		t.Fatal("embedded Web UI entry missing")
	}
}

func startWeb(t *testing.T, executable, cwd string, env []string) (string, *http.Client) {
	t.Helper()
	command := exec.Command(executable, "web", "--from", "workstation.dev-a", "--vantage", "office-lan", "--address", "127.0.0.1:0")
	command.Dir = cwd
	command.Env = env
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("read Web startup: %v: %s", err, stderr.String())
	}
	const prefix = "Locus Web listening at "
	if !strings.HasPrefix(line, prefix) {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("unexpected Web startup output %q: %s", line, stderr.String())
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	return strings.TrimSpace(strings.TrimPrefix(line, prefix)), &http.Client{Timeout: 5 * time.Second}
}

func webJSON(t *testing.T, client *http.Client, method, endpoint, body string, wantStatus int) (map[string]any, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", method, endpoint, response.StatusCode, wantStatus, payload)
	}
	result := map[string]any{}
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(payload, &result); err != nil {
			t.Fatalf("decode %s %s: %v: %s", method, endpoint, err, payload)
		}
	}
	return result, payload
}

func assertKnowledgeIndex(t *testing.T, value any) string {
	t.Helper()
	documents, ok := value.([]any)
	if !ok || len(documents) != 2 {
		t.Fatalf("unexpected knowledge index: %#v", value)
	}
	for _, item := range documents {
		document, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("unexpected document: %#v", item)
		}
		path, _ := document["path"].(string)
		if filepath.IsAbs(path) || strings.Contains(filepath.ToSlash(path), "../") {
			t.Fatalf("document path escaped Scope docs: %#v", document)
		}
		if document["title"] == "Production access" {
			associations, ok := document["associations"].([]any)
			if !ok || len(associations) != 2 {
				t.Fatalf("project document was not deduplicated: %#v", document)
			}
			id, ok := document["id"].(string)
			if !ok || id == "" {
				t.Fatalf("document ID missing: %#v", document)
			}
			return id
		}
	}
	t.Fatalf("Production access document missing: %#v", value)
	return ""
}

func assertWebRouteStatus(t *testing.T, status map[string]any, routeID, want string) {
	t.Helper()
	routes, ok := status["routes"].([]any)
	if !ok {
		t.Fatalf("missing Web route status: %#v", status)
	}
	for _, item := range routes {
		route, ok := item.(map[string]any)
		if !ok || route["route_id"] != routeID {
			continue
		}
		evidence, ok := route["evidence"].(map[string]any)
		if !ok || evidence["status"] != want {
			t.Fatalf("route %s evidence = %#v, want %s", routeID, route["evidence"], want)
		}
		return
	}
	t.Fatalf("route %s missing from Web status: %#v", routeID, status)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func goBuild(t *testing.T, repository, output, packagePath string) {
	t.Helper()
	command := exec.Command("go", "build", "-o", output, packagePath)
	command.Dir = repository
	command.Env = os.Environ()
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", packagePath, err, combined)
	}
}

func startEndpoint(t *testing.T) (net.Listener, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, connection)
			_ = connection.Close()
		}
	}()
	return listener, listener.Addr().(*net.TCPAddr).Port
}

func materializeProject(t *testing.T, caseRoot, target, projectID, workstation string, port int) {
	t.Helper()
	registry := filepath.Join(target, ".locus", "registry")
	materializeFixture(t, filepath.Join(caseRoot, "project"), target, map[string]string{
		"{{PROJECT_ID}}":  projectID,
		"{{WORKSTATION}}": workstation,
		"{{PORT}}":        strconv.Itoa(port),
		"{{FRPC_CONFIG}}": filepath.ToSlash(filepath.Join(registry, "frpc.toml")),
	})
}

func materializeFixture(t *testing.T, source, target string, replacements map[string]string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			materializeFixture(t, sourcePath, targetPath, replacements)
			continue
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for token, value := range replacements {
			content = strings.ReplaceAll(content, token, value)
		}
		mustWrite(t, targetPath, content)
	}
}

func runCLI(t *testing.T, executable, cwd string, env []string, args ...string) map[string]any {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Dir, command.Env = cwd, env
	output, err := command.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("locus %s failed: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, output, exit.Stderr)
		}
		t.Fatalf("locus %s failed: %v", strings.Join(args, " "), err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("invalid JSON from %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return result
}

func runCLIExpectExit(t *testing.T, executable, cwd string, env []string, want int, args ...string) {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Dir, command.Env = cwd, env
	output, err := command.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != want {
		t.Fatalf("expected locus %s to exit %d, got %v\n%s", strings.Join(args, " "), want, err, output)
	}
}

func runCLIExpectExitJSON(t *testing.T, executable, cwd string, env []string, want int, args ...string) map[string]any {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Dir, command.Env = cwd, env
	output, err := command.Output()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != want {
		t.Fatalf("expected locus %s to exit %d, got %v\nstdout: %s", strings.Join(args, " "), want, err, output)
	}
	var result map[string]any
	if unmarshalErr := json.Unmarshal(output, &result); unmarshalErr != nil {
		t.Fatalf("invalid failure JSON from %s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), unmarshalErr, output, exit.Stderr)
	}
	return result
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertStringAt(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()
	want := keys[len(keys)-1]
	current := any(value)
	for _, key := range keys[:len(keys)-1] {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object in %#v", key, value)
		}
		current = object[key]
	}
	if current != want {
		t.Fatalf("expected %q at %v, got %#v", want, keys[:len(keys)-1], current)
	}
}

func assertBoolAt(t *testing.T, value map[string]any, key string, want bool) {
	t.Helper()
	if value[key] != want {
		t.Fatalf("expected %s=%t, got %#v", key, want, value[key])
	}
}

func assertArrayContains(t *testing.T, value any, want string) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected array, got %#v", value)
	}
	for _, item := range items {
		if item == want {
			return
		}
	}
	t.Fatalf("expected array to contain %q, got %#v", want, value)
}

func mustObjectAt(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	object, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %#v", key, value[key])
	}
	return object
}

func assertImport(t *testing.T, value any, alias, scopeID string) {
	t.Helper()
	imports, ok := value.([]any)
	if !ok || len(imports) != 1 {
		t.Fatalf("expected one import, got %#v", value)
	}
	item, ok := imports[0].(map[string]any)
	if !ok || item["alias"] != alias || item["scope_id"] != scopeID {
		t.Fatalf("expected import %s -> %s, got %#v", alias, scopeID, value)
	}
}

func assertRouteSteps(t *testing.T, result map[string]any, expected ...string) {
	t.Helper()
	object := mustObjectAt(t, result, "object")
	steps, ok := object["steps"].([]any)
	if !ok || len(steps) != len(expected) {
		t.Fatalf("expected route steps %v, got %#v", expected, object["steps"])
	}
	for index, linkID := range expected {
		step, ok := steps[index].(map[string]any)
		if !ok || step["link"] != linkID {
			t.Fatalf("expected step %d to be %s, got %#v", index, linkID, steps[index])
		}
	}
}

func assertResolvedProviders(t *testing.T, value any, expected ...string) {
	t.Helper()
	steps, ok := value.([]any)
	if !ok || len(steps) != len(expected) {
		t.Fatalf("expected providers %v, got %#v", expected, value)
	}
	for index, provider := range expected {
		step, ok := steps[index].(map[string]any)
		if !ok || step["provider"] != provider {
			t.Fatalf("expected provider %d to be %s, got %#v", index, provider, steps[index])
		}
	}
}

func assertNativeHint(t *testing.T, result map[string]any, executable string, args ...string) {
	t.Helper()
	route := mustObjectAt(t, result, "route")
	steps, ok := route["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("expected one resolved step, got %#v", route["steps"])
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected resolved step object, got %#v", steps[0])
	}
	hint, ok := step["native_hint"].(map[string]any)
	if !ok || hint["executable"] != executable {
		t.Fatalf("expected %s NativeHint, got %#v", executable, step["native_hint"])
	}
	actual, ok := hint["args"].([]any)
	if !ok || len(actual) != len(args) {
		t.Fatalf("expected NativeHint args %v, got %#v", args, hint["args"])
	}
	for index, expected := range args {
		if actual[index] != expected {
			t.Fatalf("expected NativeHint arg %d to be %s, got %#v", index, expected, actual[index])
		}
	}
}

func assertObservation(t *testing.T, value any, subject, vantage, status string) {
	t.Helper()
	observations, ok := value.([]any)
	if !ok || len(observations) == 0 {
		t.Fatalf("expected observations, got %#v", value)
	}
	observation, ok := observations[0].(map[string]any)
	if !ok || observation["subject"] != subject || observation["vantage"] != vantage || observation["status"] != status {
		t.Fatalf("expected %s observation for %s at %s, got %#v", status, subject, vantage, value)
	}
}

func assertNoRuntimeEvidence(t *testing.T, value map[string]any) {
	t.Helper()
	for _, key := range []string{"observation", "observations", "evidence", "evidence_status"} {
		if _, exists := value[key]; exists {
			t.Fatalf("show output unexpectedly contains %q: %#v", key, value)
		}
	}
}

func assertObservationCount(t *testing.T, statePath string, want int) {
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
	if count != want {
		t.Fatalf("expected %d persisted link observations, got %d", want, count)
	}
	var routeCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM observations WHERE subject LIKE '%::route.%'`).Scan(&routeCount); err != nil {
		t.Fatal(err)
	}
	if routeCount != 0 {
		t.Fatalf("route observations must not be persisted, got %d", routeCount)
	}
}

func assertProbeInvocations(t *testing.T, logPath string, expected ...string) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) && len(expected) == 0 {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	actual := strings.Fields(string(data))
	if !slices.Equal(actual, expected) {
		t.Fatalf("expected probe invocations %v, got %v", expected, actual)
	}
}

func assertCandidateRoutes(t *testing.T, value any, expected ...string) {
	t.Helper()
	candidates, ok := value.([]any)
	if !ok || len(candidates) != len(expected) {
		t.Fatalf("expected candidate routes %v, got %#v", expected, value)
	}
	actual := make([]string, 0, len(candidates))
	for _, value := range candidates {
		candidate, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected candidate object, got %#v", value)
		}
		id, ok := candidate["canonical_id"].(string)
		if !ok {
			t.Fatalf("candidate lacks canonical_id: %#v", candidate)
		}
		actual = append(actual, id)
	}
	if !slices.Equal(actual, expected) {
		t.Fatalf("expected candidate routes %v, got %v", expected, actual)
	}
}
