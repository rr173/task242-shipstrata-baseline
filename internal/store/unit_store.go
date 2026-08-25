package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanUnitRow(rs rowScanner) (*model.StrataUnit, error) {
	var (
		id, siteID, label, category, status string
		depthMin, depthMax                  float64
		version                             int
		createdAt, updatedAt                sql.NullString
	)
	if err := rs.Scan(&id, &siteID, &label, &category, &depthMin, &depthMax, &status, &version, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return &model.StrataUnit{
		ID:        id,
		SiteID:    siteID,
		Label:     label,
		Category:  category,
		DepthMin:  depthMin,
		DepthMax:  depthMax,
		Status:    status,
		Version:   version,
		CreatedAt: mustParse(createdAt.String),
		UpdatedAt: mustParse(updatedAt.String),
	}, nil
}

func (s *Store) CreateUnit(u *model.StrataUnit) error {
	_, err := s.DB.Exec(
		`INSERT INTO strata_units (id,site_id,label,category,depth_min,depth_max,status,version,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		u.ID, u.SiteID, u.Label, u.Category, u.DepthMin, u.DepthMax, u.Status, u.Version,
		u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	return err
}

func (s *Store) GetUnit(id string) (*model.StrataUnit, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,label,category,depth_min,depth_max,status,version,created_at,updated_at
		 FROM strata_units WHERE id=?`, id)
	return scanUnitRow(row)
}

func (s *Store) GetUnitForSite(siteID, id string) (*model.StrataUnit, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,label,category,depth_min,depth_max,status,version,created_at,updated_at
		 FROM strata_units WHERE id=?`, id)
	return scanUnitRow(row)
}

func (s *Store) ListUnits(siteID string) ([]model.StrataUnit, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,label,category,depth_min,depth_max,status,version,created_at,updated_at
		 FROM strata_units WHERE site_id=? ORDER BY label`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StrataUnit
	for rows.Next() {
		u, err := scanUnitRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUnitStatus(id, status string) error {
	_, err := s.DB.Exec(
		`UPDATE strata_units SET status=?, version=version+1, updated_at=? WHERE id=?`,
		status, nowUTC(), id)
	return err
}
