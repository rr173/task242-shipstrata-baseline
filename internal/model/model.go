// Package model 定义船舶沉没遗址层位关系复核台的核心实体、状态枚举与错误。
package model

import (
	"errors"
	"time"
)

// 遗址批次状态
const (
	SiteStatusCollecting    = "collecting"
	SiteStatusPendingReview = "pending_review"
	SiteStatusPublished     = "published"
	SiteStatusSealed        = "sealed"
)

// 地层单元状态
const (
	UnitStatusCandidate = "candidate"
	UnitStatusStable    = "stable"
	UnitStatusDisturbed = "disturbed"
	UnitStatusExcluded  = "excluded"
)

// 地层单元类别
const (
	UnitCategorySediment  = "sediment"
	UnitCategoryComponent = "component"
)

// 接触关系类型：覆盖与切割均表示「上覆单元晚于下伏单元」
const (
	RelOverlies = "overlies"
	RelCuts     = "cuts"
)

// 接触关系状态机
const (
	ContactStatusPending   = "pending"
	ContactStatusOverlies  = "overlies"
	ContactStatusCuts      = "cuts"
	ContactStatusConflict  = "conflict"
	ContactStatusConfirmed = "confirmed"
	ContactStatusExcluded  = "excluded"
)

// 剖面版本状态机
const (
	ProfileStatusDraft      = "draft"
	ProfileStatusShared     = "shared"
	ProfileStatusFrozen     = "frozen"
	ProfileStatusSuperseded = "superseded"
)

// 侵扰候选状态
const (
	IntrusionOpen      = "open"
	IntrusionAccepted  = "accepted"
	IntrusionDismissed = "dismissed"
)

// 业务错误
var (
	ErrNotFound             = errors.New("entity not found")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrInvalidStatus        = errors.New("invalid status transition")
	ErrSelfLoop             = errors.New("contact self-loop is not allowed")
	ErrUnknownUnit          = errors.New("unknown unit referenced by contact")
	ErrSampleOutOfBounds    = errors.New("sample depth is out of unit depth bounds")
	ErrFrozenImmutable      = errors.New("frozen profile cannot be modified")
	ErrDuplicateFingerprint = errors.New("duplicate survey edge fingerprint")
	ErrVersionConflict      = errors.New("version conflict during adjudication")
	ErrSiteSealed           = errors.New("site is sealed and cannot be modified")
	ErrInvalidRelation      = errors.New("invalid contact relation type")
	ErrNoOpenCandidate      = errors.New("no open intrusion candidate for unit")
)

// SiteBatch 遗址批次
type SiteBatch struct {
	ID        string
	Code      string
	Name      string
	Location  string
	Status    string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
	SealedAt  *time.Time
}

// StrataUnit 地层单元（构件 / 沉积层）
type StrataUnit struct {
	ID        string
	SiteID    string
	Label     string
	Category  string
	DepthMin  float64
	DepthMax  float64
	Status    string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Contact 接触关系（地层单元间的有向边）
type Contact struct {
	ID           string
	SiteID       string
	FromUnitID   string
	ToUnitID     string
	Relation     string // overlies | cuts
	Status       string // pending|overlies|cuts|conflict|confirmed|excluded
	SurveySource string
	SurveySeq    int
	Fingerprint  string
	Note         string
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SamplePoint 采样点（绑定到单元，作为证据）
type SamplePoint struct {
	ID          string
	SiteID      string
	UnitID      string
	Label       string
	Depth       float64
	Material    string
	CollectedAt time.Time
	Note        string
	CreatedAt   time.Time
}

// ProfileVersion 剖面版本（冻结后不可变）
type ProfileVersion struct {
	ID        string
	SiteID    string
	Version   int
	Status    string
	Snapshot  string // JSON 不可变快照
	Note      string
	CreatedAt time.Time
	FrozenAt  *time.Time
}

// Contradiction 不可能循环矛盾
type Contradiction struct {
	ID               string
	SiteID           string
	CycleUnits       string // JSON []string
	InvolvedContacts string // JSON []string
	Resolved         bool
	Resolution       string
	DetectedAt       time.Time
}

// ContactParticipates reports whether a contact is eligible for stratigraphic
// solving. Pending survey evidence must not affect the relation graph.
func ContactParticipates(status string) bool {
	return status == ContactStatusConfirmed || status == ContactStatusConflict
}

// CanSupersedeProfile enforces the immutable profile lifecycle.
func CanSupersedeProfile(oldStatus, newStatus string) bool {
	return (oldStatus == ProfileStatusShared || oldStatus == ProfileStatusFrozen) && newStatus == ProfileStatusDraft
}

// IntrusionCandidate 侵扰候选（疑似后期侵扰层或误连）
type IntrusionCandidate struct {
	ID                 string
	SiteID             string
	UnitID             string
	Reason             string
	SupportingContacts string // JSON []string
	Status             string
	CreatedAt          time.Time
}

// 校验辅助

func validSiteStatus(s string) bool {
	switch s {
	case SiteStatusCollecting, SiteStatusPendingReview, SiteStatusPublished, SiteStatusSealed:
		return true
	}
	return false
}

func validUnitStatus(s string) bool {
	switch s {
	case UnitStatusCandidate, UnitStatusStable, UnitStatusDisturbed, UnitStatusExcluded:
		return true
	}
	return false
}

func validRelation(r string) bool {
	return r == RelOverlies || r == RelCuts
}

func validContactStatus(s string) bool {
	switch s {
	case ContactStatusPending, ContactStatusOverlies, ContactStatusCuts,
		ContactStatusConflict, ContactStatusConfirmed, ContactStatusExcluded:
		return true
	}
	return false
}

func validProfileStatus(s string) bool {
	switch s {
	case ProfileStatusDraft, ProfileStatusShared, ProfileStatusFrozen, ProfileStatusSuperseded:
		return true
	}
	return false
}

// ValidUnitStatus 导出校验
func ValidUnitStatus(s string) bool { return validUnitStatus(s) }

// ValidRelation 导出校验
func ValidRelation(r string) bool { return validRelation(r) }

// ValidContactStatus 导出校验
func ValidContactStatus(s string) bool { return validContactStatus(s) }

// ValidSiteStatus 导出校验
func ValidSiteStatus(s string) bool { return validSiteStatus(s) }

// ValidProfileStatus 导出校验
func ValidProfileStatus(s string) bool { return validProfileStatus(s) }
