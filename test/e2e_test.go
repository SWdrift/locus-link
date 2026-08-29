package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWorkspaceEndToEnd(t *testing.T) {
	repository := repositoryRoot(t)
	root := filepath.Join(repository, "temp", "e2e-run")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	bin := filepath.Join(root, "bin")
	mustMkdir(t, bin)

	locusExe := filepath.Join(bin, executableName("locus"))
	helperExe := filepath.Join(bin, executableName("probe-helper"))
	goBuild(t, repository, locusExe, "./cmd/locus")
	goBuild(t, repository, helperExe, "./test/helper")
	copyFile(t, helperExe, filepath.Join(bin, executableName("frpc")))
	copyFile(t, helperExe, filepath.Join(bin, executableName("ssh")))

	listener, port := startEndpoint(t)
	defer listener.Close()

	environmentRegistry := filepath.Join(root, "environments", "customer-a", ".locus", "registry")
	writeEnvironment(t, environmentRegistry)

	projectA := filepath.Join(root, "projects", "alpha")
	projectARegistry := filepath.Join(projectA, ".locus", "registry")
	writeProject(t, projectARegistry, "project.alpha", "workstation.dev-a", port)

	projectB := filepath.Join(root, "projects", "beta")
	mustMkdir(t, projectB)
	projectBRegistry := filepath.Join(projectB, ".locus", "registry")
	deviceA := filepath.Join(root, "devices", "dev-a")
	mustMkdir(t, deviceA)
	mustWrite(t, filepath.Join(deviceA, "frp-up"), "up")
	mustWrite(t, filepath.Join(deviceA, "ssh-up"), "up")

	statePath := filepath.Join(root, "state", "state.db")
	env := append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"LOCUS_STATE_PATH="+statePath,
		"LOCUS_SIM_ROOT="+deviceA,
	)

	initResult := runCLI(t, locusExe, projectB, env, "init", "--scope-kind", "project", "--scope-id", "project.beta", "--registry", projectBRegistry, "--json")
	assertStringAt(t, initResult, "scope", "id", "project.beta")
	writeProject(t, projectBRegistry, "project.beta", "workstation.dev-b", port)

	common := []string{"--registry", projectARegistry, "--from", "workstation.dev-a", "--vantage", "office-lan", "--json"}
	validate := runCLI(t, locusExe, projectA, env, append([]string{"validate"}, common...)...)
	assertBoolAt(t, validate, "valid", true)
	contextResult := runCLI(t, locusExe, projectA, env, append([]string{"context"}, common...)...)
	assertStringAt(t, contextResult, "active_scope", "id", "project.alpha")
	list := runCLI(t, locusExe, projectA, env, append([]string{"list", "route"}, common...)...)
	assertArrayContains(t, list["objects"], "project.alpha::route.prod-shell")
	show := runCLI(t, locusExe, projectA, env, append([]string{"show", "production-host"}, common...)...)
	assertStringAt(t, show, "object", "canonical_id", "environment.customer-a::host.prod-01")

	resolveArgs := append([]string{"resolve", "production-host", "--capability", "shell"}, common...)
	before := runCLI(t, locusExe, projectA, env, resolveArgs...)
	assertStringAt(t, before, "route", "evidence_status", "unknown")

	check := runCLI(t, locusExe, projectA, env, append([]string{"check", "route.prod-shell"}, common...)...)
	if observations, ok := check["observations"].([]any); !ok || len(observations) != 2 {
		t.Fatalf("expected two observations, got %#v", check)
	}
	status := runCLI(t, locusExe, projectA, env, append([]string{"status", "route.prod-shell"}, common...)...)
	assertStringAt(t, status, "evidence", "status", "success")
	after := runCLI(t, locusExe, projectA, env, resolveArgs...)
	assertStringAt(t, after, "route", "evidence_status", "success")

	betaCommon := []string{"--registry", projectBRegistry, "--from", "workstation.dev-b", "--vantage", "device-b", "--json"}
	betaContext := runCLI(t, locusExe, projectB, env, append([]string{"context"}, betaCommon...)...)
	assertStringAt(t, betaContext, "active_scope", "id", "project.beta")
	betaResolve := runCLI(t, locusExe, projectB, env, append([]string{"resolve", "production-host", "--capability", "shell"}, betaCommon...)...)
	assertStringAt(t, betaResolve, "canonical_target", "environment.customer-a::host.prod-01")
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

func writeEnvironment(t *testing.T, registry string) {
	t.Helper()
	mustWrite(t, filepath.Join(registry, "scope.yaml"), "api_version: locus/v0\nscope:\n  id: environment.customer-a\n  kind: environment\n")
	mustWrite(t, filepath.Join(registry, "entities", "host.yaml"), "api_version: locus/v0\ntype: entity\nid: host.prod-01\nkind: host\nname: Production Host\n")
	mustWrite(t, filepath.Join(registry, "entities", "frps.yaml"), "api_version: locus/v0\ntype: entity\nid: frps.primary\nkind: service\nname: FRP Server\n")
}

func writeProject(t *testing.T, registry, scopeID, workstation string, port int) {
	t.Helper()
	manifest := "api_version: locus/v0\nscope:\n  id: " + scopeID + "\n  kind: project\nimports:\n  - alias: customer\n    path: ../../../../environments/customer-a/.locus/registry\nbindings:\n  production-host: customer::host.prod-01\n"
	mustWrite(t, filepath.Join(registry, "scope.yaml"), manifest)
	mustWrite(t, filepath.Join(registry, "entities", "workstation.yaml"), fmt.Sprintf("api_version: locus/v0\ntype: entity\nid: %s\nkind: workstation\nname: Simulated Workstation\n", workstation))
	mustWrite(t, filepath.Join(registry, "links", "frp.yaml"), fmt.Sprintf("api_version: locus/v0\ntype: link\nid: link.prod-frp\nfrom: %s\nto: customer::frps.primary\nprovider: frp-stcp\nprovides: [tcp-forward.ssh]\nprovider_data:\n  config: %s\n  local_host: 127.0.0.1\n  local_port: %d\n", workstation, filepath.ToSlash(filepath.Join(registry, "frpc.toml")), port))
	mustWrite(t, filepath.Join(registry, "links", "ssh.yaml"), fmt.Sprintf("api_version: locus/v0\ntype: link\nid: link.prod-ssh\nfrom: %s\nto: customer::host.prod-01\nprovider: ssh\nrequires: [tcp-forward.ssh]\nprovides: [shell, exec]\nprovider_data:\n  user: deploy\n  host: 127.0.0.1\n  port: %d\n  credential_ref: secret://ssh/customer-a-prod\n", workstation, port))
	mustWrite(t, filepath.Join(registry, "routes", "shell.yaml"), "api_version: locus/v0\ntype: route\nid: route.prod-shell\nsteps:\n  - link: link.prod-frp\n  - link: link.prod-ssh\n")
	mustWrite(t, filepath.Join(registry, "frpc.toml"), "# simulated config\n")
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
