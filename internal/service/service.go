// Package service 是业务编排层，聚合 store 与各业务包（site/contact 以包函数形式
// 暴露，strata/intrusion/profile/sample 以 Service 形式暴露），向上层（httpapi、
// main）提供统一入口。该层不含 HTTP 细节，也不直接操作数据库，仅做业务组合。
package service

import (
	"task242-shipstrata/internal/intrusion"
	"task242-shipstrata/internal/profile"
	"task242-shipstrata/internal/sample"
	"task242-shipstrata/internal/store"
	"task242-shipstrata/internal/strata"
)

// Services 持有 store 与全部业务服务实例。
type Services struct {
	Store     *store.Store
	Strata    *strata.Service
	Intrusion *intrusion.Service
	Profile   *profile.Service
	Sample    *sample.Service
}

// New 基于已初始化的 store 构造全部业务服务。
func New(s *store.Store) *Services {
	strSvc := strata.NewService(s)
	return &Services{
		Store:     s,
		Strata:    strSvc,
		Intrusion: intrusion.NewService(s, strSvc),
		Profile:   profile.NewService(s, strSvc),
		Sample:    sample.NewService(s),
	}
}
