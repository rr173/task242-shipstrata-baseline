// Package contact 负责接触关系的导入、指纹幂等与确认/否决。
package contact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

// ImportInput is one survey edge in an atomic batch import.
type ImportInput struct {
	FromUnitID   string
	ToUnitID     string
	Relation     string
	SurveySource string
	SurveySeq    int
	Note         string
}

// Fingerprint 计算测绘边的稳定指纹：单元对 + 关系 + 测绘来源 + 序号。
func Fingerprint(from, to, rel, source string, seq int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d", from, to, rel, source, seq)))
	return hex.EncodeToString(h[:])
}

// Import 导入一条接触关系（同一测绘边指纹幂等）。
// 返回 (inserted bool, contact, error)：若指纹已存在则 inserted=false 且返回已有记录。
func Import(st *store.Store, siteID, fromUnit, toUnit, rel, source string, seq int, note string) (bool, *model.Contact, error) {
	if err := st.EnsureSiteWritable(siteID); err != nil {
		return false, nil, err
	}
	if !model.ValidRelation(rel) {
		return false, nil, model.ErrInvalidRelation
	}
	if fromUnit == toUnit {
		return false, nil, model.ErrSelfLoop
	}
	// 校验两端单元存在且属于同一遗址
	for _, uid := range []string{fromUnit, toUnit} {
		u, err := st.GetUnit(uid)
		if err != nil {
			return false, nil, model.ErrUnknownUnit
		}
		if u.SiteID != siteID {
			return false, nil, model.ErrUnknownUnit
		}
	}
	fp := Fingerprint(fromUnit, toUnit, rel, source, seq)
	if existing, err := st.GetContactByFingerprint(fp); err == nil {
		return false, existing, nil
	}

	now := time.Now().UTC()
	c := &model.Contact{
		ID:           model.NewID("ct"),
		SiteID:       siteID,
		FromUnitID:   fromUnit,
		ToUnitID:     toUnit,
		Relation:     rel,
		Status:       model.ContactStatusPending,
		SurveySource: source,
		SurveySeq:    seq,
		Fingerprint:  fp,
		Note:         note,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	ok, err := st.CreateContact(c)
	if err != nil {
		return false, nil, err
	}
	if !ok {
		// 竞态下被其他写入抢先，返回已有记录
		existing, e2 := st.GetContactByFingerprint(fp)
		if e2 == nil {
			return false, existing, nil
		}
		return false, nil, model.ErrDuplicateFingerprint
	}
	return true, c, nil
}

// ImportBatch validates all survey edges before writing and persists the
// complete batch in one transaction.
func ImportBatch(st *store.Store, siteID string, inputs []ImportInput) (int, int, error) {
	if err := st.EnsureSiteWritable(siteID); err != nil {
		return 0, 0, err
	}
	contacts := make([]*model.Contact, 0, len(inputs))
	for _, in := range inputs {
		if !model.ValidRelation(in.Relation) {
			return 0, 0, model.ErrInvalidRelation
		}
		if in.FromUnitID == in.ToUnitID {
			return 0, 0, model.ErrSelfLoop
		}
		for _, uid := range []string{in.FromUnitID, in.ToUnitID} {
			u, err := st.GetUnit(uid)
			if err != nil || u.SiteID != siteID {
				return 0, 0, model.ErrUnknownUnit
			}
		}
		now := time.Now().UTC()
		contacts = append(contacts, &model.Contact{
			ID:           model.NewID("ct"),
			SiteID:       siteID,
			FromUnitID:   in.FromUnitID,
			ToUnitID:     in.ToUnitID,
			Relation:     in.Relation,
			Status:       model.ContactStatusPending,
			SurveySource: in.SurveySource,
			SurveySeq:    in.SurveySeq,
			Fingerprint:  Fingerprint(in.FromUnitID, in.ToUnitID, in.Relation, in.SurveySource, in.SurveySeq),
			Note:         in.Note,
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	return st.CreateContactsBatch(contacts)
}

// Confirm 将接触关系确认为纳入偏序（pending/conflict → confirmed）。
func Confirm(st *store.Store, id string) (*model.Contact, error) {
	c, err := st.GetContact(id)
	if err != nil {
		return nil, err
	}
	if c.Status == model.ContactStatusExcluded {
		return nil, model.ErrInvalidStatus
	}
	if err := st.UpdateContactStatus(id, model.ContactStatusConfirmed); err != nil {
		return nil, err
	}
	c.Status = model.ContactStatusConfirmed
	c.Version++
	return c, nil
}

// Reject 否决误连，将接触关系排除（不再参与偏序求解）。
func Reject(st *store.Store, id string) (*model.Contact, error) {
	c, err := st.GetContact(id)
	if err != nil {
		return nil, err
	}
	if err := st.UpdateContactStatus(id, model.ContactStatusExcluded); err != nil {
		return nil, err
	}
	c.Status = model.ContactStatusExcluded
	c.Version++
	return c, nil
}
