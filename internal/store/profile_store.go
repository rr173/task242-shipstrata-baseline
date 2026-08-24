package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanProfileRow(rs rowScanner) (*model.ProfileVersion, error) {
	var (
		id, siteID, status, snapshot, note string
		version                            int
		createdAt, frozenAt               sql.NullString
	)
	if err := rs.Scan(&id, &siteID, &version, &status, &snapshot, &note, &createdAt, &frozenAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	p := &model.ProfileVersion{
		ID:        id,
		SiteID:    siteID,
		Version:   version,
		Status:    status,
		Snapshot:  snapshot,
		Note:      note,
		CreatedAt: mustParse(createdAt.String),
	}
	if frozenAt.Valid && frozenAt.String != "" {
		t := mustParse(frozenAt.String)
		p.FrozenAt = &t
	}
	return p, nil
}

func (s *Store) CreateProfile(p *model.ProfileVersion) error {
	_, err := s.DB.Exec(
		`INSERT INTO profile_versions (id,site_id,version,status,snapshot,note,created_at,frozen_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		p.ID, p.SiteID, p.Version, p.Status, p.Snapshot, p.Note,
		p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), nullTimeStr(p.FrozenAt))
	return err
}

func (s *Store) GetProfile(id string) (*model.ProfileVersion, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,version,status,snapshot,note,created_at,frozen_at FROM profile_versions WHERE id=?`, id)
	return scanProfileRow(row)
}

func (s *Store) ListProfiles(siteID string) ([]model.ProfileVersion, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,version,status,snapshot,note,created_at,frozen_at FROM profile_versions WHERE site_id=? ORDER BY version`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ProfileVersion
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) NextProfileVersion(siteID string) (int, error) {
	var v sql.NullInt64
	err := s.DB.QueryRow(`SELECT MAX(version) FROM profile_versions WHERE site_id=?`, siteID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if !v.Valid {
		return 1, nil
	}
	return int(v.Int64) + 1, nil
}

func (s *Store) UpdateProfileStatus(id, status string, frozen bool) error {
	var frozenAt interface{}
	if frozen {
		frozenAt = nowUTC()
	}
	_, err := s.DB.Exec(
		`UPDATE profile_versions SET status=?, frozen_at=COALESCE(frozen_at,?) WHERE id=?`,
		status, frozenAt, id)
	return err
}
