// Package strata 提供层位偏序求解的业务服务层，聚合 solver 与 store：
// 读取活动接触关系、求解偏序、并将矛盾与侵扰候选持久化（reconcile）。
package strata

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

// Service 层位服务。
type Service struct {
	store *store.Store
	mu    sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	contacts, err := s.store.ListActiveContacts(siteID)
	if err != nil {
		return SolveView{}, fmt.Errorf("list active contacts: %w", err)
	}
	res := Solve(contacts)

	// 1) 依据求解结果完整投影接触状态，清理已经失效的 conflict 派生标记。
	for _, c := range contacts {
		if res.Status[c.ID] == model.ContactStatusConflict {
			if err := s.store.UpdateContactStatus(c.ID, model.ContactStatusConflict); err != nil {
				return SolveView{}, fmt.Errorf("mark conflict: %w", err)
			}
		} else if c.Status == model.ContactStatusConflict && res.Status[c.ID] == model.ContactStatusPending {
			if err := s.store.RestoreContactConfirmation(c.ID); err != nil {
				return SolveView{}, fmt.Errorf("restore confirmed contact: %w", err)
			}
		}
	}

	// 2) 保留矛盾审计历史：消失的当前矛盾标记 resolved，仍存在的不要重复插入。
	existing, err := s.store.ListContradictions(siteID)
	if err != nil {
		return SolveView{}, fmt.Errorf("list contradictions: %w", err)
	}
	current := make(map[string]bool, len(res.Contradictions))
	for _, c := range res.Contradictions {
		cu, _ := json.Marshal(c.CycleUnits)
		current[string(cu)] = true
	}
	for _, old := range existing {
		if !old.Resolved && !current[old.CycleUnits] {
			if err := s.store.ResolveContradiction(old.ID, "cycle removed after contact adjudication"); err != nil {
				return SolveView{}, fmt.Errorf("resolve contradiction: %w", err)
			}
		}
	}
	for _, c := range res.Contradictions {
		cu, _ := json.Marshal(c.CycleUnits)
		ic, _ := json.Marshal(c.InvolvedContacts)
		duplicate := false
		for _, old := range existing {
			if !old.Resolved && old.CycleUnits == string(cu) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
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
