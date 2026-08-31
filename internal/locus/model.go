package locus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

type Documentation struct {
	Ref   string `yaml:"ref" json:"ref"`
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
}

type Binding struct {
	ID          string `json:"id"`
	CanonicalID string `json:"canonical_id"`
	ScopeID     string `json:"scope_id"`
	Target      string `json:"target"`
}

type Entity struct {
	APIVersion    string            `yaml:"api_version" json:"api_version"`
	Type          string            `yaml:"type" json:"type"`
	ID            string            `yaml:"id" json:"id"`
	CanonicalID   string            `yaml:"-" json:"canonical_id"`
	ScopeID       string            `yaml:"-" json:"scope_id"`
	Kind          string            `yaml:"kind" json:"kind"`
	Name          string            `yaml:"name" json:"name"`
	Labels        map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Documentation []Documentation   `yaml:"documentation,omitempty" json:"documentation,omitempty"`
}

type Link struct {
	APIVersion    string          `yaml:"api_version" json:"api_version"`
	Type          string          `yaml:"type" json:"type"`
	ID            string          `yaml:"id" json:"id"`
	CanonicalID   string          `yaml:"-" json:"canonical_id"`
	ScopeID       string          `yaml:"-" json:"scope_id"`
	From          string          `yaml:"from" json:"from"`
	To            string          `yaml:"to" json:"to"`
	Provider      string          `yaml:"provider" json:"provider"`
	Requires      []string        `yaml:"requires,omitempty" json:"requires,omitempty"`
	Provides      []string        `yaml:"provides,omitempty" json:"provides,omitempty"`
	ProviderData  map[string]any  `yaml:"provider_data,omitempty" json:"provider_data,omitempty"`
	Documentation []Documentation `yaml:"documentation,omitempty" json:"documentation,omitempty"`
}

type RouteStep struct {
	Link string `yaml:"link" json:"link"`
}

type Route struct {
	APIVersion    string          `yaml:"api_version" json:"api_version"`
	Type          string          `yaml:"type" json:"type"`
	ID            string          `yaml:"id" json:"id"`
	CanonicalID   string          `yaml:"-" json:"canonical_id"`
	ScopeID       string          `yaml:"-" json:"scope_id"`
	Steps         []RouteStep     `yaml:"steps" json:"steps"`
	Documentation []Documentation `yaml:"documentation,omitempty" json:"documentation,omitempty"`
}

type rawType struct {
	Type string `yaml:"type"`
}

func validDeclarationName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func canonical(scopeID, localID string) string { return scopeID + "::" + localID }

func decodeYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Load(data, target, yaml.WithKnownFields(), yaml.WithSingleDocument(), yaml.WithUniqueKeys()); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func decodeObjectNode(path string) (yaml.Node, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return yaml.Node{}, nil, err
	}
	loader, err := yaml.NewLoader(bytes.NewReader(data), yaml.WithUniqueKeys())
	if err != nil {
		return yaml.Node{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	var node yaml.Node
	if err := loader.Load(&node); err != nil {
		return yaml.Node{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	var extra yaml.Node
	if err := loader.Load(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return yaml.Node{}, nil, fmt.Errorf("%s: multiple YAML documents are not supported", path)
		}
		return yaml.Node{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	return node, data, nil
}

func decodeYAMLNode(path string, node *yaml.Node, target any) error {
	if err := node.Load(target, yaml.WithKnownFields()); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func validateObject(canonicalID, apiVersion, localID string) []string {
	var issues []string
	if apiVersion != APIVersion {
		issues = append(issues, canonicalID+": api_version must be "+APIVersion)
	}
	if !validDeclarationName(localID) {
		issues = append(issues, fmt.Sprintf("%s: invalid local id %q", canonicalID, localID))
	}
	return issues
}

func validateDocumentation(root, source, canonicalID string, values []Documentation, remote bool) []string {
	var issues []string
	docsRoot := filepath.Join(root, "docs")
	for index, document := range values {
		path, _, _ := strings.Cut(document.Ref, "#")
		if document.Ref == "" || document.Ref != strings.TrimSpace(document.Ref) || path == "" {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref is invalid", canonicalID, index))
			continue
		}
		documentPath := filepath.FromSlash(path)
		if filepath.IsAbs(documentPath) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref must be relative", canonicalID, index))
			continue
		}
		documentPath = filepath.Clean(filepath.Join(filepath.Dir(source), documentPath))
		if !pathContainedBy(docsRoot, documentPath) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q must stay within the scope docs directory", canonicalID, index, document.Ref))
			continue
		}
		info, err := os.Lstat(documentPath)
		if err != nil || info.IsDir() {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q does not reference a file", canonicalID, index, document.Ref))
			continue
		}
		if remote && info.Mode()&os.ModeSymlink != 0 {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q is a remote symlink", canonicalID, index, document.Ref))
			continue
		}
		resolvedDocsRoot, docsErr := filepath.EvalSymlinks(docsRoot)
		resolvedDocument, documentErr := filepath.EvalSymlinks(documentPath)
		if docsErr != nil || documentErr != nil || !pathContainedBy(resolvedDocsRoot, resolvedDocument) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q resolves outside the scope docs directory", canonicalID, index, document.Ref))
		}
	}
	sort.Strings(issues)
	return issues
}

func pathContainedBy(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
