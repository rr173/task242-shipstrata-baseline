package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanSiteRow(rs rowScanner) (*model.SiteBatch, error) {
	var (
		id, code, name, location, status string
		version                          int
		createdAt, updatedAt, sealedAt   sql.NullString
	)
	if err := rs.Scan(&id, &code, &name, &location, &status, &version, &createdAt, &updatedAt, &sealedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	sb := &model.SiteBatch{
		ID:        id,
		Code:      code,
		Name:      name,
		Location:  location,
		Status:    status,
		Version:   version,
		CreatedAt: mustParse(createdAt.String),
		UpdatedAt: mustParse(updatedAt.String),
	}
	if sealedAt.Valid && sealedAt.String != "" {
		t := mustParse(sealedAt.String)
		sb.SealedAt = &t
	}
	return sb, nil
}

func (s *Store) CreateSite(site *model.SiteBatch) error {
	_, err := s.DB.Exec(
		`INSERT INTO site_batches (id,code,name,location,status,version,created_at,updated_at,sealed_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		site.ID, site.Code, site.Name, site.Location, site.Status, site.Version,
		site.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		site.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		nullTimeStr(site.SealedAt),
	)
	return err
}

func (s *Store) GetSite(id string) (*model.SiteBatch, error) {
	row := s.DB.QueryRow(
		`SELECT id,code,name,location,status,version,created_at,updated_at,sealed_at
		 FROM site_batches WHERE id=?`, id)
	return scanSiteRow(row)
}

// EnsureSiteWritable centralizes the lifecycle boundary for evidence writes.
func (s *Store) EnsureSiteWritable(id string) error {
	sb, err := s.GetSite(id)
	if err != nil {
		return err
	}
	if sb.Status == model.SiteStatusSealed {
		return model.ErrSiteSealed
	}
	return nil
}

func (s *Store) ListSites() ([]model.SiteBatch, error) {
	rows, err := s.DB.Query(
		`SELECT id,code,name,location,status,version,created_at,updated_at,sealed_at
		 FROM site_batches ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SiteBatch
	for rows.Next() {
		sb, err := scanSiteRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sb)
	}
	return out, rows.Err()
}

// UpdateSiteStatus 设置遗址批次状态并自增版本号；sealed 时写入 sealed_at。
func (s *Store) UpdateSiteStatus(id, status string, sealed bool) error {
	var sealedAt interface{}
	if sealed {
		sealedAt = nowUTC()
	}
	_, err := s.DB.Exec(
		`UPDATE site_batches SET status=?, version=version+1, updated_at=?, sealed_at=COALESCE(sealed_at,?)
		 WHERE id=?`,
		status, nowUTC(), sealedAt, id)
	return err
}

func (s *Store) AdvanceSiteCAS(id, from, to string, seal bool) error {
	var sealedAt interface{}
	if seal {
		sealedAt = nowUTC()
	}
	res, err := s.DB.Exec(
		`UPDATE site_batches SET status=?, version=version+1, updated_at=?, sealed_at=COALESCE(sealed_at,?) WHERE id=? AND status=?`,
		to, nowUTC(), sealedAt, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return model.ErrVersionConflict
	}
	return nil
}
