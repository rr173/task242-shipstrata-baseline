// Package sample 负责采样点（sample）证据的管理。
//
// 采样点是地层单元的物理证据来源（如沉积物、构件标本）。每个采样点归属一个
// 地层单元，并携带深度、材料与采集时间，用于支撑层位关系的复核。
package sample

import (
	"context"
	"fmt"
	"math"
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

// Service 采样点服务。
type Service struct {
	store *store.Store
}

// NewService 构造采样点服务。
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// CreateInput 创建采样点的输入。
type CreateInput struct {
	UnitID   string  `json:"unit_id"`
	Label    string  `json:"label"`
	Depth    float64 `json:"depth"`
	Material string  `json:"material"`
	Note     string  `json:"note"`
}

// Create 为某地层单元创建一个采样点。
func (s *Service) Create(ctx context.Context, siteID string, in CreateInput) (model.SamplePoint, error) {
	if in.UnitID == "" || in.Label == "" {
		return model.SamplePoint{}, fmt.Errorf("%w: unit_id and label required", model.ErrInvalidArgument)
	}
	if err := s.store.EnsureSiteWritable(siteID); err != nil {
		return model.SamplePoint{}, err
	}
	u, err := s.store.GetUnit(in.UnitID)
	if err != nil {
		return model.SamplePoint{}, fmt.Errorf("get unit: %w", err)
	}
	if u.SiteID != siteID {
		return model.SamplePoint{}, fmt.Errorf("%w: unit does not belong to site", model.ErrUnknownUnit)
	}
	if math.IsNaN(in.Depth) || math.IsInf(in.Depth, 0) || in.Depth < u.DepthMin || in.Depth > u.DepthMax {
		return model.SamplePoint{}, model.ErrSampleOutOfBounds
	}
	now := time.Now().UTC()
	sp := model.SamplePoint{
		ID:          model.NewID("smp"),
		SiteID:      siteID,
		UnitID:      in.UnitID,
		Label:       in.Label,
		Depth:       in.Depth,
		Material:    in.Material,
		CollectedAt: now,
		Note:        in.Note,
		CreatedAt:   now,
	}
	if err := s.store.CreateSample(&sp); err != nil {
		return model.SamplePoint{}, fmt.Errorf("create sample: %w", err)
	}
	return sp, nil
}

// ListByUnit 返回某单元的全部采样点。
func (s *Service) ListByUnit(ctx context.Context, unitID string) ([]model.SamplePoint, error) {
	u, err := s.store.GetUnit(unitID)
	if err != nil {
		return nil, fmt.Errorf("get unit: %w", err)
	}
	all, err := s.store.ListSamples(u.SiteID)
	if err != nil {
		return nil, fmt.Errorf("list samples: %w", err)
	}
	var out []model.SamplePoint
	for _, sp := range all {
		if sp.UnitID == unitID {
			out = append(out, sp)
		}
	}
	return out, nil
}

// List 返回某遗址的全部采样点。
func (s *Service) List(ctx context.Context, siteID string) ([]model.SamplePoint, error) {
	return s.store.ListSamples(siteID)
}

// Delete 删除采样点。
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.DeleteSample(id)
}
