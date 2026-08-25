// Package profile 负责剖面版本（profile）的发布、共享、冻结与替代。
//
// 剖面版本是层位关系复核的阶段性结论快照。frozen 之后的版本不可变，保留原始
// 裁决结果；后续修订通过发布新版本并 supersede 旧版本来表达，保证可追溯。
package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
	"task242-shipstrata/internal/strata"
)

// Service 剖面服务。
type Service struct {
	store  *store.Store
	strata *strata.Service
}

// NewService 构造剖面服务。
func NewService(s *store.Store, st *strata.Service) *Service {
	return &Service{store: s, strata: st}
}

// PublishInput 发布剖面版本的输入。
type PublishInput struct {
	Label       string `json:"label"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

// Snapshot 剖面快照内容（层位偏序 + 实体状态）。
type Snapshot struct {
	Order          map[string]string           `json:"order"`
	Cycles         [][]string                  `json:"cycles"`
	Contradictions []strata.ContradictionInput `json:"contradictions"`
	Units          []model.StrataUnit          `json:"units"`
	Contacts       []model.Contact             `json:"contacts"`
	GeneratedAt    string                      `json:"generated_at"`
}

// Publish 基于当前偏序求解结果发布一个 draft 剖面版本。
func (s *Service) Publish(ctx context.Context, siteID string, in PublishInput) (model.ProfileVersion, Snapshot, error) {
	if in.Label == "" {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("%w: label required", model.ErrInvalidArgument)
	}
	batch, err := s.store.GetSite(siteID)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("get site: %w", err)
	}
	if batch.Status == model.SiteStatusSealed {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("%w: site sealed, cannot publish profile", model.ErrSiteSealed)
	}
	units, err := s.store.ListUnits(siteID)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("list units: %w", err)
	}
	contacts, err := s.store.ListActiveContacts(siteID)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("list contacts: %w", err)
	}
	view, err := s.strata.Solve(ctx, siteID)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("solve: %w", err)
	}
	snap := Snapshot{
		Order:          view.Status,
		Cycles:         view.Cycles,
		Contradictions: view.Contradictions,
		Units:          units,
		Contacts:       contacts,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	blob, err := json.Marshal(snap)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("marshal snapshot: %w", err)
	}
	ver, err := s.store.NextProfileVersion(siteID)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("next version: %w", err)
	}
	p := model.ProfileVersion{
		ID:        model.NewID("prf"),
		SiteID:    siteID,
		Version:   ver,
		Status:    model.ProfileStatusDraft,
		Snapshot:  string(blob),
		Note:      in.Description,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateProfile(&p); err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("create profile: %w", err)
	}
	return p, snap, nil
}

// Share 将 draft 剖面标记为 shared（对外共享但尚可修订）。
func (s *Service) Share(ctx context.Context, id string) (model.ProfileVersion, error) {
	p, err := s.store.GetProfile(id)
	if err != nil {
		return model.ProfileVersion{}, fmt.Errorf("get profile: %w", err)
	}
	if p.Status != model.ProfileStatusDraft {
		return model.ProfileVersion{}, fmt.Errorf("%w: only draft can be shared", model.ErrInvalidStatus)
	}
	if err := s.store.UpdateProfileStatus(id, model.ProfileStatusShared, false); err != nil {
		return model.ProfileVersion{}, fmt.Errorf("update profile: %w", err)
	}
	p.Status = model.ProfileStatusShared
	return *p, nil
}

// Freeze 冻结剖面：进入 frozen 不可变状态，保留原始快照。
func (s *Service) Freeze(ctx context.Context, id string) (model.ProfileVersion, error) {
	p, err := s.store.GetProfile(id)
	if err != nil {
		return model.ProfileVersion{}, fmt.Errorf("get profile: %w", err)
	}
	if p.Status == model.ProfileStatusFrozen || p.Status == model.ProfileStatusSuperseded {
		return model.ProfileVersion{}, fmt.Errorf("%w: profile already %s", model.ErrInvalidStatus, p.Status)
	}
	if p.Status != model.ProfileStatusShared {
		return model.ProfileVersion{}, fmt.Errorf("%w: only shared can be frozen", model.ErrInvalidStatus)
	}
	if err := s.store.UpdateProfileStatus(id, model.ProfileStatusFrozen, true); err != nil {
		return model.ProfileVersion{}, fmt.Errorf("freeze profile: %w", err)
	}
	frozen, err := s.store.GetProfile(id)
	if err != nil {
		return model.ProfileVersion{}, fmt.Errorf("reload frozen profile: %w", err)
	}
	return *frozen, nil
}

// Supersede 用新版本替代旧版本：旧版本标记 superseded。
func (s *Service) Supersede(ctx context.Context, oldID, newID string) error {
	if err := s.store.SupersedeProfile(oldID, newID); err != nil {
		return fmt.Errorf("supersede profile: %w", err)
	}
	return nil
}

// Get 返回剖面与其快照。
func (s *Service) Get(ctx context.Context, id string) (model.ProfileVersion, Snapshot, error) {
	p, err := s.store.GetProfile(id)
	if err != nil {
		return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("get profile: %w", err)
	}
	var snap Snapshot
	if p.Snapshot != "" {
		if err := json.Unmarshal([]byte(p.Snapshot), &snap); err != nil {
			return model.ProfileVersion{}, Snapshot{}, fmt.Errorf("unmarshal snapshot: %w", err)
		}
	}
	return *p, snap, nil
}

// List 返回某遗址下的全部剖面（含被替代的，保证可追溯）。
func (s *Service) List(ctx context.Context, siteID string) ([]model.ProfileVersion, error) {
	return s.store.ListProfiles(siteID)
}
