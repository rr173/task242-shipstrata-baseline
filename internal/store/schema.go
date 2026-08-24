package store

import (
	"fmt"
)

// Migrate 创建全部数据表（幂等）。
func (s *Store) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS site_batches (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			location TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			sealed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS strata_units (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			label TEXT NOT NULL,
			category TEXT NOT NULL,
			depth_min REAL NOT NULL,
			depth_max REAL NOT NULL,
			status TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			from_unit_id TEXT NOT NULL,
			to_unit_id TEXT NOT NULL,
			relation TEXT NOT NULL,
			status TEXT NOT NULL,
			survey_source TEXT NOT NULL DEFAULT '',
			survey_seq INTEGER NOT NULL DEFAULT 0,
			fingerprint TEXT NOT NULL UNIQUE,
			note TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sample_points (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			unit_id TEXT NOT NULL,
			label TEXT NOT NULL,
			depth REAL NOT NULL,
			material TEXT NOT NULL DEFAULT '',
			collected_at TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS profile_versions (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			snapshot TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			frozen_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS contradictions (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			cycle_units TEXT NOT NULL DEFAULT '[]',
			involved_contacts TEXT NOT NULL DEFAULT '[]',
			resolved INTEGER NOT NULL DEFAULT 0,
			resolution TEXT NOT NULL DEFAULT '',
			detected_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS intrusion_candidates (
			id TEXT PRIMARY KEY,
			site_id TEXT NOT NULL,
			unit_id TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			supporting_contacts TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_units_site ON strata_units(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_site ON contacts(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_fp ON contacts(fingerprint)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_site ON sample_points(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_profiles_site ON profile_versions(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_contradictions_site ON contradictions(site_id)`,
		`CREATE INDEX IF NOT EXISTS idx_intrusion_site ON intrusion_candidates(site_id)`,
	}
	for _, st := range stmts {
		if _, err := s.DB.Exec(st); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}
