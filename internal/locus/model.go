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

type Scope struct {
	ID   string `yaml:"id" json:"id"`
	Kind string `yaml:"kind" json:"kind"`
}

type Import struct {
	Alias string `yaml:"alias" json:"alias"`
	Path  string `yaml:"path" json:"path"`
}

type Manifest struct {
	APIVersion string            `yaml:"api_version" json:"api_version"`
	Scope      Scope             `yaml:"scope" json:"scope"`
	Imports    []Import          `yaml:"imports,omitempty" json:"imports,omitempty"`
	Bindings   map[string]string `yaml:"bindings,omitempty" json:"bindings,omitempty"`
}

type Documentation struct {
	Ref   string `yaml:"ref" json:"ref"`
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
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

type Registry struct {
	Root     string
	Manifest Manifest
	Scopes   map[string]Manifest
	Aliases  map[string]string
	Bindings map[string]string
	Entities map[string]*Entity
	Links    map[string]*Link
	Routes   map[string]*Route
	local    map[string]map[string]string
	sources  map[string]string
}

type rawType struct {
	Type string `yaml:"type"`
}

func DiscoverRegistry(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(current, ".locus", "registry")
		if _, err := os.Stat(filepath.Join(candidate, "scope.yaml")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no .locus/registry found from %s", start)
		}
		current = parent
	}
}

func LoadRegistry(root string) (*Registry, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		Root: absolute, Scopes: map[string]Manifest{}, Aliases: map[string]string{}, Bindings: map[string]string{},
		Entities: map[string]*Entity{}, Links: map[string]*Link{}, Routes: map[string]*Route{},
		local: map[string]map[string]string{}, sources: map[string]string{},
	}
	manifest, err := readManifest(filepath.Join(absolute, "scope.yaml"))
	if err != nil {
		return nil, err
	}
	registry.Manifest = manifest
	if err := registry.loadScope(absolute, manifest); err != nil {
		return nil, err
	}
	if err := validateEnvironmentManifest(manifest); err != nil {
		return nil, err
	}
	for _, imported := range manifest.Imports {
		if imported.Alias == "" || imported.Path == "" {
			return nil, errors.New("import alias and path are required")
		}
		if !validDeclarationName(imported.Alias) {
			return nil, fmt.Errorf("invalid import alias %q", imported.Alias)
		}
		if _, exists := registry.Aliases[imported.Alias]; exists {
			return nil, fmt.Errorf("duplicate import alias %q", imported.Alias)
		}
		path := imported.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(absolute, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		importManifest, err := readManifest(filepath.Join(path, "scope.yaml"))
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", imported.Alias, err)
		}
		if importManifest.Scope.Kind != "environment" {
			return nil, fmt.Errorf("import %s is not an environment scope", imported.Alias)
		}
		if err := validateEnvironmentManifest(importManifest); err != nil {
			return nil, fmt.Errorf("import %s: %w", imported.Alias, err)
		}
		registry.Aliases[imported.Alias] = importManifest.Scope.ID
		if err := registry.loadScope(path, importManifest); err != nil {
			return nil, err
		}
	}
	roles := make([]string, 0, len(manifest.Bindings))
	for role := range manifest.Bindings {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		if !validDeclarationName(role) {
			return nil, fmt.Errorf("invalid binding role %q", role)
		}
		canonical, err := registry.resolveRef(manifest.Scope.ID, manifest.Bindings[role], "entity")
		if err != nil {
			return nil, fmt.Errorf("binding %s: %w", role, err)
		}
		registry.Bindings[role] = canonical
	}
	if issues := registry.Validate(); len(issues) > 0 {
		return nil, errors.New(strings.Join(issues, "; "))
	}
	return registry, nil
}

func readManifest(path string) (Manifest, error) {
	var manifest Manifest
	if err := decodeYAML(path, &manifest); err != nil {
		return manifest, err
	}
	if manifest.APIVersion != "locus/v0" {
		return manifest, fmt.Errorf("%s: api_version must be locus/v0", path)
	}
	if manifest.Scope.ID == "" || !validDeclarationName(manifest.Scope.ID) ||
		(manifest.Scope.Kind != "project" && manifest.Scope.Kind != "environment") {
		return manifest, fmt.Errorf("%s: invalid scope", path)
	}
	return manifest, nil
}

