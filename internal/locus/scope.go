package locus

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

const APIVersion = "locus/v1"

const LocusHomeRegistryURI = "${LOCUS_HOME}/registry"

type Source struct {
	Kind     string `yaml:"kind" json:"kind"`
	URI      string `yaml:"uri" json:"uri"`
	Revision string `yaml:"revision,omitempty" json:"revision,omitempty"`
	Path     string `yaml:"path,omitempty" json:"path,omitempty"`
}

type Import struct {
	Alias           string `yaml:"-" json:"alias"`
	ExpectedScopeID string `yaml:"scope_id,omitempty" json:"scope_id,omitempty"`
	Source          Source `yaml:"source" json:"source"`
}

type Manifest struct {
	APIVersion string            `yaml:"api_version" json:"api_version"`
	ScopeID    string            `yaml:"scope_id" json:"scope_id"`
	Imports    map[string]Import `yaml:"-" json:"imports,omitempty"`
	Bindings   map[string]string `yaml:"bindings,omitempty" json:"bindings,omitempty"`
}

type manifestWire struct {
	APIVersion string               `yaml:"api_version"`
	ScopeID    string               `yaml:"scope_id"`
	Imports    map[string]yaml.Node `yaml:"imports,omitempty"`
	Bindings   map[string]string    `yaml:"bindings,omitempty"`
}

type importWire struct {
	ScopeID string `yaml:"scope_id,omitempty"`
	Source  Source `yaml:"source"`
}

type manifestOutput struct {
	APIVersion string                `yaml:"api_version"`
	ScopeID    string                `yaml:"scope_id"`
	Imports    map[string]importWire `yaml:"imports,omitempty"`
	Bindings   map[string]string     `yaml:"bindings,omitempty"`
}

func (m Manifest) MarshalYAML() (any, error) {
	output := manifestOutput{
		APIVersion: m.APIVersion, ScopeID: m.ScopeID,
		Imports: make(map[string]importWire, len(m.Imports)), Bindings: m.Bindings,
	}
	for alias, imported := range m.Imports {
		output.Imports[alias] = importWire{ScopeID: imported.ExpectedScopeID, Source: imported.Source}
	}
	return output, nil
}

func ValidIdentifier(value string) bool {
	return validDeclarationName(value)
}

func readManifest(path string) (Manifest, error) {
	var wire manifestWire
	if err := decodeYAML(path, &wire); err != nil {
		return Manifest{}, err
	}
	if wire.APIVersion != APIVersion {
		return Manifest{}, fmt.Errorf("%s: api_version must be %s", path, APIVersion)
	}
	if !validDeclarationName(wire.ScopeID) {
		return Manifest{}, fmt.Errorf("%s: invalid scope_id %q", path, wire.ScopeID)
	}
	manifest := Manifest{
		APIVersion: wire.APIVersion,
		ScopeID:    wire.ScopeID,
		Imports:    make(map[string]Import, len(wire.Imports)),
		Bindings:   wire.Bindings,
	}
	for alias, node := range wire.Imports {
		if !validDeclarationName(alias) {
			return Manifest{}, fmt.Errorf("%s: invalid import alias %q", path, alias)
		}
		value, err := decodeImport(path, alias, &node)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Imports[alias] = value
	}
	if manifest.Bindings == nil {
		manifest.Bindings = map[string]string{}
	}
	for role, target := range manifest.Bindings {
		if !validDeclarationName(role) {
			return Manifest{}, fmt.Errorf("%s: invalid binding role %q", path, role)
		}
		if strings.TrimSpace(target) == "" {
			return Manifest{}, fmt.Errorf("%s: binding %s target is required", path, role)
		}
	}
	return manifest, nil
}

