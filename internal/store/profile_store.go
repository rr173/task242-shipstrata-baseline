package store

import (
	"context"
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanProfileRow(rs rowScanner) (*model.ProfileVersion, error) {
	var (
		id, siteID, status, snapshot, note string
		version                            int
		createdAt, frozenAt                sql.NullString
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

// CreateNextProfile 在单个写事务中分配下一个连续版本号并写入快照。
// BEGIN IMMEDIATE 在 SELECT MAX 之前获取写锁，保证并发发布者串行进入
// 临界区：每个发布者读到的 MAX(version) 都是已提交的前序插入结果，
// 因此分配出的版本号唯一且连续。返回分配到的版本号。
func (s *Store) CreateNextProfile(p *model.ProfileVersion) (int, error) {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	// 首条语句用 BEGIN IMMEDIATE 语义抢写锁；modernc/sqlite 在事务首条
	// 语句上应用 busy_timeout，并发写者会等待而非立即失败。
	var next int
	if err := tx.QueryRow(
		`SELECT COALESCE(MAX(version),0)+1 FROM profile_versions WHERE site_id=?`,
		p.SiteID,
	).Scan(&next); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	p.Version = next
	if _, err := tx.Exec(
		`INSERT INTO profile_versions (id,site_id,version,status,snapshot,note,created_at,frozen_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		p.ID, p.SiteID, p.Version, p.Status, p.Snapshot, p.Note,
		p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), nullTimeStr(p.FrozenAt)); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
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

func (s *Store) SupersedeProfile(oldID, newID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	var oldSite, oldStatus, newSite, newStatus string
	if err := tx.QueryRow(`SELECT site_id,status FROM profile_versions WHERE id=?`, oldID).Scan(&oldSite, &oldStatus); err != nil {
		_ = tx.Rollback()
		return model.ErrNotFound
	}
	if err := tx.QueryRow(`SELECT site_id,status FROM profile_versions WHERE id=?`, newID).Scan(&newSite, &newStatus); err != nil {
		_ = tx.Rollback()
		return model.ErrNotFound
	}
	if oldSite != newSite || !model.CanSupersedeProfile(oldStatus, newStatus) {
		_ = tx.Rollback()
		return model.ErrInvalidStatus
	}
	res, err := tx.Exec(`UPDATE profile_versions SET status=? WHERE id=? AND site_id=? AND status IN (?,?)`, model.ProfileStatusSuperseded, oldID, oldSite, model.ProfileStatusShared, model.ProfileStatusFrozen)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		_ = tx.Rollback()
		return model.ErrVersionConflict
	}
	return tx.Commit()
}
