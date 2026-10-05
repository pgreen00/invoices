// Package store owns the SQLite database: opening, migrating, seeding, and
// every query the app runs.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"invoices/internal/dates"
)

//go:embed schema.sql
var schema string

// Store is a handle on the invoices database.
type Store struct {
	db   *sql.DB
	Path string
}

// Pragmas applied to every connection.
//
// DELETE journalling keeps the database a single self-contained file at rest,
// which is far friendlier to folder-syncing and file-copy backups than WAL's
// -wal/-shm sidecars.
var pragmas = []string{
	"busy_timeout(5000)",
	"foreign_keys(1)",
	"journal_mode(DELETE)",
	"synchronous(FULL)",
}

// Open opens (creating if necessary), migrates, and seeds the database at
// path. The parent directory must already exist.
func Open(path string) (*Store, error) {
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("could not open the database at %s\n  the path may not contain '?'", path)
	}

	dsn := path + "?_pragma=" + strings.Join(pragmas, "&_pragma=")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open the database at %s\n  %w", path, err)
	}

	// One connection serialises every statement. This is a single-user app
	// with tiny queries, and it rules out SQLITE_BUSY between our own
	// goroutines entirely.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not open the database at %s\n  %w", path, err)
	}

	s := &Store{db: db, Path: path}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not migrate the database at %s\n  %w", path, err)
	}
	if err := s.seed(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not seed the database at %s\n  %w", path, err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// DB exposes the underlying handle, for tests.
func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) count(table string) (int64, error) {
	var n int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	return n, err
}

func (s *Store) seed() error {
	settingsCount, err := s.count("settings")
	if err != nil {
		return err
	}
	if settingsCount == 0 {
		_, err := s.db.Exec(
			`INSERT INTO settings (
			   id, business_name, business_tagline, address_line1, address_line2,
			   city_state_zip, email, phone, tax_id, default_rate_cents,
			   default_terms_days, payment_instructions, notes, footer_message
			 ) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"Ravenline Software LLC",
			"Custom Software Development",
			"1420 Beacon Street",
			"Suite 300",
			"Austin, TX 78701",
			"billing@ravenline.dev",
			"(512) 555-0142",
			"EIN 87-1234567",
			16500,
			15,
			strings.Join([]string{
				"ACH / Wire — Lone Star Bank, N.A.",
				"Account Name: Ravenline Software LLC",
				"Routing: 111000025 · Account: 000123456789",
			}, "\n"),
			strings.Join([]string{
				"Payment due within 15 days of the invoice date.",
				"Late balances accrue 1.5% interest per month.",
				"Hours are billed in 15-minute increments; detailed time logs available on request.",
			}, "\n"),
			"Thank you for your business.",
		)
		if err != nil {
			return err
		}
	}

	clientCount, err := s.count("clients")
	if err != nil {
		return err
	}
	if clientCount == 0 {
		_, err := s.db.Exec(
			`INSERT INTO clients (
			   name, attn, address_line1, address_line2, city_state_zip, email,
			   default_project, default_contract, default_po,
			   default_rate_cents, default_terms_days, created_at
			 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"Northwind Retail Group, Inc.",
			"Dana Whitfield, VP Engineering",
			"98 Harbor Point Drive",
			"Floor 12",
			"Seattle, WA 98104",
			"ap@northwindretail.com",
			"Order Management Platform — Phase 2",
			"MSA dated 2024-01-15",
			"NW-PO-4471",
			16500,
			15,
			dates.Now(),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// transaction runs fn inside a transaction, rolling back on any error.
func (s *Store) transaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback() // the original error is the interesting one
		return err
	}
	return tx.Commit()
}
