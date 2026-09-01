package locus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"locus-link/internal/migration"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Observation struct {
	ID                    int64          `json:"id,omitempty"`
	Subject               string         `json:"subject"`
	Vantage               string         `json:"vantage"`
	DeclarationDigest     string         `json:"declaration_digest"`
	SourceDigest          string         `json:"source_digest"`
	BindingDigest         string         `json:"binding_digest"`
	ProbeKind             string         `json:"probe_kind"`
	ProbeSemanticsVersion string         `json:"probe_semantics_version"`
	ContextFingerprint    string         `json:"context_fingerprint"`
	Status                string         `json:"status"`
	ObservedAt            time.Time      `json:"observed_at"`
	ExpiresAt             time.Time      `json:"expires_at"`
	Provider              string         `json:"provider"`
	Evidence              map[string]any `json:"evidence,omitempty"`
	Error                 string         `json:"error,omitempty"`
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultStatePath() (string, error) {
	if path := os.Getenv("LOCUS_STATE_PATH"); path != "" {
		return filepath.Abs(path)
	}
	home, err := DefaultHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "state", "state.db"), nil
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migration.PrepareCurrentState(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string { return s.path }

func (s *Store) Append(ctx context.Context, observation Observation) (Observation, error) {
	evidence, err := json.Marshal(observation.Evidence)
	if err != nil {
		return observation, err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO observations(
		subject,vantage,declaration_digest,source_digest,binding_digest,probe_kind,probe_semantics_version,context_fingerprint,
		status,observed_at,expires_at,provider,evidence,error
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		observation.Subject, observation.Vantage, observation.DeclarationDigest, observation.SourceDigest, observation.BindingDigest,
		observation.ProbeKind, observation.ProbeSemanticsVersion, observation.ContextFingerprint,
		observation.Status, observation.ObservedAt.Format(time.RFC3339Nano), observation.ExpiresAt.Format(time.RFC3339Nano),
		observation.Provider, string(evidence), observation.Error)
	if err != nil {
		return observation, err
	}
	observation.ID, _ = result.LastInsertId()
	return observation, nil
}

func (s *Store) Latest(ctx context.Context, subject, vantage string) (*Observation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,subject,vantage,declaration_digest,source_digest,binding_digest,probe_kind,probe_semantics_version,context_fingerprint,status,observed_at,expires_at,provider,evidence,error
		FROM observations WHERE subject=? AND vantage=? ORDER BY observed_at DESC,id DESC LIMIT 1`, subject, vantage)
	observation, err := scanObservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return observation, err
}

func (s *Store) LatestApplicable(ctx context.Context, value ObservationApplicability) (*Observation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,subject,vantage,declaration_digest,source_digest,binding_digest,probe_kind,probe_semantics_version,context_fingerprint,status,observed_at,expires_at,provider,evidence,error
		FROM observations
		WHERE subject=? AND vantage=? AND declaration_digest=? AND source_digest=? AND binding_digest=?
		  AND probe_kind=? AND probe_semantics_version=? AND context_fingerprint=?
		ORDER BY observed_at DESC,id DESC LIMIT 1`,
		value.Subject, value.Vantage, value.DeclarationDigest, value.SourceDigest, value.BindingDigest,
		value.ProbeKind, value.ProbeSemanticsVersion, value.ContextFingerprint)
	observation, err := scanObservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return observation, err
}

func (s *Store) AllLatest(ctx context.Context, subject string) ([]Observation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,o.subject,o.vantage,o.declaration_digest,o.source_digest,o.binding_digest,o.probe_kind,o.probe_semantics_version,o.context_fingerprint,o.status,o.observed_at,o.expires_at,o.provider,o.evidence,o.error
		FROM observations o
		JOIN (SELECT vantage,MAX(id) id FROM observations WHERE subject=? GROUP BY vantage) latest ON o.id=latest.id
		ORDER BY o.vantage`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Observation
	for rows.Next() {
		value, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	return values, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanObservation(row scanner) (*Observation, error) {
	var value Observation
	var observed, expires, evidence string
	if err := row.Scan(
		&value.ID, &value.Subject, &value.Vantage, &value.DeclarationDigest, &value.SourceDigest, &value.BindingDigest,
		&value.ProbeKind, &value.ProbeSemanticsVersion, &value.ContextFingerprint,
		&value.Status, &observed, &expires, &value.Provider, &evidence, &value.Error,
	); err != nil {
		return nil, err
	}
	var err error
	value.ObservedAt, err = time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return nil, err
	}
	value.ExpiresAt, err = time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(evidence), &value.Evidence); err != nil {
		return nil, fmt.Errorf("invalid observation evidence: %w", err)
	}
	return &value, nil
}

func newObservation(link *Link, runtime RuntimeContext, provider, evidenceKind string) Observation {
	now := time.Now().UTC()
	return Observation{
		Subject:    link.CanonicalID,
		Vantage:    runtime.Vantage,
		Status:     "unknown",
		ObservedAt: now,
		ExpiresAt:  now.Add(15 * time.Minute),
		Provider:   provider,
		Evidence:   map[string]any{"kind": evidenceKind},
	}
}

func finishObservation(value Observation, err error) Observation {
	if err == nil {
		value.Status = "success"
		return value
	}
	value.Status = "failure"
	value.Error = err.Error()
	return value
}
