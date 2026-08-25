// Package httpapi 暴露 HTTP 接口，所有路由前缀为 /api。本层只做协议转换
// （JSON 编解码、路径解析）与调用 service / 业务包，不含业务逻辑。
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"task242-shipstrata/internal/contact"
	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/profile"
	"task242-shipstrata/internal/sample"
	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/site"
)

// Server HTTP 服务。
type Server struct {
	svc *service.Services
}

// NewServer 构造 HTTP 服务。
func NewServer(svc *service.Services) *Server {
	return &Server{svc: svc}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

// lastSeg 取路径最后一段（实体 ID）。
func lastSeg(r *http.Request) string {
	p := strings.TrimSuffix(r.URL.Path, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[idx+1:]
}

// Router 返回配置好的路由。
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// 遗址批次
	mux.HandleFunc("POST /api/sites", s.createSite)
	mux.HandleFunc("GET /api/sites", s.listSites)
	mux.HandleFunc("GET /api/sites/", s.siteDispatch)
	mux.HandleFunc("POST /api/sites/", s.siteDispatch)
	// 单元
	mux.HandleFunc("POST /api/units", s.createUnit)
	mux.HandleFunc("GET /api/units", s.listUnits)
	mux.HandleFunc("GET /api/units/", s.getUnit)
	// 接触关系
	mux.HandleFunc("POST /api/contacts", s.createContact)
	mux.HandleFunc("POST /api/contacts/batch", s.batchContacts)
	mux.HandleFunc("GET /api/contacts", s.listContacts)
	mux.HandleFunc("GET /api/contacts/", s.contactDispatch)
	mux.HandleFunc("POST /api/contacts/", s.contactDispatch)
	// 矛盾 / 侵扰
	mux.HandleFunc("GET /api/contradictions", s.listContradictions)
	mux.HandleFunc("POST /api/intrusion/detect", s.detectIntrusion)
	mux.HandleFunc("GET /api/intrusion/candidates", s.listIntrusion)
	mux.HandleFunc("POST /api/intrusion/", s.adjudicateIntrusion)
	// 剖面
	mux.HandleFunc("POST /api/profiles", s.publishProfile)
	mux.HandleFunc("GET /api/profiles", s.listProfiles)
	mux.HandleFunc("GET /api/profiles/", s.profileDispatch)
	mux.HandleFunc("POST /api/profiles/", s.profileDispatch)
	// 采样点
	mux.HandleFunc("POST /api/samples", s.createSample)
	mux.HandleFunc("GET /api/samples", s.listSamples)
	// 图 / 统计 / 自检
	mux.HandleFunc("GET /api/graph", s.graph)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/self-check", s.selfCheck)

	return mux
}

// ---- 站点 ----

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Location string `json:"location"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if in.Code == "" || in.Name == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	sb, err := site.Create(s.svc.Store, in.Code, in.Name, in.Location)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, sb)
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	bs, err := s.svc.Store.ListSites()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

func (s *Server) siteDispatch(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if strings.HasSuffix(p, "/advance") {
		s.advanceSite(w, r, lastSegStrip(p, "/advance"))
		return
	}
	if strings.HasSuffix(p, "/seal") {
		s.sealSite(w, r, lastSegStrip(p, "/seal"))
		return
	}
	id := lastSeg(r)
	sb, err := s.svc.Store.GetSite(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, sb)
}

// lastSegStrip 去掉后缀并返回倒数第二段（站点 ID）。
func lastSegStrip(p, suffix string) string {
	base := strings.TrimSuffix(p, "/")
	base = strings.TrimSuffix(base, suffix)
	idx := strings.LastIndex(base, "/")
	if idx < 0 {
		return ""
	}
	return base[idx+1:]
}

func (s *Server) advanceSite(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, model.ErrInvalidStatus)
		return
	}
	sb, err := s.svc.Store.GetSite(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	var out *model.SiteBatch
	switch sb.Status {
	case model.SiteStatusCollecting:
		out, err = site.StartReview(s.svc.Store, id)
	case model.SiteStatusPendingReview:
		out, err = site.Publish(s.svc.Store, id)
	default:
		writeErr(w, http.StatusBadRequest, model.ErrInvalidStatus)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) sealSite(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, model.ErrInvalidStatus)
		return
	}
	sb, err := site.Seal(s.svc.Store, id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sb)
}

// ---- 单元 ----

func (s *Server) createUnit(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	var in struct {
		Label    string  `json:"label"`
		Category string  `json:"category"`
		DepthMin float64 `json:"depth_min"`
		DepthMax float64 `json:"depth_max"`
		Note     string  `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if in.Label == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	u, err := site.CreateUnit(s.svc.Store, siteID, in.Label, in.Category, in.DepthMin, in.DepthMax, in.Note)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) listUnits(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	us, err := s.svc.Store.ListUnits(siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, us)
}

func (s *Server) getUnit(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.Store.GetUnit(lastSeg(r))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- 接触 ----

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	var in struct {
		FromUnitID   string `json:"from_unit_id"`
		ToUnitID     string `json:"to_unit_id"`
		Relation     string `json:"relation"`
		SurveySource string `json:"survey_source"`
		SurveySeq    int    `json:"survey_seq"`
		Note         string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	added, c, err := contact.Import(s.svc.Store, siteID, in.FromUnitID, in.ToUnitID, in.Relation, in.SurveySource, in.SurveySeq, in.Note)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"contact": c, "added": added})
}

