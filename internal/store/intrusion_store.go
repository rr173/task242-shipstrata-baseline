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
			ID:                id,
			SiteID:            sid,
			UnitID:            uid,
			Reason:            reason,
			SupportingContacts: sc,
			Status:            status,
			CreatedAt:         mustParse(createdAt.String),
		})
	}
	return out, rows.Err()
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
