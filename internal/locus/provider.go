package locus

import (
	"context"
	"fmt"
	"math"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RuntimeContext struct {
	CWD                     string                      `json:"cwd"`
	CurrentEntity           string                      `json:"current_entity"`
	AvailableTools          []string                    `json:"available_tools"`
	Vantage                 string                      `json:"vantage"`
	MechanismBindings       map[string]MechanismBinding `json:"-"`
	MechanismBindingsSource string                      `json:"mechanism_bindings_source,omitempty"`
}

type NativeHint struct {
	Executable     string   `json:"executable"`
	Args           []string `json:"args"`
	CredentialRefs []string `json:"credential_refs,omitempty"`
}
type ProbeSemantics struct {
	Kind    string
	Version string
}

type Provider interface {
	Name() string
	Executable() string
	ProbeSemantics() ProbeSemantics
	Validate(link *Link) []string
	Render(link *Link, runtime RuntimeContext) (NativeHint, error)
	Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation
}

type Providers struct{ values map[string]Provider }

func NewProviders() *Providers {
	values := []Provider{frpProvider{}, sshProvider{}, saltProvider{}}
	registry := &Providers{values: map[string]Provider{}}
	for _, value := range values {
		registry.values[value.Name()] = value
	}
	return registry
}

func (p *Providers) Get(name string) (Provider, bool) { value, ok := p.values[name]; return value, ok }

func (p *Providers) Available() []string {
	seen := map[string]bool{}
	available := make([]string, 0, len(p.values))
	for _, provider := range p.values {
		executable := provider.Executable()
		if seen[executable] {
			continue
		}
		seen[executable] = true
		if _, err := exec.LookPath(executable); err == nil {
			available = append(available, executable)
		}
	}
	sort.Strings(available)
	return available
}

type frpProvider struct{}

func (frpProvider) Name() string       { return "frp-stcp" }
func (frpProvider) Executable() string { return "frpc" }
func (frpProvider) ProbeSemantics() ProbeSemantics {
	return ProbeSemantics{Kind: "frpc-config-and-tcp-connect", Version: "1"}
}
func (frpProvider) Validate(link *Link) []string {
	var issues []string
	issues = append(issues, validateProviderString(link, "config")...)
	issues = append(issues, validateProviderString(link, "local_host")...)
	issues = append(issues, validateProviderPort(link, "local_port")...)
	return issues
}
func (provider frpProvider) Render(link *Link, runtime RuntimeContext) (NativeHint, error) {
	config, err := providerString(link, "config")
	if err != nil {
		return NativeHint{}, err
	}
	return NativeHint{Executable: runtime.mechanismExecutable(link.CanonicalID, provider.Executable()), Args: []string{"-c", config}}, nil
}
func (provider frpProvider) Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation {
	observation := newObservation(link, runtime, provider.Name(), provider.ProbeSemantics().Kind)
	if issues := provider.Validate(link); len(issues) != 0 {
		return finishObservation(observation, providerValidationError(issues))
	}

	config, err := providerString(link, "config")
	if err == nil {
		command := exec.CommandContext(ctx, runtime.mechanismExecutable(link.CanonicalID, provider.Executable()), "verify", "-c", config)
		if commandErr := command.Run(); commandErr != nil {
			err = summarizeCommandFailure(ctx, "frpc verify", commandErr)
		}
	}
	if err == nil {
		err = dialProviderEndpoint(ctx, link)
	}
	return finishObservation(observation, err)
}

type sshProvider struct{}

func (sshProvider) Name() string       { return "ssh" }
func (sshProvider) Executable() string { return "ssh" }
func (sshProvider) ProbeSemantics() ProbeSemantics {
	return ProbeSemantics{Kind: "tcp-connect-and-ssh-config", Version: "1"}
}
func (sshProvider) Validate(link *Link) []string {
	var issues []string
	issues = append(issues, validateProviderString(link, "user")...)
	issues = append(issues, validateProviderString(link, "host")...)
	issues = append(issues, validateProviderPort(link, "port")...)
	if link != nil && link.ProviderData != nil {
		if _, exists := link.ProviderData["credential_ref"]; exists {
			issues = append(issues, validateProviderString(link, "credential_ref")...)
		}
	}
	return issues
}
func (provider sshProvider) Render(link *Link, runtime RuntimeContext) (NativeHint, error) {
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
	hint := NativeHint{Executable: runtime.mechanismExecutable(link.CanonicalID, provider.Executable()), Args: []string{"-p", strconv.Itoa(port), user + "@" + host}}
	if credential, ok := link.ProviderData["credential_ref"].(string); ok && credential != "" {
		hint.CredentialRefs = []string{credential}
	}
	return hint, nil
}
func (provider sshProvider) Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation {
	observation := newObservation(link, runtime, provider.Name(), provider.ProbeSemantics().Kind)
	if issues := provider.Validate(link); len(issues) != 0 {
		return finishObservation(observation, providerValidationError(issues))
	}

	err := dialProviderEndpoint(ctx, link)
	if err == nil {
		hint, renderErr := provider.Render(link, runtime)
		if renderErr != nil {
			err = renderErr
		} else {
			args := append([]string{"-G"}, hint.Args...)
			command := exec.CommandContext(ctx, hint.Executable, args...)
			if commandErr := command.Run(); commandErr != nil {
				err = summarizeCommandFailure(ctx, "ssh config probe", commandErr)
			}
		}
	}
	return finishObservation(observation, err)
}

type saltProvider struct{}

func (saltProvider) Name() string       { return "salt" }
func (saltProvider) Executable() string { return "salt" }
func (saltProvider) ProbeSemantics() ProbeSemantics {
	return ProbeSemantics{Kind: "salt-test-ping", Version: "1"}
}
func (saltProvider) Validate(link *Link) []string {
	return validateProviderString(link, "minion_id")
}
func (provider saltProvider) Render(link *Link, runtime RuntimeContext) (NativeHint, error) {
	minionID, err := providerString(link, "minion_id")
	if err != nil {
		return NativeHint{}, err
	}
	return NativeHint{Executable: runtime.mechanismExecutable(link.CanonicalID, provider.Executable()), Args: []string{minionID, "test.ping", "--out=json"}}, nil
}
func (provider saltProvider) Probe(ctx context.Context, link *Link, runtime RuntimeContext) Observation {
	observation := newObservation(link, runtime, provider.Name(), provider.ProbeSemantics().Kind)
	if issues := provider.Validate(link); len(issues) != 0 {
		return finishObservation(observation, providerValidationError(issues))
	}

	hint, err := provider.Render(link, runtime)
	if err == nil {
		command := exec.CommandContext(ctx, hint.Executable, hint.Args...)
		if commandErr := command.Run(); commandErr != nil {
			err = summarizeCommandFailure(ctx, "salt test.ping", commandErr)
		}
	}
	return finishObservation(observation, err)
}

func validateProviderString(link *Link, key string) []string {
	subject := providerSubject(link)
	if link == nil || link.ProviderData == nil {
		return []string{fmt.Sprintf("%s: provider_data.%s is required", subject, key)}
	}
	value, exists := link.ProviderData[key]
	if !exists {
		return []string{fmt.Sprintf("%s: provider_data.%s is required", subject, key)}
	}
	text, ok := value.(string)
	if !ok {
		return []string{fmt.Sprintf("%s: provider_data.%s must be a string", subject, key)}
	}
	if strings.TrimSpace(text) == "" {
		return []string{fmt.Sprintf("%s: provider_data.%s must not be empty", subject, key)}
	}
	return nil
}

func validateProviderPort(link *Link, key string) []string {
	if _, err := providerPortField(link, key); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func providerValidationError(issues []string) error {
	return fmt.Errorf("invalid provider declaration: %s", strings.Join(issues, "; "))
}

func providerSubject(link *Link) string {
	if link == nil {
		return "link"
	}
	if link.CanonicalID != "" {
		return link.CanonicalID
	}
	if link.ID != "" {
		return link.ID
	}
	return "link"
}

func providerString(link *Link, key string) (string, error) {
	if issues := validateProviderString(link, key); len(issues) != 0 {
		return "", fmt.Errorf("%s", issues[0])
	}
	return link.ProviderData[key].(string), nil
}

func providerPort(link *Link) (int, error) {
	key := "port"
	if link == nil || link.ProviderData == nil {
		return providerPortField(link, key)
	}
	if _, exists := link.ProviderData[key]; !exists {
		key = "local_port"
	}
	return providerPortField(link, key)
}

func providerPortField(link *Link, key string) (int, error) {
	subject := providerSubject(link)
	if link == nil || link.ProviderData == nil {
		return 0, fmt.Errorf("%s: provider_data.%s is required", subject, key)
	}
	value, exists := link.ProviderData[key]
	if !exists {
		return 0, fmt.Errorf("%s: provider_data.%s is required", subject, key)
	}

	var port int64
	switch value := value.(type) {
	case int:
		port = int64(value)
	case int8:
		port = int64(value)
	case int16:
		port = int64(value)
	case int32:
		port = int64(value)
	case int64:
		port = value
	case uint:
		if uint64(value) > 65535 {
			return 0, fmt.Errorf("%s: provider_data.%s must be between 1 and 65535", subject, key)
		}
		port = int64(value)
	case uint8:
		port = int64(value)
	case uint16:
		port = int64(value)
	case uint32:
		port = int64(value)
	case uint64:
		if value > 65535 {
			return 0, fmt.Errorf("%s: provider_data.%s must be between 1 and 65535", subject, key)
		}
		port = int64(value)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
			return 0, fmt.Errorf("%s: provider_data.%s must be an integer", subject, key)
		}
		if value < 1 || value > 65535 {
			return 0, fmt.Errorf("%s: provider_data.%s must be between 1 and 65535", subject, key)
		}
		port = int64(value)
	default:
		return 0, fmt.Errorf("%s: provider_data.%s must be an integer", subject, key)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s: provider_data.%s must be between 1 and 65535", subject, key)
	}
	return int(port), nil
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

func summarizeCommandFailure(ctx context.Context, operation string, err error) error {
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out", operation)
	}
	if ctx.Err() == context.Canceled {
		return fmt.Errorf("%s canceled", operation)
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s failed with exit code %d", operation, exitError.ExitCode())
	}
	return fmt.Errorf("%s failed to start", operation)
}
