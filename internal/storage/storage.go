package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sqlx.DB
}

func Open() (*Storage, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "data.sqlite")

	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db}, nil
}

func dataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "chankat"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "chankat"), nil
}

func (s *Storage) Migrate() error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.Get(&version, `PRAGMA user_version`); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if _, err := tx.Exec(migrations[i]); err != nil {
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return fmt.Errorf("set schema version %d: %w", i+1, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) QueryRow(query string, args ...interface{}) *sqlx.Row {
	return s.db.QueryRowx(query, args...)
}
