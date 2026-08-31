package locus

import (
	"context"
	"errors"
	"os"
	"sort"
)

type RootContext struct {
	RootOrigin       string               `json:"root_origin"`
	RegistryPath     string               `json:"registry_path"`
	UserRegistryPath string               `json:"user_registry_path"`
	Registered       bool                 `json:"registered"`
	Registration     *ProjectRegistration `json:"registration,omitempty"`
	HasUserImport    bool                 `json:"has_user_import"`
	SourceCache      []SourceCacheEntry   `json:"source_cache,omitempty"`
}

func LoadActiveRegistry(root string) (*Registry, error) {
	statePath, err := DefaultStatePath()
	if err != nil {
		return nil, err
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	registry, _, err := LoadRegistryContext(root, store)
	return registry, err
}

func LoadRegistryContext(root string, store *Store) (*Registry, RootContext, error) {
	home, err := DefaultHome()
	if err != nil {
		return nil, RootContext{}, err
	}
	layout, err := LocusHomeLayout()
	if err != nil {
		return nil, RootContext{}, err
	}
	origin := "explicit"
	if root == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, RootContext{}, cwdErr
		}
		root, err = DiscoverRegistry(cwd)
		if err == nil {
			origin = "project"
		} else {
			root = layout.Registry
			origin = "user"
		}
	}
	registry, err := CollectRegistry(root, CollectorOptions{Home: home, Store: store})
	if err != nil {
		return nil, RootContext{}, err
	}
	result := RootContext{
		RootOrigin: origin, RegistryPath: registry.Root, UserRegistryPath: layout.Registry,
	}
	for _, imported := range registry.Manifest.Imports {
		if imported.Source.Kind == "directory" && imported.Source.URI == LocusHomeRegistryURI {
			result.HasUserImport = true
			break
		}
	}
	if store != nil {
		registration, registrationErr := store.ProjectRegistration(context.Background(), registry.RootScopeID)
		if registrationErr != nil {
			return nil, RootContext{}, registrationErr
		}
		result.Registration = registration
		result.Registered = registration != nil
		for scopeID, manifest := range registry.Scopes {
			entries, cacheErr := store.SourceCacheEntries(scopeID)
			if cacheErr != nil {
				return nil, RootContext{}, cacheErr
			}
			for index := range entries {
				if imported, exists := manifest.Imports[entries[index].ImportAlias]; exists {
					entries[index].CurrentSourceDigest = sourceDigest(imported.Source)
					entries[index].ConfigurationChanged = entries[index].ConfiguredSourceDigest != entries[index].CurrentSourceDigest
				}
			}
			result.SourceCache = append(result.SourceCache, entries...)
		}
		sort.Slice(result.SourceCache, func(i, j int) bool {
			if result.SourceCache[i].OwnerScopeID != result.SourceCache[j].OwnerScopeID {
				return result.SourceCache[i].OwnerScopeID < result.SourceCache[j].OwnerScopeID
			}
			return result.SourceCache[i].ImportAlias < result.SourceCache[j].ImportAlias
		})
	}
	return registry, result, nil
}

func ObservationVantage(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", errors.New("hostname is unavailable")
	}
	return "host:" + host, nil
}
