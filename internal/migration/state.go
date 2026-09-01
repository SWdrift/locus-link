package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	PreviousStateSchemaVersion = 0
	CurrentStateSchemaVersion  = 1
)

var ErrMigrationRequired = errors.New("state database requires migration")

type StateResult struct {
	Status      string `json:"status"`
	StatePath   string `json:"state_path"`
	BackupPath  string `json:"backup_path,omitempty"`
	FromVersion int    `json:"from_version"`
	ToVersion   int    `json:"to_version"`
}

func PrepareCurrentState(db *sql.DB) error {
	version, err := stateVersion(db)
	if err != nil {
		return err
	}
	empty, err := stateIsEmpty(db)
	if err != nil {
		return err
	}
	if version == PreviousStateSchemaVersion && empty {
		return initializeState(db)
	}
	if version == PreviousStateSchemaVersion {
		return fmt.Errorf("%w: schema %d must be migrated to %d", ErrMigrationRequired, version, CurrentStateSchemaVersion)
	}
	if version != CurrentStateSchemaVersion {
		return fmt.Errorf("unsupported state schema %d; expected %d", version, CurrentStateSchemaVersion)
	}
	return nil
}

func MigrateState(path, backupDir string) (StateResult, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return StateResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return StateResult{}, err
	}
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return StateResult{}, err
	}
	defer db.Close()

	version, err := stateVersion(db)
	if err != nil {
		return StateResult{}, err
	}
	result := StateResult{StatePath: absolute, FromVersion: version, ToVersion: CurrentStateSchemaVersion}
	if version == CurrentStateSchemaVersion {
		result.Status = "no_change"
		return result, nil
	}
	if version != PreviousStateSchemaVersion {
		return StateResult{}, fmt.Errorf("unsupported state schema %d; only %d to %d is supported", version, PreviousStateSchemaVersion, CurrentStateSchemaVersion)
	}
	empty, err := stateIsEmpty(db)
	if err != nil {
		return StateResult{}, err
	}
	if empty {
		if err := initializeState(db); err != nil {
			return StateResult{}, err
		}
		result.Status = "initialized"
		return result, nil
	}

	backupPath, err := backupState(db, absolute, backupDir)
	if err != nil {
		return StateResult{}, err
	}
	result.BackupPath = backupPath
	if err := migratePreviousState(db); err != nil {
		return StateResult{}, err
	}
	result.Status = "migrated"
	return result, nil
}

func initializeState(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := applyCurrentSchema(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func migratePreviousState(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := applyCurrentSchema(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

type schemaExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

func applyCurrentSchema(db schemaExecutor) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subject TEXT NOT NULL,
			vantage TEXT NOT NULL,
			declaration_digest TEXT NOT NULL DEFAULT '',
			source_digest TEXT NOT NULL DEFAULT '',
			binding_digest TEXT NOT NULL DEFAULT '',
			probe_kind TEXT NOT NULL DEFAULT '',
			probe_semantics_version TEXT NOT NULL DEFAULT '',
			context_fingerprint TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			observed_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			provider TEXT NOT NULL,
			evidence TEXT NOT NULL,
			error TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_registrations (
			scope_id TEXT PRIMARY KEY,
			registry_path TEXT NOT NULL UNIQUE,
			registered_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS source_cache_entries (
			owner_scope_id TEXT NOT NULL,
			import_alias TEXT NOT NULL,
			configured_source_digest TEXT NOT NULL,
			expected_scope_id TEXT NOT NULL,
			actual_scope_id TEXT NOT NULL,
			active_content_digest TEXT NOT NULL,
			resolved_revision TEXT NOT NULL,
			object_path TEXT NOT NULL,
			etag TEXT NOT NULL,
			last_modified TEXT NOT NULL,
			last_refresh_status TEXT NOT NULL,
			last_refresh_error TEXT NOT NULL,
			refreshed_at TEXT NOT NULL,
			PRIMARY KEY(owner_scope_id, import_alias)
		)`,
		`CREATE TABLE IF NOT EXISTS scope_authorities (
			scope_id TEXT PRIMARY KEY,
			active_content_digest TEXT NOT NULL,
			object_path TEXT NOT NULL,
			provenance TEXT NOT NULL,
			activated_at TEXT NOT NULL
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	if err := addObservationApplicabilityColumns(db); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS observations_applicability ON observations(
		subject,vantage,declaration_digest,source_digest,binding_digest,probe_kind,probe_semantics_version,context_fingerprint,observed_at DESC
	)`); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", CurrentStateSchemaVersion))
	return err
}

func addObservationApplicabilityColumns(db schemaExecutor) error {
	rows, err := db.Query(`PRAGMA table_info(observations)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, column := range []string{
		"declaration_digest", "source_digest", "binding_digest", "probe_kind", "probe_semantics_version", "context_fingerprint",
	} {
		if columns[column] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE observations ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

func stateVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func stateIsEmpty(db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','index') AND name NOT LIKE 'sqlite_%'`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func backupState(db *sql.DB, statePath, backupDir string) (string, error) {
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(statePath), "backups")
	}
	absoluteBackupDir, err := filepath.Abs(backupDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absoluteBackupDir, 0o755); err != nil {
		return "", err
	}
	backupPath := filepath.Join(absoluteBackupDir, fmt.Sprintf("state-schema-%d-%s.db", PreviousStateSchemaVersion, time.Now().UTC().Format("20060102T150405.000000000Z")))
	quoted := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := db.Exec(`VACUUM INTO '` + quoted + `'`); err != nil {
		return "", fmt.Errorf("backup state database: %w", err)
	}
	return backupPath, nil
}
