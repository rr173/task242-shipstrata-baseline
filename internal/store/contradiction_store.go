package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func (s *Store) CreateContradiction(c *model.Contradiction) error {
	_, err := s.DB.Exec(
		`INSERT INTO contradictions (id,site_id,cycle_units,involved_contacts,resolved,resolution,detected_at)
		 VALUES (?,?,?,?,?,?,?)`,
		c.ID, c.SiteID, c.CycleUnits, c.InvolvedContacts, boolToInt(c.Resolved), c.Resolution,
		c.DetectedAt.Format("2006-01-02T15:04:05Z07:00"))
	return err
}

func (s *Store) ListContradictions(siteID string) ([]model.Contradiction, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,cycle_units,involved_contacts,resolved,resolution,detected_at
		 FROM contradictions WHERE site_id=? ORDER BY detected_at`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Contradiction
	for rows.Next() {
		var (
			id, sid, cu, ic, resolution string
			resolved                    int
			detectedAt                  sql.NullString
		)
		if err := rows.Scan(&id, &sid, &cu, &ic, &resolved, &resolution, &detectedAt); err != nil {
			return nil, err
		}
		out = append(out, model.Contradiction{
			ID:               id,
			SiteID:           sid,
			CycleUnits:       cu,
			InvolvedContacts: ic,
			Resolved:         resolved != 0,
			Resolution:       resolution,
			DetectedAt:       mustParse(detectedAt.String),
		})
	}
	return out, rows.Err()
}

// DeleteUnresolvedContradictions 删除未解决矛盾，供重新求解时刷新。
func (s *Store) DeleteUnresolvedContradictions(siteID string) error {
	_, err := s.DB.Exec(`DELETE FROM contradictions WHERE site_id=? AND resolved=0`, siteID)
	return err
}

func (s *Store) ResolveContradiction(id, resolution string) error {
	_, err := s.DB.Exec(`UPDATE contradictions SET resolved=1, resolution=? WHERE id=? AND resolved=0`, resolution, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