func validateEnvironmentManifest(manifest Manifest) error {
	if manifest.Scope.Kind != "environment" {
		return nil
	}
	if len(manifest.Imports) > 0 {
		return errors.New("environment imports are not supported in v0")
	}
	if len(manifest.Bindings) > 0 {
		return errors.New("environment bindings are not supported in v0")
	}
	return nil
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

func (r *Registry) loadScope(root string, manifest Manifest) error {
	if _, exists := r.Scopes[manifest.Scope.ID]; exists {
		return fmt.Errorf("duplicate scope id %q", manifest.Scope.ID)
	}
	r.Scopes[manifest.Scope.ID] = manifest
	r.local[manifest.Scope.ID] = map[string]string{}
	for _, dir := range []string{"entities", "links", "routes"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") {
				continue
			}
			if err := r.loadObject(filepath.Join(root, dir, entry.Name()), manifest.Scope.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) loadObject(path, scopeID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	loader, err := yaml.NewLoader(bytes.NewReader(data), yaml.WithUniqueKeys())
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var node yaml.Node
	if err := loader.Load(&node); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var extra yaml.Node
	if err := loader.Load(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%s: multiple YAML documents are not supported", path)
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	var header rawType
	if err := node.Load(&header); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	switch header.Type {
	case "entity":
		var value Entity
		if err := decodeYAMLNode(path, &node, &value); err != nil {
			return err
		}
		value.ScopeID, value.CanonicalID = scopeID, canonical(scopeID, value.ID)
		if err := r.reserve(scopeID, value.ID, value.CanonicalID); err != nil {
			return err
		}
		r.Entities[value.CanonicalID] = &value
		r.sources[value.CanonicalID] = path
	case "link":
		var value Link
		if err := decodeYAMLNode(path, &node, &value); err != nil {
			return err
		}
		value.ScopeID, value.CanonicalID = scopeID, canonical(scopeID, value.ID)
		if err := r.reserve(scopeID, value.ID, value.CanonicalID); err != nil {
			return err
		}
		r.Links[value.CanonicalID] = &value
		r.sources[value.CanonicalID] = path
	case "route":
		var value Route
		if err := decodeYAMLNode(path, &node, &value); err != nil {
			return err
		}
		value.ScopeID, value.CanonicalID = scopeID, canonical(scopeID, value.ID)
		if err := r.reserve(scopeID, value.ID, value.CanonicalID); err != nil {
			return err
		}
		r.Routes[value.CanonicalID] = &value
		r.sources[value.CanonicalID] = path
	default:
		return fmt.Errorf("%s: unsupported type %q", path, header.Type)
	}
	return nil
}

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

func decodeYAMLNode(path string, node *yaml.Node, target any) error {
	if err := node.Load(target, yaml.WithKnownFields()); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (r *Registry) reserve(scopeID, localID, canonicalID string) error {
	if localID == "" {
		return errors.New("object id is required")
	}
	if _, exists := r.local[scopeID][localID]; exists {
		return fmt.Errorf("duplicate local id %s in scope %s", localID, scopeID)
	}
	r.local[scopeID][localID] = canonicalID
	return nil
}

func canonical(scopeID, localID string) string { return scopeID + "::" + localID }

func (r *Registry) resolveRef(scopeID, ref, expected string) (string, error) {
	candidate := ref
	if strings.Contains(ref, "::") {
		parts := strings.SplitN(ref, "::", 2)
		if scope, ok := r.Aliases[parts[0]]; ok {
			candidate = canonical(scope, parts[1])
		}
	} else {
		candidate = canonical(scopeID, ref)
	}
	if !r.exists(candidate, expected) {
		return "", fmt.Errorf("unknown %s reference %q", expected, ref)
	}
	return candidate, nil
}

func (r *Registry) exists(id, kind string) bool {
	switch kind {
	case "entity":
		_, ok := r.Entities[id]
		return ok
	case "link":
		_, ok := r.Links[id]
		return ok
	case "route":
		_, ok := r.Routes[id]
		return ok
	}
	return false
}

func (r *Registry) Validate() []string {
	var issues []string
	for scopeID, manifest := range r.Scopes {
		if manifest.APIVersion != "locus/v0" {
			issues = append(issues, fmt.Sprintf("%s: api_version must be locus/v0", scopeID))
		}
		if !validDeclarationName(manifest.Scope.ID) {
			issues = append(issues, fmt.Sprintf("%s: invalid scope id %q", scopeID, manifest.Scope.ID))
		}
		if err := validateEnvironmentManifest(manifest); err != nil {
			issues = append(issues, scopeID+": "+err.Error())
		}
		for _, imported := range manifest.Imports {
			if !validDeclarationName(imported.Alias) {
				issues = append(issues, fmt.Sprintf("%s: invalid import alias %q", scopeID, imported.Alias))
			}
			if imported.Path == "" {
				issues = append(issues, scopeID+": import path is required")
			}
		}
		for role := range manifest.Bindings {
			if !validDeclarationName(role) {
				issues = append(issues, fmt.Sprintf("%s: invalid binding role %q", scopeID, role))
			}
		}
	}
	for _, entity := range r.Entities {
		issues = append(issues, r.validateObject(entity.CanonicalID, entity.APIVersion, entity.ID, entity.Documentation)...)
	}
	providers := NewProviders()
	for _, link := range r.Links {
		issues = append(issues, r.validateObject(link.CanonicalID, link.APIVersion, link.ID, link.Documentation)...)
		from, err := r.resolveRef(link.ScopeID, link.From, "entity")
		if err != nil {
			issues = append(issues, link.CanonicalID+": "+err.Error())
		} else {
			link.From = from
		}
		to, err := r.resolveRef(link.ScopeID, link.To, "entity")
		if err != nil {
			issues = append(issues, link.CanonicalID+": "+err.Error())
		} else {
			link.To = to
		}
		if link.Provider == "" {
			issues = append(issues, link.CanonicalID+": provider is required")
		} else if provider, ok := providers.Get(link.Provider); !ok {
			issues = append(issues, fmt.Sprintf("%s: unsupported provider %s", link.CanonicalID, link.Provider))
		} else {
			issues = append(issues, provider.Validate(link)...)
		}
	}
	for _, route := range r.Routes {
		issues = append(issues, r.validateObject(route.CanonicalID, route.APIVersion, route.ID, route.Documentation)...)
		available := map[string]bool{}
		for index := range route.Steps {
			linkID, err := r.resolveRef(route.ScopeID, route.Steps[index].Link, "link")
			if err != nil {
				issues = append(issues, route.CanonicalID+": "+err.Error())
				continue
			}
			route.Steps[index].Link = linkID
			link := r.Links[linkID]
			for _, required := range link.Requires {
				if !available[required] {
					issues = append(issues, fmt.Sprintf("%s: step %s requires unavailable capability %s", route.CanonicalID, linkID, required))
				}
			}
			for _, provided := range link.Provides {
				available[provided] = true
			}
		}
		if len(route.Steps) == 0 {
			issues = append(issues, route.CanonicalID+": route requires at least one step")
		}
	}
	sort.Strings(issues)
	return issues
}

func (r *Registry) validateObject(canonicalID, apiVersion, localID string, documentation []Documentation) []string {
	var issues []string
	if apiVersion != "locus/v0" {
		issues = append(issues, canonicalID+": api_version must be locus/v0")
	}
	if !validDeclarationName(localID) {
		issues = append(issues, fmt.Sprintf("%s: invalid local id %q", canonicalID, localID))
	}
	source := r.sources[canonicalID]
	for index, document := range documentation {
		ref := document.Ref
		path, _, _ := strings.Cut(ref, "#")
		if ref == "" || ref != strings.TrimSpace(ref) || path == "" {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref is invalid", canonicalID, index))
			continue
		}
		documentPath := filepath.FromSlash(path)
		if filepath.IsAbs(documentPath) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref must be relative", canonicalID, index))
			continue
		}
		if source == "" {
			continue
		}
		scopeRoot := filepath.Dir(filepath.Dir(source))
		docsRoot := filepath.Join(scopeRoot, "docs")
		documentPath = filepath.Clean(filepath.Join(filepath.Dir(source), documentPath))
		if !pathContainedBy(docsRoot, documentPath) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q must stay within the scope docs directory", canonicalID, index, ref))
			continue
		}
		info, err := os.Stat(documentPath)
		if err != nil || info.IsDir() {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q does not reference a file", canonicalID, index, ref))
			continue
		}
		resolvedScopeRoot, rootErr := filepath.EvalSymlinks(scopeRoot)
		resolvedDocsRoot, docsErr := filepath.EvalSymlinks(docsRoot)
		resolvedDocument, documentErr := filepath.EvalSymlinks(documentPath)
		if rootErr != nil || docsErr != nil || documentErr != nil ||
			!pathContainedBy(resolvedScopeRoot, resolvedDocsRoot) ||
			!pathContainedBy(resolvedDocsRoot, resolvedDocument) {
			issues = append(issues, fmt.Sprintf("%s: documentation[%d].ref %q resolves outside the scope docs directory", canonicalID, index, ref))
		}
	}
	return issues
}

func pathContainedBy(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (r *Registry) ResolveEntity(ref string) (string, error) {
	if binding, ok := r.Bindings[ref]; ok {
		return binding, nil
	}
	return r.resolveRef(r.Manifest.Scope.ID, ref, "entity")
}

func (r *Registry) ResolveAny(ref string) (string, string, error) {
	if binding, ok := r.Bindings[ref]; ok {
		return binding, "entity", nil
	}
	for _, kind := range []string{"entity", "link", "route"} {
		if value, err := r.resolveRef(r.Manifest.Scope.ID, ref, kind); err == nil {
			return value, kind, nil
		}
	}
	return "", "", fmt.Errorf("unknown object reference %q", ref)
}

func (r *Registry) ObjectIDs(kind string) []string {
	var ids []string
	if kind == "" || kind == "entity" {
		for id := range r.Entities {
			ids = append(ids, id)
		}
	}
	if kind == "" || kind == "link" {
		for id := range r.Links {
			ids = append(ids, id)
		}
	}
	if kind == "" || kind == "route" {
		for id := range r.Routes {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
