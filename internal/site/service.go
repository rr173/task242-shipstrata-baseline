// Package site 负责遗址批次的创建与状态机流转。
package site

import (
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

// Create 创建处于 collecting 状态的遗址批次。
func Create(st *store.Store, code, name, location string) (*model.SiteBatch, error) {
	now := time.Now().UTC()
	sb := &model.SiteBatch{
		ID:        model.NewID("sb"),
		Code:      code,
		Name:      name,
		Location:  location,
		Status:    model.SiteStatusCollecting,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.CreateSite(sb); err != nil {
		return nil, err
	}
	return sb, nil
}

func canTransition(from, to string) bool {
	switch from {
	case model.SiteStatusCollecting:
		return to == model.SiteStatusPendingReview
	case model.SiteStatusPendingReview:
		return to == model.SiteStatusPublished
	case model.SiteStatusPublished:
		return to == model.SiteStatusSealed
	}
	return false
}

// StartReview collecting → pending_review
func StartReview(st *store.Store, id string) (*model.SiteBatch, error) {
	sb, err := st.GetSite(id)
	if err != nil {
		return nil, err
	}
	if sb.Status == model.SiteStatusSealed {
		return nil, model.ErrSiteSealed
	}
	if !canTransition(sb.Status, model.SiteStatusPendingReview) {
		return nil, model.ErrInvalidStatus
	}
	if err := st.UpdateSiteStatus(id, model.SiteStatusPendingReview, false); err != nil {
		return nil, err
	}
	sb.Status = model.SiteStatusPendingReview
	sb.Version++
	return sb, nil
}

// Publish pending_review → published
func Publish(st *store.Store, id string) (*model.SiteBatch, error) {
	sb, err := st.GetSite(id)
	if err != nil {
		return nil, err
	}
	if sb.Status == model.SiteStatusSealed {
		return nil, model.ErrSiteSealed
	}
	if !canTransition(sb.Status, model.SiteStatusPublished) {
		return nil, model.ErrInvalidStatus
	}
	if err := st.UpdateSiteStatus(id, model.SiteStatusPublished, false); err != nil {
		return nil, err
	}
	sb.Status = model.SiteStatusPublished
	sb.Version++
	return sb, nil
}

// Seal published → sealed（封存后禁止修改）
func Seal(st *store.Store, id string) (*model.SiteBatch, error) {
	sb, err := st.GetSite(id)
	if err != nil {
		return nil, err
	}
	if !canTransition(sb.Status, model.SiteStatusSealed) {
		return nil, model.ErrInvalidStatus
	}
	if err := st.UpdateSiteStatus(id, model.SiteStatusSealed, true); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sb.Status = model.SiteStatusSealed
	sb.Version++
	sb.SealedAt = &now
	return sb, nil
}

// CreateUnit 在遗址下创建一个处于 candidate 状态的地层单元（构件 / 沉积层）。
func CreateUnit(st *store.Store, siteID, label, category string, depthMin, depthMax float64, note string) (*model.StrataUnit, error) {
	if err := st.EnsureSiteWritable(siteID); err != nil {
		return nil, err
	}
	if category != model.UnitCategorySediment && category != model.UnitCategoryComponent {
		category = model.UnitCategorySediment
	}
	now := time.Now().UTC()
	u := &model.StrataUnit{
		ID:        model.NewID("u"),
		SiteID:    siteID,
		Label:     label,
		Category:  category,
		DepthMin:  depthMin,
		DepthMax:  depthMax,
		Status:    model.UnitStatusCandidate,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.CreateUnit(u); err != nil {
		return nil, err
	}
	return u, nil
}
