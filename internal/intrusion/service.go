// Package intrusion 负责侵扰候选的识别与裁决。
//
// 侵扰（intrusion）指后期活动破坏原始沉积顺序的现象。求解器在检测到不可能循环
// （层位矛盾）时会将环内连接度最高的单元标记为侵扰候选，等待研究者复核裁决。
package intrusion

import (
	"context"
	"fmt"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
	"task242-shipstrata/internal/strata"
)

// Service 侵扰服务。
type Service struct {
	store  *store.Store
	strata *strata.Service
}

// NewService 构造侵扰服务。
func NewService(s *store.Store, st *strata.Service) *Service {
	return &Service{store: s, strata: st}
}

// Detect 触发重新求解并刷新侵扰候选，返回最新候选列表。
func (s *Service) Detect(ctx context.Context, siteID string) ([]model.IntrusionCandidate, error) {
	if _, err := s.strata.Reconcile(ctx, siteID); err != nil {
		return nil, fmt.Errorf("reconcile: %w", err)
	}
	return s.store.ListIntrusionCandidates(siteID)
}

// List 返回当前侵扰候选。
func (s *Service) List(ctx context.Context, siteID string) ([]model.IntrusionCandidate, error) {
	return s.store.ListIntrusionCandidates(siteID)
}

// Adjudicate 研究者对侵扰候选作出裁决：
//   - accepted：确认该单元为后期侵扰层（disturbed），候选置 accepted。
//   - dismissed：排除该侵扰嫌疑（excluded），候选置 dismissed。
//
// 裁决后重新求解，保证层位偏序与候选集合一致。
func (s *Service) Adjudicate(ctx context.Context, siteID, unitID, verdict string) (model.StrataUnit, error) {
	if verdict != "accepted" && verdict != "dismissed" {
		return model.StrataUnit{}, fmt.Errorf("%w: verdict must be accepted or dismissed", model.ErrInvalidArgument)
	}
	u, err := s.store.GetUnit(unitID)
	if err != nil {
		return model.StrataUnit{}, fmt.Errorf("get unit: %w", err)
	}
	switch verdict {
	case "accepted":
		if err := s.store.UpdateUnitStatus(unitID, model.UnitStatusDisturbed); err != nil {
			return model.StrataUnit{}, fmt.Errorf("mark disturbed: %w", err)
		}
		u.Status = model.UnitStatusDisturbed
		if id := intrusionIDForUnit(s.store, unitID); id != "" {
			if err := s.store.UpdateIntrusionStatus(id, model.IntrusionAccepted); err != nil {
				return model.StrataUnit{}, fmt.Errorf("resolve candidate: %w", err)
			}
		}
	case "dismissed":
		if err := s.store.UpdateUnitStatus(unitID, model.UnitStatusExcluded); err != nil {
			return model.StrataUnit{}, fmt.Errorf("mark excluded: %w", err)
		}
		u.Status = model.UnitStatusExcluded
		if id := intrusionIDForUnit(s.store, unitID); id != "" {
			if err := s.store.UpdateIntrusionStatus(id, model.IntrusionDismissed); err != nil {
				return model.StrataUnit{}, fmt.Errorf("resolve candidate: %w", err)
			}
		}
	}
	// 裁决后刷新候选集合（被排除/扰动单元可能影响其它候选）
	if _, err := s.strata.Reconcile(ctx, siteID); err != nil {
		return model.StrataUnit{}, fmt.Errorf("reconcile: %w", err)
	}
	return *u, nil
}

// intrusionIDForUnit 找到该单元当前 open 的侵扰候选 ID（用于状态置位）。
func intrusionIDForUnit(st *store.Store, unitID string) string {
	cands, err := st.ListIntrusionCandidates("")
	if err != nil {
		return ""
	}
	for _, c := range cands {
		if c.UnitID == unitID && c.Status == model.IntrusionOpen {
			return c.ID
		}
	}
	return ""
}
