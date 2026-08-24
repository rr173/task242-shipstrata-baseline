// Package strata 提供层位偏序求解的业务服务层，聚合 solver 与 store：
// 读取活动接触关系、求解偏序、并将矛盾与侵扰候选持久化（reconcile）。
package strata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

// Service 层位服务。
type Service struct {
	store *store.Store
}

// NewService 构造层位服务。
func NewService(s *store.Store) *Service {
	return &Service{store: s}
}

// Solve 基于某遗址的活动接触关系求解层位偏序。
func (s *Service) Solve(ctx context.Context, siteID string) (SolveView, error) {
	contacts, err := s.store.ListActiveContacts(siteID)
	if err != nil {
		return SolveView{}, fmt.Errorf("list active contacts: %w", err)
	}
	res := Solve(contacts)
	return SolveView{
		Status:         res.Status,
		Cycles:         res.Cycles,
		Contradictions: res.Contradictions,
		Intrusions:     res.Intrusions,
		IsConsistent:   len(res.Contradictions) == 0,
	}, nil
}

// SolveView 偏序求解结果的 API 友好视图。
type SolveView struct {
	Status         map[string]string    `json:"status"`
	Cycles         [][]string           `json:"cycles"`
	Contradictions []ContradictionInput `json:"contradictions"`
	Intrusions     []IntrusionInput     `json:"intrusions"`
	IsConsistent   bool                 `json:"is_consistent"`
}

// Reconcile 重新求解并持久化：将矛盾接触置为 conflict、写入矛盾记录与侵扰候选。
// 幂等：先删除未解决矛盾与 open 侵扰候选，再据最新求解结果重建。
func (s *Service) Reconcile(ctx context.Context, siteID string) (SolveView, error) {
	contacts, err := s.store.ListActiveContacts(siteID)
	if err != nil {
		return SolveView{}, fmt.Errorf("list active contacts: %w", err)
	}
	res := Solve(contacts)

	// 1) 依据求解结果修正接触状态（conflict 标记矛盾边）
	for cid, st := range res.Status {
		if st == model.ContactStatusConflict {
			if err := s.store.UpdateContactStatus(cid, model.ContactStatusConflict); err != nil {
				return SolveView{}, fmt.Errorf("mark conflict: %w", err)
			}
		}
	}

	// 2) 刷新矛盾记录
	if err := s.store.DeleteUnresolvedContradictions(siteID); err != nil {
		return SolveView{}, fmt.Errorf("delete contradictions: %w", err)
	}
	for _, c := range res.Contradictions {
		cu, _ := json.Marshal(c.CycleUnits)
		ic, _ := json.Marshal(c.InvolvedContacts)
		rec := &model.Contradiction{
			ID:               model.NewID("cd"),
			SiteID:           siteID,
			CycleUnits:       string(cu),
			InvolvedContacts: string(ic),
			Resolved:         false,
			Resolution:       "",
			DetectedAt:       time.Now().UTC(),
		}
		if err := s.store.CreateContradiction(rec); err != nil {
			return SolveView{}, fmt.Errorf("create contradiction: %w", err)
		}
	}

	// 3) 刷新侵扰候选（仅 open 状态被重建）
	if err := s.store.DeleteOpenIntrusionCandidates(siteID); err != nil {
		return SolveView{}, fmt.Errorf("delete intrusions: %w", err)
	}
	for _, in := range res.Intrusions {
		sc, _ := json.Marshal(in.SupportingContacts)
		rec := &model.IntrusionCandidate{
			ID:                 model.NewID("ic"),
			SiteID:             siteID,
			UnitID:             in.UnitID,
			Reason:             in.Reason,
			SupportingContacts: string(sc),
			Status:             model.IntrusionOpen,
			CreatedAt:          time.Now().UTC(),
		}
		if err := s.store.CreateIntrusionCandidate(rec); err != nil {
			return SolveView{}, fmt.Errorf("create intrusion: %w", err)
		}
	}

	return SolveView{
		Status:         res.Status,
		Cycles:         res.Cycles,
		Contradictions: res.Contradictions,
		Intrusions:     res.Intrusions,
		IsConsistent:   len(res.Contradictions) == 0,
	}, nil
}
