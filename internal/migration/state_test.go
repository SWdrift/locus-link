package migration

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateStateInitializesFreshDatabase(t *testing.T) {
	root := migrationTestRoot(t, "fresh")
	statePath := filepath.Join(root, "state.db")
	result, err := MigrateState(statePath, filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "initialized" || result.FromVersion != 0 || result.ToVersion != CurrentStateSchemaVersion || result.BackupPath != "" {
		t.Fatalf("unexpected result: %#v", result)
	}

	db := openMigrationTestDB(t, statePath)
	defer db.Close()
	assertStateVersion(t, db, CurrentStateSchemaVersion)
	for _, table := range []string{"observations", "project_registrations", "source_cache_entries", "scope_authorities"} {
		var found string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&found); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
}

func TestMigrateStateBacksUpAndPreservesPreviousSchema(t *testing.T) {
	root := migrationTestRoot(t, "previous")
	statePath := filepath.Join(root, "state.db")
	db := openMigrationTestDB(t, statePath)
	_, err := db.Exec(`CREATE TABLE observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject TEXT NOT NULL,
		vantage TEXT NOT NULL,
		status TEXT NOT NULL,
		observed_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		provider TEXT NOT NULL,
		evidence TEXT NOT NULL,
		error TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO observations(subject,vantage,status,observed_at,expires_at,provider,evidence,error) VALUES('scope::link','office','success','2026-01-01T00:00:00Z','0001-01-01T00:00:00Z','ssh','{}','')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(root, "backups")
	result, err := MigrateState(statePath, backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "migrated" || result.BackupPath == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	db = openMigrationTestDB(t, statePath)
	defer db.Close()
	assertStateVersion(t, db, CurrentStateSchemaVersion)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM observations WHERE subject='scope::link'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy observation count = %d, err = %v", count, err)
	}

	second, err := MigrateState(statePath, backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "no_change" || second.BackupPath != "" {
		t.Fatalf("second migration = %#v", second)
	}
}

func TestPrepareCurrentStateRejectsUnmigratedDatabase(t *testing.T) {
	root := migrationTestRoot(t, "required")
	db := openMigrationTestDB(t, filepath.Join(root, "state.db"))
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE legacy(value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := PrepareCurrentState(db); !errors.Is(err, ErrMigrationRequired) {
		t.Fatalf("PrepareCurrentState error = %v", err)
	}
}

func migrationTestRoot(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "temp", "migration-tests", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func openMigrationTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertStateVersion(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	version, err := stateVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	if version != want {
		t.Fatalf("schema version = %d, want %d", version, want)
	}
}