func (s *Server) batchContacts(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	var in struct {
		Contacts []struct {
			FromUnitID   string `json:"from_unit_id"`
			ToUnitID     string `json:"to_unit_id"`
			Relation     string `json:"relation"`
			SurveySource string `json:"survey_source"`
			SurveySeq    int    `json:"survey_seq"`
			Note         string `json:"note"`
		} `json:"contacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	inputs := make([]contact.ImportInput, 0, len(in.Contacts))
	for _, c := range in.Contacts {
		inputs = append(inputs, contact.ImportInput{
			FromUnitID: c.FromUnitID, ToUnitID: c.ToUnitID, Relation: c.Relation,
			SurveySource: c.SurveySource, SurveySeq: c.SurveySeq, Note: c.Note,
		})
	}
	added, skipped := 0, 0
	for _, in := range inputs {
		a, _, err := contact.Import(s.svc.Store, siteID, in.FromUnitID, in.ToUnitID, in.Relation, in.SurveySource, in.SurveySeq, in.Note)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if a {
			added++
		} else {
			skipped++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "skipped": skipped})
}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	cs, err := s.svc.Store.ListContacts(siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) contactDispatch(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if strings.HasSuffix(p, "/confirm") {
		s.confirmContact(w, r, lastSegStrip(p, "/confirm"))
		return
	}
	if strings.HasSuffix(p, "/reject") {
		s.rejectContact(w, r, lastSegStrip(p, "/reject"))
		return
	}
	c, err := s.svc.Store.GetContact(lastSeg(r))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) confirmContact(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, model.ErrInvalidStatus)
		return
	}
	c, err := contact.Confirm(s.svc.Store, id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// 确认后重新求解，刷新矛盾与侵扰候选
	if _, err := s.svc.Strata.Reconcile(r.Context(), c.SiteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) rejectContact(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, model.ErrInvalidStatus)
		return
	}
	c, err := contact.Reject(s.svc.Store, id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.svc.Strata.Reconcile(r.Context(), c.SiteID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ---- 矛盾 / 侵扰 ----

func (s *Server) listContradictions(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	cs, err := s.svc.Store.ListContradictions(siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (s *Server) detectIntrusion(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	cands, err := s.svc.Intrusion.Detect(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) listIntrusion(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	cands, err := s.svc.Intrusion.List(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) adjudicateIntrusion(w http.ResponseWriter, r *http.Request) {
	// 路径形如 /api/intrusion/{unitId}/adjudicate
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[len(parts)-1] != "adjudicate" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	unitID := parts[len(parts)-2]
	var in struct {
		Verdict string `json:"verdict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	u, err := s.svc.Intrusion.Adjudicate(r.Context(), siteID, unitID, in.Verdict)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- 剖面 ----

func (s *Server) publishProfile(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	var in profile.PublishInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p, snap, err := s.svc.Profile.Publish(r.Context(), siteID, in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"profile": p, "snapshot": snap})
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	ps, err := s.svc.Profile.List(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) profileDispatch(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if strings.HasSuffix(p, "/share") {
		id := lastSegStrip(p, "/share")
		out, err := s.svc.Profile.Share(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if strings.HasSuffix(p, "/freeze") {
		id := lastSegStrip(p, "/freeze")
		out, err := s.svc.Profile.Freeze(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	if strings.HasSuffix(p, "/supersede") {
		id := lastSegStrip(p, "/supersede")
		var in struct {
			NewID string `json:"new_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := s.svc.Profile.Supersede(r.Context(), id, in.NewID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"superseded": id, "replaced_by": in.NewID})
		return
	}
	pv, snap, err := s.svc.Profile.Get(r.Context(), lastSeg(r))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": pv, "snapshot": snap})
}

// ---- 采样点 ----

func (s *Server) createSample(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	var in sample.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	sp, err := s.svc.Sample.Create(r.Context(), siteID, in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, sp)
}

func (s *Server) listSamples(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	smps, err := s.svc.Sample.List(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, smps)
}

// ---- 图 / 统计 / 自检 ----

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	if siteID == "" {
		writeErr(w, http.StatusBadRequest, model.ErrInvalidArgument)
		return
	}
	res, err := s.svc.Strata.Solve(r.Context(), siteID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sites, err := s.svc.Store.ListSites()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var units, contacts, profiles, contradictions, intrusions int
	for _, sb := range sites {
		us, _ := s.svc.Store.ListUnits(sb.ID)
		cs, _ := s.svc.Store.ListContacts(sb.ID)
		ps, _ := s.svc.Profile.List(ctx, sb.ID)
		cds, _ := s.svc.Store.ListContradictions(sb.ID)
		ics, _ := s.svc.Store.ListIntrusionCandidates(sb.ID)
		units += len(us)
		contacts += len(cs)
		profiles += len(ps)
		contradictions += len(cds)
		intrusions += len(ics)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_batches":         len(sites),
		"units":                units,
		"contacts":             contacts,
		"profiles":             profiles,
		"contradictions":       contradictions,
		"intrusion_candidates": intrusions,
	})
}

// selfCheck 端到端健康自检：store 可读 + 求解器可运行。
func (s *Server) selfCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if _, err := s.svc.Store.ListSites(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"store": "unhealthy: " + err.Error()})
		return
	}
	sites, _ := s.svc.Store.ListSites()
	consistent := true
	for _, sb := range sites {
		res, err := s.svc.Strata.Solve(ctx, sb.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"solver": "unhealthy: " + err.Error()})
			return
		}
		if !res.IsConsistent {
			consistent = false
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"store":        "ok",
		"solver":       "ok",
		"consistent":   consistent,
		"site_batches": len(sites),
		"ts":           time.Now().UTC().Format(time.RFC3339),
	})
}
