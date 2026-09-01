package locus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ProjectRegistration struct {
	ScopeID      string    `json:"scope_id"`
	RegistryPath string    `json:"registry_path"`
	RegisteredAt time.Time `json:"registered_at"`
	Available    bool      `json:"available"`
}

func (s *Store) RegisterProject(ctx context.Context, registryPath, home string) (ProjectRegistration, error) {
	absolute, err := filepath.Abs(filepath.Clean(registryPath))
	if err != nil {
		return ProjectRegistration{}, err
	}
	registry, err := CollectRegistry(absolute, CollectorOptions{Home: home, Store: s})
	if err != nil {
		return ProjectRegistration{}, err
	}
	if registry.Completeness != Complete {
		return ProjectRegistration{}, errors.New("project Registry is partial")
	}
	registeredAt := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO project_registrations(scope_id,registry_path,registered_at)
		VALUES(?,?,?)
		ON CONFLICT(scope_id) DO UPDATE SET registry_path=excluded.registry_path,registered_at=excluded.registered_at`,
		registry.RootScopeID, absolute, registeredAt.Format(time.RFC3339Nano))
	if err != nil {
		return ProjectRegistration{}, fmt.Errorf("register project %s: %w", registry.RootScopeID, err)
	}
	return ProjectRegistration{ScopeID: registry.RootScopeID, RegistryPath: absolute, RegisteredAt: registeredAt, Available: true}, nil
}

func (s *Store) UnregisterProject(ctx context.Context, scopeID string) (bool, error) {
	if !validDeclarationName(scopeID) {
		return false, fmt.Errorf("invalid scope_id %q", scopeID)
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM project_registrations WHERE scope_id=?`, scopeID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count != 0, err
}

func (s *Store) ProjectRegistration(ctx context.Context, scopeID string) (*ProjectRegistration, error) {
	row := s.db.QueryRowContext(ctx, `SELECT scope_id,registry_path,registered_at FROM project_registrations WHERE scope_id=?`, scopeID)
	value, err := scanProjectRegistration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return value, err
}

func (s *Store) ListProjects(ctx context.Context) ([]ProjectRegistration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT scope_id,registry_path,registered_at FROM project_registrations ORDER BY scope_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []ProjectRegistration
	for rows.Next() {
		var value ProjectRegistration
		var registeredAt string
		if err := rows.Scan(&value.ScopeID, &value.RegistryPath, &registeredAt); err != nil {
			return nil, err
		}
		value.RegisteredAt, err = time.Parse(time.RFC3339Nano, registeredAt)
		if err != nil {
			return nil, err
		}
		_, statErr := os.Stat(filepath.Join(value.RegistryPath, "scope.yaml"))
		value.Available = statErr == nil
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ScopeID < values[j].ScopeID })
	return values, rows.Err()
}

func scanProjectRegistration(row *sql.Row) (*ProjectRegistration, error) {
	var value ProjectRegistration
	var registeredAt string
	if err := row.Scan(&value.ScopeID, &value.RegistryPath, &registeredAt); err != nil {
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, registeredAt)
	if err != nil {
		return nil, err
	}
	value.RegisteredAt = parsed
	_, statErr := os.Stat(filepath.Join(value.RegistryPath, "scope.yaml"))
	value.Available = statErr == nil
	return &value, nil
}
