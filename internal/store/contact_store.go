package store

import (
	"database/sql"

	"task242-shipstrata/internal/model"
)

func scanContactRow(rs rowScanner) (*model.Contact, error) {
	var (
		id, siteID, fromID, toID, relation, status, source, note string
		seq                                                      int
		fp                                                       string
		version                                                  int
		createdAt, updatedAt                                     sql.NullString
	)
	if err := rs.Scan(&id, &siteID, &fromID, &toID, &relation, &status, &source, &seq, &fp, &note, &version, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return &model.Contact{
		ID:           id,
		SiteID:       siteID,
		FromUnitID:   fromID,
		ToUnitID:     toID,
		Relation:     relation,
		Status:       status,
		SurveySource: source,
		SurveySeq:    seq,
		Fingerprint:  fp,
		Note:         note,
		Version:      version,
		CreatedAt:    mustParse(createdAt.String),
		UpdatedAt:    mustParse(updatedAt.String),
	}, nil
}

// CreateContact 插入接触关系；若指纹已存在则忽略并报告重复（幂等）。
func (s *Store) CreateContact(c *model.Contact) (bool, error) {
	res, err := s.DB.Exec(
		`INSERT OR IGNORE INTO contacts
		 (id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.SiteID, c.FromUnitID, c.ToUnitID, c.Relation, c.Status, c.SurveySource, c.SurveySeq, c.Fingerprint, c.Note, c.Version,
		c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CreateContactsBatch inserts all contacts under one SQLite transaction.
func (s *Store) CreateContactsBatch(contacts []*model.Contact) (int, int, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	added, skipped := 0, 0
	for _, c := range contacts {
		res, execErr := tx.Exec(
			`INSERT OR IGNORE INTO contacts
			 (id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			c.ID, c.SiteID, c.FromUnitID, c.ToUnitID, c.Relation, c.Status, c.SurveySource, c.SurveySeq, c.Fingerprint, c.Note, c.Version,
			c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		)
		if execErr != nil {
			_ = tx.Rollback()
			return 0, 0, execErr
		}
		n, rowsErr := res.RowsAffected()
		if rowsErr != nil {
			_ = tx.Rollback()
			return 0, 0, rowsErr
		}
		if n == 0 {
			skipped++
		} else {
			added++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return added, skipped, nil
}

func (s *Store) GetContact(id string) (*model.Contact, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at
		 FROM contacts WHERE id=?`, id)
	return scanContactRow(row)
}

func (s *Store) GetContactByFingerprint(fp string) (*model.Contact, error) {
	row := s.DB.QueryRow(
		`SELECT id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at
		 FROM contacts WHERE fingerprint=?`, fp)
	return scanContactRow(row)
}

func (s *Store) ListContacts(siteID string) ([]model.Contact, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at
		 FROM contacts WHERE site_id=? ORDER BY survey_seq`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Contact
	for rows.Next() {
		c, err := scanContactRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ListActiveContacts 返回已确认或当前矛盾的接触关系（参与偏序求解）。
func (s *Store) ListActiveContacts(siteID string) ([]model.Contact, error) {
	rows, err := s.DB.Query(
		`SELECT id,site_id,from_unit_id,to_unit_id,relation,status,survey_source,survey_seq,fingerprint,note,version,created_at,updated_at
		 FROM contacts WHERE site_id=? AND status<>? ORDER BY survey_seq`, siteID, model.ContactStatusExcluded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Contact
	for rows.Next() {
		c, err := scanContactRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateContactStatus(id, status string) error {
	_, err := s.DB.Exec(
		`UPDATE contacts SET status=?, version=version+1, updated_at=? WHERE id=?`,
		status, nowUTC(), id)
	return err
}

// RestoreContactConfirmation clears a stale derived conflict after the
// underlying confirmed edge is no longer part of a cycle.
func (s *Store) RestoreContactConfirmation(id string) error {
	_, err := s.DB.Exec(
		`UPDATE contacts SET status=?, version=version+1, updated_at=? WHERE id=? AND status=?`,
		model.ContactStatusConfirmed, nowUTC(), id, model.ContactStatusConflict)
	return err
}
