package locus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type LocusScopeEntry struct {
	ScopeID      string    `json:"scope_id"`
	Kind         string    `json:"kind"`
	RegistryPath string    `json:"registry_path"`
	RegisteredAt time.Time `json:"registered_at,omitempty"`
	Availability string    `json:"availability"`
	Openable     bool      `json:"openable"`
	Active       bool      `json:"active"`
}

func (s *Store) LocusCatalog(ctx context.Context, home, activeScopeID string) ([]LocusScopeEntry, error) {
	values := []LocusScopeEntry{}
	userPath := filepath.Join(home, "registry")
	user := inspectCatalogScope(userPath, "user", "")
	user.Active = user.ScopeID != "" && user.ScopeID == activeScopeID
	values = append(values, user)
	projects, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		value := inspectCatalogScope(project.RegistryPath, "project", project.ScopeID)
		value.RegisteredAt = project.RegisteredAt
		value.Active = value.ScopeID == activeScopeID
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Kind != values[j].Kind {
			return values[i].Kind == "user"
		}
		return values[i].ScopeID < values[j].ScopeID
	})
	return values, nil
}

func (s *Store) OpenableScopePath(ctx context.Context, home, scopeID string) (string, error) {
	values, err := s.LocusCatalog(ctx, home, "")
	if err != nil {
		return "", err
	}
	for _, value := range values {
		if value.ScopeID == scopeID && value.Openable {
			return value.RegistryPath, nil
		}
	}
	return "", errors.New("Scope is not registered and available")
}

func inspectCatalogScope(registryPath, kind, expectedScopeID string) LocusScopeEntry {
	value := LocusScopeEntry{
		ScopeID: expectedScopeID, Kind: kind, RegistryPath: registryPath,
		Availability: "missing", Openable: false,
	}
	if _, err := os.Stat(filepath.Join(registryPath, "scope.yaml")); err != nil {
		return value
	}
	registry, err := LoadScopeRegistry(registryPath, false)
	if err != nil {
		value.Availability = "invalid"
		return value
	}
	if expectedScopeID != "" && registry.Manifest.ScopeID != expectedScopeID {
		value.Availability = "identity_mismatch"
		return value
	}
	value.ScopeID = registry.Manifest.ScopeID
	value.Availability = "available"
	value.Openable = true
	return value
}