func decodeImport(path, alias string, node *yaml.Node) (Import, error) {
	value := Import{Alias: alias}
	switch node.Kind {
	case yaml.ScalarNode:
		var scalar string
		if err := node.Load(&scalar); err != nil {
			return Import{}, fmt.Errorf("%s: import %s: %w", path, alias, err)
		}
		source, err := normalizeScalarSource(scalar)
		if err != nil {
			return Import{}, fmt.Errorf("%s: import %s: %w", path, alias, err)
		}
		value.Source = source
	case yaml.MappingNode:
		var wire importWire
		if err := node.Load(&wire, yaml.WithKnownFields()); err != nil {
			return Import{}, fmt.Errorf("%s: import %s: %w", path, alias, err)
		}
		value.ExpectedScopeID = wire.ScopeID
		value.Source = wire.Source
		if value.ExpectedScopeID != "" && !validDeclarationName(value.ExpectedScopeID) {
			return Import{}, fmt.Errorf("%s: import %s: invalid expected scope_id %q", path, alias, value.ExpectedScopeID)
		}
		if err := validateSource(value.Source); err != nil {
			return Import{}, fmt.Errorf("%s: import %s: %w", path, alias, err)
		}
	default:
		return Import{}, fmt.Errorf("%s: import %s must be a scalar or mapping", path, alias)
	}
	return value, nil
}

func normalizeScalarSource(value string) (Source, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return Source{}, errors.New("source URI is required and must be trimmed")
	}
	source := Source{Kind: "directory", URI: value}
	switch {
	case strings.HasPrefix(value, "git+https://"), strings.HasPrefix(value, "git+ssh://"), strings.HasPrefix(value, "git+file://"):
		source.Kind = "git"
		source.URI = strings.TrimPrefix(value, "git+")
	case strings.HasPrefix(value, "http://"), strings.HasPrefix(value, "https://"):
		source.Kind = "url"
	}
	if err := validateSource(source); err != nil {
		return Source{}, err
	}
	return source, nil
}

func validateSource(source Source) error {
	if source.URI == "" || source.URI != strings.TrimSpace(source.URI) {
		return errors.New("source.uri is required and must be trimmed")
	}
	if strings.Contains(source.URI, "${") && source.URI != LocusHomeRegistryURI {
		return fmt.Errorf("unsupported source variable in %q", source.URI)
	}
	switch source.Kind {
	case "directory":
		if source.Revision != "" {
			return errors.New("directory source does not accept revision")
		}
	case "git":
		parsed, err := url.Parse(source.URI)
		if err != nil || parsed.Scheme == "" || (parsed.Scheme != "https" && parsed.Scheme != "ssh" && parsed.Scheme != "file") {
			return errors.New("git source URI must use https, ssh, or file")
		}
		if parsed.User != nil {
			return errors.New("source URI userinfo is not allowed")
		}
	case "url":
		parsed, err := url.Parse(source.URI)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("URL source URI must use http or https")
		}
		if parsed.User != nil {
			return errors.New("source URI userinfo is not allowed")
		}
		if source.Revision != "" {
			return errors.New("URL source does not accept revision")
		}
	default:
		return fmt.Errorf("unsupported source kind %q", source.Kind)
	}
	if source.Path != "" {
		clean := filepath.Clean(filepath.FromSlash(source.Path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
			return errors.New("source.path must stay within the source root")
		}
	}
	return nil
}

func resolveDirectorySource(source Source, importingRoot, home string) (string, error) {
	if source.Kind != "directory" {
		return "", fmt.Errorf("source kind %s is not a directory", source.Kind)
	}
	uri := source.URI
	if uri == LocusHomeRegistryURI {
		if home == "" {
			return "", errors.New("LOCUS_HOME is unavailable")
		}
		uri = filepath.Join(home, "registry")
	}
	if strings.Contains(uri, "${") {
		return "", fmt.Errorf("unsupported source variable in %q", source.URI)
	}
	if !filepath.IsAbs(uri) {
		uri = filepath.Join(importingRoot, filepath.FromSlash(uri))
	}
	absolute, err := filepath.Abs(uri)
	if err != nil {
		return "", err
	}
	if source.Path != "" {
		candidate := filepath.Join(absolute, filepath.FromSlash(source.Path))
		if !pathContainedBy(absolute, candidate) {
			return "", errors.New("source.path escapes source root")
		}
		absolute = candidate
	}
	return filepath.Clean(absolute), nil
}

func sanitizeSource(source Source) Source {
	clean := source
	if parsed, err := url.Parse(clean.URI); err == nil && parsed.Scheme != "" {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.Fragment = ""
		clean.URI = parsed.String()
	}
	return clean
}

func sourceDigest(source Source) string {
	digest, _ := digestValue(source)
	return digest
}
