package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type TastingSession struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	ProductCategory  string                `json:"productCategory"`
	HostUserID       string                `json:"hostUserID"`
	ReviewerUserIDs  []string              `json:"reviewerUserIDs"`
	ScheduledAt      time.Time             `json:"scheduledAt"`
	Status           SessionStatus         `json:"status"`
	Version          int64                 `json:"version"`
	Seed             string                `json:"seed"`
	Scales           []ScaleDimension      `json:"scales"`
	Samples          []Sample              `json:"samples"`
	Assignments      []BlindAssignment     `json:"assignments,omitempty"`
	Evaluations      []Evaluation          `json:"evaluations,omitempty"`
	Findings         []VerificationFinding `json:"findings,omitempty"`
	RevealApprovals  []RevealApproval      `json:"revealApprovals,omitempty"`
	Conclusions      []DimensionConclusion `json:"conclusions,omitempty"`
	PlanHash         string                `json:"planHash,omitempty"`
	ArchiveReceiptID string                `json:"archiveReceiptID,omitempty"`
	Audit            []AuditEntry          `json:"audit"`
	CreatedAt        time.Time             `json:"createdAt"`
	UpdatedAt        time.Time             `json:"updatedAt"`
}

type SessionConfig struct {
	Name            string           `json:"name"`
	ProductCategory string           `json:"productCategory"`
	HostUserID      string           `json:"hostUserID"`
	ReviewerUserIDs []string         `json:"reviewerUserIDs"`
	ScheduledAt     time.Time        `json:"scheduledAt"`
	Seed            string           `json:"seed"`
	Scales          []ScaleDimension `json:"scales"`
}

func NewSession(id string, cfg SessionConfig, now time.Time) (*TastingSession, error) {
	s := &TastingSession{ID: strings.TrimSpace(id), Status: StatusDraft, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	if s.ID == "" {
		return nil, NewError(CodeValidation, "会话 ID 不能为空")
	}
	// Preserve the original create payload, which allowed a single omitted order;
	// subsequent draft replacements require explicit, unique ordering.
	if len(cfg.Scales) > 0 {
		allMissing := true
		for _, scale := range cfg.Scales {
			if scale.Order != 0 {
				allMissing = false
				break
			}
		}
		if allMissing {
			cfg.Scales = append([]ScaleDimension(nil), cfg.Scales...)
			for i := range cfg.Scales {
				cfg.Scales[i].Order = i + 1
			}
		}
	}
	if err := s.Configure(cfg, "system", now); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *TastingSession) Configure(cfg SessionConfig, actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if s.HostUserID != "" {
		if err := s.RequireHost(actor); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.Name) == "" || strings.TrimSpace(cfg.ProductCategory) == "" {
		return NewError(CodeValidation, "会话名称和产品类别不能为空")
	}
	if strings.TrimSpace(cfg.HostUserID) == "" {
		return NewError(CodeValidation, "主持人不能为空")
	}
	if len(cfg.ReviewerUserIDs) < 2 {
		return NewError(CodeValidation, "至少需要两名评审员")
	}
	seen := map[string]bool{}
	reviewers := make([]string, 0, len(cfg.ReviewerUserIDs))
	for _, raw := range cfg.ReviewerUserIDs {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			return NewError(CodeValidation, "评审员不能为空或重复")
		}
		if id == cfg.HostUserID {
			return NewError(CodeValidation, "主持人不能同时作为评审员")
		}
		seen[id] = true
		reviewers = append(reviewers, id)
	}
	if len(cfg.Scales) == 0 {
		return NewError(CodeValidation, "至少需要一个量表维度")
	}
	scales := append([]ScaleDimension(nil), cfg.Scales...)
	sort.Slice(scales, func(i, j int) bool { return scales[i].Order < scales[j].Order })
	keys := map[string]bool{}
	orders := map[int]bool{}
	for i := range scales {
		scales[i].Key = strings.TrimSpace(scales[i].Key)
		scales[i].Name = strings.TrimSpace(scales[i].Name)
		if scales[i].Key == "" || scales[i].Name == "" || keys[scales[i].Key] {
			return NewError(CodeValidation, "量表键和名称不能为空且键不能重复")
		}
		if scales[i].Order <= 0 || orders[scales[i].Order] {
			return NewError(CodeValidation, "量表 %s 的 order 必须为正整数且不能重复", scales[i].Key)
		}
		if math.IsNaN(scales[i].Min) || math.IsNaN(scales[i].Max) || math.IsInf(scales[i].Min, 0) || math.IsInf(scales[i].Max, 0) {
			return NewError(CodeValidation, "量表 %s 的范围必须是有限数值", scales[i].Key)
		}
		if decimalPlaces(scales[i].Min) > 2 || decimalPlaces(scales[i].Max) > 2 {
			return NewError(CodeValidation, "量表 %s 的范围最多保留两位小数", scales[i].Key)
		}
		if scales[i].Max <= scales[i].Min {
			return NewError(CodeValidation, "量表 %s 的最大值必须大于最小值", scales[i].Name)
		}
		keys[scales[i].Key] = true
		orders[scales[i].Order] = true
	}
	s.Name = strings.TrimSpace(cfg.Name)
	s.ProductCategory = strings.TrimSpace(cfg.ProductCategory)
	s.HostUserID = strings.TrimSpace(cfg.HostUserID)
	s.ReviewerUserIDs = reviewers
	s.ScheduledAt = cfg.ScheduledAt.UTC()
	s.Seed = strings.TrimSpace(cfg.Seed)
	if s.Seed == "" {
		s.Seed = s.ID
	}
	s.Scales = scales
	s.touch(actor, "session_configured", map[string]string{"scaleCount": fmt.Sprintf("%d", len(scales))}, now)
	return nil
}

func decimalPlaces(v float64) int {
	for p := 0; p <= 2; p++ {
		scaled := v * math.Pow10(p)
		if math.Abs(scaled-math.Round(scaled)) < 1e-9 {
			return p
		}
	}
	return 3
}

func (s *TastingSession) ensureMutableDraft() error {
	if s.Status == StatusSealed {
		return NewError(CodeState, "会话已封存，禁止修改")
	}
	if s.Status != StatusDraft {
		return NewError(CodeState, "仅草稿状态允许修改配置")
	}
	return nil
}

func (s *TastingSession) RequireHost(actor string) error {
	if actor != s.HostUserID {
		return NewError(CodeForbidden, "该操作仅允许主持人执行")
	}
	return nil
}

func (s *TastingSession) IsReviewer(actor string) bool {
	for _, id := range s.ReviewerUserIDs {
		if id == actor {
			return true
		}
	}
	return false
}

func (s *TastingSession) touch(actor, action string, detail map[string]string, now time.Time) {
	s.UpdatedAt = now.UTC()
	s.Audit = append(s.Audit, AuditEntry{Action: action, Actor: actor, At: now.UTC(), Detail: detail})
}
