package locus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type SourceCacheEntry struct {
	OwnerScopeID           string    `json:"owner_scope_id"`
	ImportAlias            string    `json:"import_alias"`
	ConfiguredSourceDigest string    `json:"configured_source_digest"`
	CurrentSourceDigest    string    `json:"current_source_digest,omitempty"`
	ConfigurationChanged   bool      `json:"configuration_changed"`
	ExpectedScopeID        string    `json:"expected_scope_id,omitempty"`
	ActualScopeID          string    `json:"actual_scope_id,omitempty"`
	ActiveContentDigest    string    `json:"active_content_digest,omitempty"`
	ResolvedRevision       string    `json:"resolved_revision,omitempty"`
	ObjectPath             string    `json:"object_path,omitempty"`
	ETag                   string    `json:"etag,omitempty"`
	LastModified           string    `json:"last_modified,omitempty"`
	LastRefreshStatus      string    `json:"last_refresh_status"`
	LastRefreshError       string    `json:"last_refresh_error,omitempty"`
	RefreshedAt            time.Time `json:"refreshed_at"`
}

type ScopeAuthority struct {
	ScopeID             string    `json:"scope_id"`
	ActiveContentDigest string    `json:"active_content_digest"`
	ObjectPath          string    `json:"object_path"`
	Provenance          Source    `json:"provenance"`
	ActivatedAt         time.Time `json:"activated_at"`
}

func (s *Store) SourceCacheEntry(ownerScopeID, alias string) (*SourceCacheEntry, error) {
	row := s.db.QueryRow(`SELECT owner_scope_id,import_alias,configured_source_digest,expected_scope_id,actual_scope_id,
		active_content_digest,resolved_revision,object_path,etag,last_modified,last_refresh_status,last_refresh_error,refreshed_at
		FROM source_cache_entries WHERE owner_scope_id=? AND import_alias=?`, ownerScopeID, alias)
	value, err := scanSourceCacheEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return value, err
}

func (s *Store) SourceCacheEntries(ownerScopeID string) ([]SourceCacheEntry, error) {
	rows, err := s.db.Query(`SELECT owner_scope_id,import_alias,configured_source_digest,expected_scope_id,actual_scope_id,
		active_content_digest,resolved_revision,object_path,etag,last_modified,last_refresh_status,last_refresh_error,refreshed_at
		FROM source_cache_entries WHERE owner_scope_id=? ORDER BY import_alias`, ownerScopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []SourceCacheEntry
	for rows.Next() {
		value, err := scanSourceCacheEntry(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ImportAlias < values[j].ImportAlias })
	return values, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSourceCacheEntry(row rowScanner) (*SourceCacheEntry, error) {
	var value SourceCacheEntry
	var refreshedAt string
	if err := row.Scan(
		&value.OwnerScopeID, &value.ImportAlias, &value.ConfiguredSourceDigest, &value.ExpectedScopeID, &value.ActualScopeID,
		&value.ActiveContentDigest, &value.ResolvedRevision, &value.ObjectPath, &value.ETag, &value.LastModified,
		&value.LastRefreshStatus, &value.LastRefreshError, &refreshedAt,
	); err != nil {
		return nil, err
	}
	if refreshedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, refreshedAt)
		if err != nil {
			return nil, err
		}
		value.RefreshedAt = parsed
	}
	return &value, nil
}

func (s *Store) ScopeAuthority(scopeID string) (*ScopeAuthority, error) {
	var value ScopeAuthority
	var provenanceJSON, activatedAt string
	err := s.db.QueryRow(`SELECT scope_id,active_content_digest,object_path,provenance,activated_at
		FROM scope_authorities WHERE scope_id=?`, scopeID).Scan(
		&value.ScopeID, &value.ActiveContentDigest, &value.ObjectPath, &provenanceJSON, &activatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(provenanceJSON), &value.Provenance); err != nil {
		return nil, err
	}
	value.ActivatedAt, err = time.Parse(time.RFC3339Nano, activatedAt)
	return &value, err
}

func (s *Store) activateSource(ctx context.Context, entry SourceCacheEntry, source Source) error {
	provenance, err := json.Marshal(sanitizeSource(source))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO source_cache_entries(
		owner_scope_id,import_alias,configured_source_digest,expected_scope_id,actual_scope_id,active_content_digest,
		resolved_revision,object_path,etag,last_modified,last_refresh_status,last_refresh_error,refreshed_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(owner_scope_id,import_alias) DO UPDATE SET
		configured_source_digest=excluded.configured_source_digest,expected_scope_id=excluded.expected_scope_id,
		actual_scope_id=excluded.actual_scope_id,active_content_digest=excluded.active_content_digest,
		resolved_revision=excluded.resolved_revision,object_path=excluded.object_path,etag=excluded.etag,
		last_modified=excluded.last_modified,last_refresh_status=excluded.last_refresh_status,
		last_refresh_error='',refreshed_at=excluded.refreshed_at`,
		entry.OwnerScopeID, entry.ImportAlias, entry.ConfiguredSourceDigest, entry.ExpectedScopeID, entry.ActualScopeID,
		entry.ActiveContentDigest, entry.ResolvedRevision, entry.ObjectPath, entry.ETag, entry.LastModified, "success", "", now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scope_authorities(scope_id,active_content_digest,object_path,provenance,activated_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT(scope_id) DO UPDATE SET active_content_digest=excluded.active_content_digest,
		object_path=excluded.object_path,provenance=excluded.provenance,activated_at=excluded.activated_at`,
		entry.ActualScopeID, entry.ActiveContentDigest, entry.ObjectPath, string(provenance), now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) recordRefreshFailure(ctx context.Context, ownerScopeID string, imported Import, category string) error {
	if category == "" {
		category = "refresh_failed"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO source_cache_entries(
		owner_scope_id,import_alias,configured_source_digest,expected_scope_id,actual_scope_id,active_content_digest,
		resolved_revision,object_path,etag,last_modified,last_refresh_status,last_refresh_error,refreshed_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(owner_scope_id,import_alias) DO UPDATE SET configured_source_digest=excluded.configured_source_digest,
		expected_scope_id=excluded.expected_scope_id,last_refresh_status='failure',last_refresh_error=excluded.last_refresh_error,
		refreshed_at=excluded.refreshed_at`,
		ownerScopeID, imported.Alias, sourceDigest(imported.Source), imported.ExpectedScopeID, "", "", "", "", "", "", "failure", category, now)
	if err != nil {
		return fmt.Errorf("record refresh failure: %w", err)
	}
	return nil
}
