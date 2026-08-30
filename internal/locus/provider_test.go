package locus

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	providerHelperCounter = "LOCUS_PROVIDER_HELPER_COUNTER"
	providerHelperOutput  = "LOCUS_PROVIDER_HELPER_OUTPUT"
)

func TestMain(m *testing.M) {
	if counterPath := os.Getenv(providerHelperCounter); counterPath != "" {
		file, err := os.OpenFile(counterPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(file, filepath.Base(os.Args[0]))
		_ = file.Close()
		_, _ = fmt.Fprint(os.Stderr, os.Getenv(providerHelperOutput))
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestBuiltInProviderValidationRequiresTypedNonEmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		expected []string
	}{
		{
			name:     "frp-stcp",
			provider: frpProvider{},
			expected: []string{"provider_data.config is required", "provider_data.local_host is required", "provider_data.local_port is required"},
		},
		{
			name:     "ssh",
			provider: sshProvider{},
			expected: []string{"provider_data.user is required", "provider_data.host is required", "provider_data.port is required"},
		},
		{
			name:     "salt",
			provider: saltProvider{},
			expected: []string{"provider_data.minion_id is required"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := test.provider.Validate(&Link{CanonicalID: "project.test/link.example"})
			if len(issues) != len(test.expected) {
				t.Fatalf("expected %d issues, got %v", len(test.expected), issues)
			}
			for index, expected := range test.expected {
				if !strings.Contains(issues[index], expected) {
					t.Fatalf("issue %d should contain %q, got %q", index, expected, issues[index])
				}
			}
		})
	}

	malformed := []struct {
		name     string
		provider Provider
		data     map[string]any
		field    string
	}{
		{"frp config type", frpProvider{}, map[string]any{"config": true, "local_host": "localhost", "local_port": 22}, "config"},
		{"frp blank host", frpProvider{}, map[string]any{"config": "frpc.toml", "local_host": "\t", "local_port": 22}, "local_host"},
		{"ssh user type", sshProvider{}, map[string]any{"user": []string{"root"}, "host": "localhost", "port": 22}, "user"},
		{"ssh blank credential", sshProvider{}, map[string]any{"user": "root", "host": "localhost", "port": 22, "credential_ref": " "}, "credential_ref"},
		{"salt minion type", saltProvider{}, map[string]any{"minion_id": 123}, "minion_id"},
	}
	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			issues := test.provider.Validate(&Link{CanonicalID: "project.test/link.example", ProviderData: test.data})
			if len(issues) == 0 || !strings.Contains(strings.Join(issues, "\n"), "provider_data."+test.field) {
				t.Fatalf("expected an issue for %s, got %v", test.field, issues)
			}
		})
	}
}

func TestProviderValidationEnforcesPortBoundaries(t *testing.T) {
	providers := []struct {
		name     string
		provider Provider
		portKey  string
		data     map[string]any
	}{
		{"frp-stcp", frpProvider{}, "local_port", map[string]any{"config": "frpc.toml", "local_host": "localhost"}},
		{"ssh", sshProvider{}, "port", map[string]any{"user": "root", "host": "localhost"}},
	}
	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			for _, port := range []any{1, 65535, float64(1), float64(65535)} {
				provider.data[provider.portKey] = port
				if issues := provider.provider.Validate(&Link{CanonicalID: "project.test/link.example", ProviderData: provider.data}); len(issues) != 0 {
					t.Fatalf("port %v should be valid, got %v", port, issues)
				}
			}
			for _, port := range []any{0, 65536, 22.5, "22", true} {
				provider.data[provider.portKey] = port
				issues := provider.provider.Validate(&Link{CanonicalID: "project.test/link.example", ProviderData: provider.data})
				if len(issues) == 0 || !strings.Contains(strings.Join(issues, "\n"), "provider_data."+provider.portKey) {
					t.Fatalf("port %v should be rejected, got %v", port, issues)
				}
			}
		})
	}
}

