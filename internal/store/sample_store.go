package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanSampleRow(rs rowScanner) (*model.SamplePoint, error) {
	var (
		id, siteID, unitID, label, material, note, collectedAt string
		depth                                               float64
		createdAt                                           sql.NullString
	)
	if err := rs.Scan(&id, &siteID, &unitID, &label, &depth, &material, &collectedAt, &note, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return &model.SamplePoint{
		ID:          id,
		SiteID:      siteID,
		UnitID:      unitID,
		Label:       label,
		Depth:       depth,
		Material:    material,
		CollectedAt: mustParse(collectedAt),
		Note:        note,
		CreatedAt:   mustParse(createdAt.String),
	}, nil
}

func (s *Store) CreateSample(sp *model.SamplePoint) error {
	_, err := s.DB.Exec(
		`INSERT INTO sample_points (id,site_id,unit_id,label,depth,material,collected_at,note,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		sp.ID, sp.SiteID, sp.UnitID, sp.Label, sp.Depth, sp.Material,
		sp.CollectedAt.Format("2006-01-02T15:04:05Z07:00"), sp.Note,
		sp.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	return err
}

func (s *Store) GetSample(id string) (*model.SamplePoint, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,unit_id,label,depth,material,collected_at,note,created_at
		 FROM sample_points WHERE id=?`, id)
	return scanSampleRow(row)
}

func (s *Store) DeleteSample(id string) error {
	_, err := s.DB.Exec(`DELETE FROM sample_points WHERE id=?`, id)
	return err
}

func (s *Store) ListSamples(siteID string) ([]model.SamplePoint, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,unit_id,label,depth,material,collected_at,note,created_at
		 FROM sample_points WHERE site_id=? ORDER BY depth`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SamplePoint
	for rows.Next() {
		sp, err := scanSampleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sp)
	}
	return out, rows.Err()
}
