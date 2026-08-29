package locus

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"
)

type RuntimeContext struct {
	CWD            string   `json:"cwd"`
	CurrentEntity  string   `json:"current_entity"`
	AvailableTools []string `json:"available_tools"`
	Vantage        string   `json:"vantage"`
}

type NativeHint struct {
	Executable     string   `json:"executable"`
	Args           []string `json:"args"`
	CredentialRefs []string `json:"credential_refs,omitempty"`
}

type Provider interface {
	Name() string
	Validate(link *Link) []string
	Render(link *Link, runtime RuntimeContext) (NativeHint, error)
	Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation
}

type Providers struct{ values map[string]Provider }

func NewProviders() *Providers {
	values := []Provider{frpProvider{}, sshProvider{}}
	registry := &Providers{values: map[string]Provider{}}
	for _, value := range values {
		registry.values[value.Name()] = value
	}
	return registry
}

func (p *Providers) Get(name string) (Provider, bool) { value, ok := p.values[name]; return value, ok }

func (p *Providers) Available() []string {
	var available []string
	for _, executable := range []string{"frpc", "ssh"} {
		if _, err := exec.LookPath(executable); err == nil {
			available = append(available, executable)
		}
	}
	return available
}

type frpProvider struct{}

func (frpProvider) Name() string { return "frp-stcp" }
func (frpProvider) Validate(link *Link) []string {
	return requireProviderFields(link, "config", "local_host", "local_port")
}
func (frpProvider) Render(link *Link, _ RuntimeContext) (NativeHint, error) {
	config, err := providerString(link, "config")
	if err != nil {
		return NativeHint{}, err
	}
	return NativeHint{Executable: "frpc", Args: []string{"-c", config}}, nil
}
func (frpProvider) Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation {
	observation := newObservation(link, runtime, "frp-stcp")
	config, err := providerString(link, "config")
	if err == nil {
		command := exec.CommandContext(ctx, "frpc", "verify", "-c", config)
		if output, commandErr := command.CombinedOutput(); commandErr != nil {
			err = fmt.Errorf("frpc verify: %v: %s", commandErr, sanitizeOutput(output))
		}
	}
	if err == nil {
		err = dialProviderEndpoint(ctx, link)
	}
	return finishObservation(observation, err)
}

type sshProvider struct{}

func (sshProvider) Name() string { return "ssh" }
func (sshProvider) Validate(link *Link) []string {
	return requireProviderFields(link, "user", "host", "port")
}
func (sshProvider) Render(link *Link, _ RuntimeContext) (NativeHint, error) {
	user, err := providerString(link, "user")
	if err != nil {
		return NativeHint{}, err
	}
	host, err := providerString(link, "host")
	if err != nil {
		return NativeHint{}, err
	}
	port, err := providerPort(link)
	if err != nil {
		return NativeHint{}, err
	}
	hint := NativeHint{Executable: "ssh", Args: []string{"-p", strconv.Itoa(port), user + "@" + host}}
	if credential, ok := link.ProviderData["credential_ref"].(string); ok && credential != "" {
		hint.CredentialRefs = []string{credential}
	}
	return hint, nil
}
func (sshProvider) Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation {
	observation := newObservation(link, runtime, "ssh")
	err := dialProviderEndpoint(ctx, link)
	if err == nil {
		hint, renderErr := (sshProvider{}).Render(link, runtime)
		if renderErr != nil {
			err = renderErr
		} else {
			args := append([]string{"-G"}, hint.Args...)
			command := exec.CommandContext(ctx, "ssh", args...)
			if output, commandErr := command.CombinedOutput(); commandErr != nil {
				err = fmt.Errorf("ssh config probe: %v: %s", commandErr, sanitizeOutput(output))
			}
		}
	}
	return finishObservation(observation, err)
}

func requireProviderFields(link *Link, fields ...string) []string {
	var issues []string
	for _, field := range fields {
		if _, exists := link.ProviderData[field]; !exists {
			issues = append(issues, fmt.Sprintf("%s: provider_data.%s is required", link.CanonicalID, field))
		}
	}
	return issues
}

func providerString(link *Link, key string) (string, error) {
	value, ok := link.ProviderData[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s: provider_data.%s must be a string", link.CanonicalID, key)
	}
	return value, nil
}

func providerPort(link *Link) (int, error) {
	value, exists := link.ProviderData["port"]
	if !exists {
		value = link.ProviderData["local_port"]
	}
	switch port := value.(type) {
	case int:
		return port, nil
	case uint64:
		return int(port), nil
	case float64:
		return int(port), nil
	default:
		return 0, fmt.Errorf("%s: provider port must be an integer", link.CanonicalID)
	}
}

func dialProviderEndpoint(ctx context.Context, link *Link) error {
	hostKey := "host"
	if _, exists := link.ProviderData[hostKey]; !exists {
		hostKey = "local_host"
	}
	host, err := providerString(link, hostKey)
	if err != nil {
		return err
	}
	port, err := providerPort(link)
	if err != nil {
		return err
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("tcp probe %s:%d: %w", host, port, err)
	}
	return connection.Close()
}

func sanitizeOutput(output []byte) string {
	if len(output) > 256 {
		output = output[:256]
	}
	return string(output)
}