func TestInvalidProviderProbeDoesNotDialOrExecute(t *testing.T) {
	helperDirectory := installProviderHelpers(t, "frpc", "ssh", "salt")
	t.Setenv("PATH", helperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	counterPath := filepath.Join(helperDirectory, "invocations")
	t.Setenv(providerHelperCounter, counterPath)

	frpObservation := (frpProvider{}).Probe(context.Background(), &Link{
		CanonicalID:  "project.test/link.frp",
		ProviderData: map[string]any{"config": "frpc.toml", "local_host": "127.0.0.1", "local_port": 0},
	}, RuntimeContext{})
	assertDeclarationFailure(t, frpObservation, "frpc-config-and-tcp-connect")

	saltObservation := (saltProvider{}).Probe(context.Background(), &Link{
		CanonicalID:  "project.test/link.salt",
		ProviderData: map[string]any{"minion_id": " "},
	}, RuntimeContext{})
	assertDeclarationFailure(t, saltObservation, "salt-test-ping")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan bool, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
			accepted <- true
			return
		}
		accepted <- false
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	sshObservation := (sshProvider{}).Probe(context.Background(), &Link{
		CanonicalID:  "project.test/link.ssh",
		ProviderData: map[string]any{"user": "", "host": "127.0.0.1", "port": port},
	}, RuntimeContext{})
	assertDeclarationFailure(t, sshObservation, "tcp-connect-and-ssh-config")
	_ = listener.Close()
	if <-accepted {
		t.Fatal("invalid ssh declaration reached the TCP dial")
	}

	if invocations, err := os.ReadFile(counterPath); err == nil {
		t.Fatalf("invalid declarations executed provider processes: %s", invocations)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestProbeDoesNotRetainExternalDiagnostics(t *testing.T) {
	helperDirectory := installProviderHelpers(t, "salt")
	t.Setenv("PATH", helperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(providerHelperCounter, filepath.Join(helperDirectory, "invocations"))
	t.Setenv(providerHelperOutput, strings.Join([]string{
		"Authorization: Bearer bearer-123",
		"AWS_SECRET_ACCESS_KEY=aws-secret-456",
		"-----BEGIN PRIVATE KEY-----",
		"bare-secret-789",
		"stdout: connected host=node.internal",
	}, "\n"))

	observation := (saltProvider{}).Probe(context.Background(), &Link{
		CanonicalID:  "project.test/link.salt",
		ProviderData: map[string]any{"minion_id": "minion-1"},
	}, RuntimeContext{})
	if observation.Status != "failure" {
		t.Fatalf("expected helper failure, got %#v", observation)
	}
	for _, output := range []string{"bearer-123", "aws-secret-456", "PRIVATE KEY", "bare-secret-789", "node.internal"} {
		if strings.Contains(observation.Error, output) {
			t.Fatalf("diagnostic retained provider output %q: %s", output, observation.Error)
		}
	}
	if observation.Error != "salt test.ping failed with exit code 1" {
		t.Fatalf("unexpected safe diagnostic summary: %q", observation.Error)
	}
	if observation.Evidence["kind"] != "salt-test-ping" {
		t.Fatalf("unexpected evidence: %#v", observation.Evidence)
	}
}

func assertDeclarationFailure(t *testing.T, observation Observation, evidenceKind string) {
	t.Helper()
	if observation.Status != "failure" || !strings.Contains(observation.Error, "invalid provider declaration") {
		t.Fatalf("expected declaration failure, got %#v", observation)
	}
	if observation.Evidence["kind"] != evidenceKind {
		t.Fatalf("expected evidence kind %q, got %#v", evidenceKind, observation.Evidence)
	}
}

func installProviderHelpers(t *testing.T, names ...string) string {
	t.Helper()
	directory := workspaceTestPath(t, "provider-helpers", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		targetName := name
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
		target := filepath.Join(directory, targetName)
		source, err := os.Open(executable)
		if err != nil {
			t.Fatal(err)
		}
		destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			_ = source.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(destination, source)
		closeDestinationErr := destination.Close()
		closeSourceErr := source.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		if closeDestinationErr != nil {
			t.Fatal(closeDestinationErr)
		}
		if closeSourceErr != nil {
			t.Fatal(closeSourceErr)
		}
	}
	return directory
}
