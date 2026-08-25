package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func (s *Store) CreateIntrusionCandidate(ic *model.IntrusionCandidate) error {
	_, err := s.DB.Exec(
		`INSERT INTO intrusion_candidates (id,site_id,unit_id,reason,supporting_contacts,status,created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		ic.ID, ic.SiteID, ic.UnitID, ic.Reason, ic.SupportingContacts, ic.Status,
		ic.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return err
}

func (s *Store) ListIntrusionCandidates(siteID string) ([]model.IntrusionCandidate, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,unit_id,reason,supporting_contacts,status,created_at
		 FROM intrusion_candidates WHERE site_id=? ORDER BY created_at`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.IntrusionCandidate
	for rows.Next() {
		var (
			id, sid, uid, reason, sc, status string
			createdAt                        sql.NullString
		)
		if err := rows.Scan(&id, &sid, &uid, &reason, &sc, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, model.IntrusionCandidate{
			ID:                 id,
			SiteID:             sid,
			UnitID:             uid,
			Reason:             reason,
			SupportingContacts: sc,
			Status:             status,
			CreatedAt:          mustParse(createdAt.String),
		})
	}
	return out, rows.Err()
}

func (s *Store) GetOpenIntrusionCandidate(siteID, unitID string) (*model.IntrusionCandidate, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,unit_id,reason,supporting_contacts,status,created_at
		 FROM intrusion_candidates WHERE site_id=? AND unit_id=? AND status=? ORDER BY created_at LIMIT 1`,
		siteID, unitID, model.IntrusionOpen)
	var id, sid, uid, reason, sc, status string
	var createdAt sql.NullString
	if err := row.Scan(&id, &sid, &uid, &reason, &sc, &status, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNoOpenCandidate
		}
		return nil, err
	}
	return &model.IntrusionCandidate{ID: id, SiteID: sid, UnitID: uid, Reason: reason, SupportingContacts: sc, Status: status, CreatedAt: mustParse(createdAt.String)}, nil
}

func (s *Store) AdjudicateIntrusion(siteID, unitID, candidateID, unitStatus, candidateStatus string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE strata_units SET status=?, version=version+1, updated_at=? WHERE site_id=? AND id=?`, unitStatus, nowUTC(), siteID, unitID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		_ = tx.Rollback()
		return model.ErrVersionConflict
	}
	res, err = tx.Exec(`UPDATE intrusion_candidates SET status=? WHERE site_id=? AND id=? AND status=?`, candidateStatus, siteID, candidateID, model.IntrusionOpen)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		_ = tx.Rollback()
		return model.ErrNoOpenCandidate
	}
	return tx.Commit()
}

// DeleteOpenIntrusionCandidates 删除仍处于 open 的侵扰候选，供重新求解时刷新。
func (s *Store) DeleteOpenIntrusionCandidates(siteID string) error {
	_, err := s.DB.Exec(`DELETE FROM intrusion_candidates WHERE site_id=? AND status=?`, siteID, model.IntrusionOpen)
	return err
}

func (s *Store) UpdateIntrusionStatus(id, status string) error {
	_, err := s.DB.Exec(`UPDATE intrusion_candidates SET status=? WHERE id=?`, status, id)
	return err
}
