package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"time"
)

type planDigestRow struct {
	Reviewer string `json:"reviewer"`
	Sample   string `json:"sample"`
	Code     string `json:"code"`
	Sequence int    `json:"sequence"`
}

func stableSeed(seed string) int64 {
	h := sha256.Sum256([]byte(seed))
	var n int64
	for i := 0; i < 8; i++ {
		n = n<<8 | int64(h[i])
	}
	return n
}

func (s *TastingSession) FreezePlan(actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	if len(s.Samples) < 2 {
		return NewError(CodeValidation, "冻结前至少需要两个样品")
	}
	if len(s.ReviewerUserIDs) < 2 || len(s.Scales) == 0 {
		return NewError(CodeValidation, "会话配置不完整")
	}
	samples := append([]Sample(nil), s.Samples...)
	sort.Slice(samples, func(i, j int) bool { return samples[i].ID < samples[j].ID })
	rows := make([]planDigestRow, 0, len(samples)*len(s.ReviewerUserIDs))
	assignments := make([]BlindAssignment, 0, cap(rows))
	usedCodes := map[string]bool{}
	for reviewerIndex, reviewer := range s.ReviewerUserIDs {
		order := make([]int, len(samples))
		for i := range order {
			order[i] = (i + reviewerIndex) % len(samples)
		}
		rng := rand.New(rand.NewSource(stableSeed(s.Seed + ":" + reviewer)))
		if len(order) > 2 {
			for i := len(order) - 1; i > 1; i-- {
				j := 1 + rng.Intn(i)
				order[i], order[j] = order[j], order[i]
			}
		}
		for sequence, sampleIndex := range order {
			sample := samples[sampleIndex]
			base := 100 + int(stableSeed(s.Seed+reviewer+sample.ID)&0x7fffffff)%900
			code := fmt.Sprintf("%03d", base)
			for usedCodes[reviewer+":"+code] {
				base = (base+137)%900 + 100
				code = fmt.Sprintf("%03d", base)
			}
			usedCodes[reviewer+":"+code] = true
			rows = append(rows, planDigestRow{Reviewer: reviewer, Sample: sample.ID, Code: code, Sequence: sequence + 1})
		}
	}
	encoded, _ := json.Marshal(rows)
	digest := sha256.Sum256(encoded)
	planHash := hex.EncodeToString(digest[:])
	for i, row := range rows {
		assignments = append(assignments, BlindAssignment{ID: "asg-" + strconv.Itoa(i+1), SessionID: s.ID, ReviewerUserID: row.Reviewer, SampleID: row.Sample, BlindCode: row.Code, Sequence: row.Sequence, PlanHash: planHash, FrozenAt: now.UTC()})
	}
	s.Assignments = assignments
	s.PlanHash = planHash
	s.Status = StatusFrozen
	s.touch(actor, "plan_frozen", map[string]string{"planHash": planHash}, now)
	return nil
}

func (s *TastingSession) StartCollection(actor string, now time.Time) error {
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	if s.Status != StatusFrozen {
		return NewError(CodeState, "仅已冻结会话可以启动采集")
	}
	s.Status = StatusCollecting
	s.touch(actor, "collection_started", nil, now)
	return nil
}

func (s *TastingSession) ReviewerAssignments(reviewer string) ([]BlindAssignment, error) {
	if !s.IsReviewer(reviewer) {
		return nil, NewError(CodeForbidden, "用户不是本会话评审员")
	}
	if s.Status != StatusCollecting && s.Status != StatusVerifying {
		return nil, NewError(CodeState, "当前状态不提供评审任务")
	}
	var out []BlindAssignment
	for _, a := range s.Assignments {
		if a.ReviewerUserID == reviewer {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, nil
}
