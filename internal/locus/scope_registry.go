package locus

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ScopeRegistry struct {
	Root     string
	Manifest Manifest
	Digest   string
	Remote   bool
	Bindings map[string]*Binding
	Entities map[string]*Entity
	Links    map[string]*Link
	Routes   map[string]*Route
	local    map[string]string
	sources  map[string]string
}

func LoadScopeRegistry(root string, remote bool) (*ScopeRegistry, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := validateRegistrySymlinks(absolute, remote); err != nil {
		return nil, err
	}
	manifest, err := readManifest(filepath.Join(absolute, "scope.yaml"))
	if err != nil {
		return nil, err
	}
	digest, err := registryContentDigest(absolute)
	if err != nil {
		return nil, err
	}
	registry := &ScopeRegistry{
		Root: absolute, Manifest: manifest, Digest: digest, Remote: remote,
		Bindings: map[string]*Binding{}, Entities: map[string]*Entity{}, Links: map[string]*Link{}, Routes: map[string]*Route{},
		local: map[string]string{}, sources: map[string]string{},
	}
	roles := sortedMapKeys(manifest.Bindings)
	for _, role := range roles {
		binding := &Binding{ID: role, CanonicalID: canonical(manifest.ScopeID, role), ScopeID: manifest.ScopeID, Target: manifest.Bindings[role]}
		if err := registry.reserve(role, binding.CanonicalID); err != nil {
			return nil, err
		}
		registry.Bindings[binding.CanonicalID] = binding
	}
	for _, directory := range []string{"entities", "links", "routes"} {
		entries, err := os.ReadDir(filepath.Join(absolute, directory))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") {
				continue
			}
			if err := registry.loadObject(filepath.Join(absolute, directory, entry.Name())); err != nil {
				return nil, err
			}
		}
	}
	if issues := registry.validateLocal(); len(issues) != 0 {
		return nil, errors.New(strings.Join(issues, "; "))
	}
	return registry, nil
}

func (r *ScopeRegistry) reserve(localID, canonicalID string) error {
	if !validDeclarationName(localID) {
		return fmt.Errorf("invalid local id %q", localID)
	}
	if _, exists := r.local[localID]; exists {
		return fmt.Errorf("duplicate local id %s in scope %s", localID, r.Manifest.ScopeID)
	}
	r.local[localID] = canonicalID
	return nil
}

func (r *ScopeRegistry) loadObject(path string) error {
	node, _, err := decodeObjectNode(path)
	if err != nil {
		return err
	}
	var header rawType
	if err := node.Load(&header); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	scopeID := r.Manifest.ScopeID
	switch header.Type {
	case "entity":
		var value Entity
		if err := decodeYAMLNode(path, &node, &value); err != nil {
			return err
		}
		value.ScopeID, value.CanonicalID = scopeID, canonical(scopeID, value.ID)
		if err := r.reserve(value.ID, value.CanonicalID); err != nil {
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
		if err := r.reserve(value.ID, value.CanonicalID); err != nil {
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
		if err := r.reserve(value.ID, value.CanonicalID); err != nil {
			return err
		}
		r.Routes[value.CanonicalID] = &value
		r.sources[value.CanonicalID] = path
	default:
		return fmt.Errorf("%s: unsupported type %q", path, header.Type)
	}
	return nil
}

func (r *ScopeRegistry) validateLocal() []string {
	var issues []string
	for _, entity := range r.Entities {
		issues = append(issues, validateObject(entity.CanonicalID, entity.APIVersion, entity.ID)...)
		if entity.Type != "entity" {
			issues = append(issues, entity.CanonicalID+": type must be entity")
		}
		if entity.Kind == "" || entity.Name == "" {
			issues = append(issues, entity.CanonicalID+": kind and name are required")
		}
		issues = append(issues, validateDocumentation(r.Root, r.sources[entity.CanonicalID], entity.CanonicalID, entity.Documentation, r.Remote)...)
	}
	providers := NewProviders()
	for _, link := range r.Links {
		issues = append(issues, validateObject(link.CanonicalID, link.APIVersion, link.ID)...)
		if link.Type != "link" {
			issues = append(issues, link.CanonicalID+": type must be link")
		}
		if link.From == "" || link.To == "" {
			issues = append(issues, link.CanonicalID+": from and to are required")
		}
		if provider, ok := providers.Get(link.Provider); !ok {
			issues = append(issues, fmt.Sprintf("%s: unsupported provider %s", link.CanonicalID, link.Provider))
		} else if len(link.ProviderData) > 0 {
			issues = append(issues, provider.Validate(link)...)
		}
		issues = append(issues, validateDocumentation(r.Root, r.sources[link.CanonicalID], link.CanonicalID, link.Documentation, r.Remote)...)
	}
	for _, route := range r.Routes {
		issues = append(issues, validateObject(route.CanonicalID, route.APIVersion, route.ID)...)
		if route.Type != "route" {
			issues = append(issues, route.CanonicalID+": type must be route")
		}
		if len(route.Steps) == 0 {
			issues = append(issues, route.CanonicalID+": route requires at least one step")
		}
		for index, step := range route.Steps {
			if step.Link == "" {
				issues = append(issues, fmt.Sprintf("%s: steps[%d].link is required", route.CanonicalID, index))
			}
		}
		issues = append(issues, validateDocumentation(r.Root, r.sources[route.CanonicalID], route.CanonicalID, route.Documentation, r.Remote)...)
	}
	sort.Strings(issues)
	return issues
}

func validateRegistrySymlinks(root string, remote bool) error {
	docsRoot := filepath.Join(root, "docs")
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink == 0 {
			return nil
		}
		if remote {
			return fmt.Errorf("remote registry symlink is not allowed: %s", filepath.ToSlash(relative))
		}
		if !pathContainedBy(docsRoot, path) {
			return fmt.Errorf("registry symlink outside docs is not allowed: %s", filepath.ToSlash(relative))
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !pathContainedBy(docsRoot, resolved) {
			return fmt.Errorf("registry docs symlink escapes docs: %s", filepath.ToSlash(relative))
		}
		return nil
	})
}

func registryContentDigest(root string) (string, error) {
	paths := []string{"scope.yaml"}
	for _, directory := range []string{"entities", "links", "routes", "docs"} {
		base := filepath.Join(root, directory)
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) && path == base {
					return filepath.SkipDir
				}
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	sort.Strings(paths)
	hash := sha256.New()
	var length [8]byte
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return "", err
		}
		binary.BigEndian.PutUint64(length[:], uint64(len(relative)))
		hash.Write(length[:])
		hash.Write([]byte(relative))
		binary.BigEndian.PutUint64(length[:], uint64(info.Size()))
		hash.Write(length[:])
		if _, err := io.Copy(hash, file); err != nil {
			file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
