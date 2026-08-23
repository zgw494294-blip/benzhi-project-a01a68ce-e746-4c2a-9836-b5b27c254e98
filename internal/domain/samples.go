package domain

import (
	"fmt"
	"strings"
	"time"
)

type SampleInput struct {
	ID           string `json:"id"`
	InternalCode string `json:"internalCode"`
	DisplayName  string `json:"displayName"`
	BatchRef     string `json:"batchRef"`
	Notes        string `json:"notes"`
}

// AddSamplesBatch validates the complete request before changing the aggregate.
func (s *TastingSession) AddSamplesBatch(inputs []SampleInput, actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	if len(inputs) < 2 {
		return NewDetailedError(CodeValidation, "批量样品校验失败", []ErrorDetail{{Field: "samples", Message: "批量登记至少需要两个样品"}})
	}
	seenID := map[string]int{}
	seenCode := map[string]int{}
	seenName := map[string]int{}
	details := make([]ErrorDetail, 0)
	conflict := false
	for i, raw := range inputs {
		in := raw
		in.ID, in.InternalCode, in.DisplayName = strings.TrimSpace(in.ID), strings.TrimSpace(in.InternalCode), strings.TrimSpace(in.DisplayName)
		prefix := fmt.Sprintf("samples[%d]", i)
		if in.ID == "" {
			details = append(details, ErrorDetail{Field: prefix + ".id", Message: "样品 ID 不能为空"})
		}
		if in.InternalCode == "" {
			details = append(details, ErrorDetail{Field: prefix + ".internalCode", Message: "内部编码不能为空"})
		}
		if in.DisplayName == "" {
			details = append(details, ErrorDetail{Field: prefix + ".displayName", Message: "显示名称不能为空"})
		}
		code := strings.ToLower(in.InternalCode)
		name := strings.ToLower(in.DisplayName)
		if j, ok := seenID[in.ID]; in.ID != "" && ok {
			details = append(details, ErrorDetail{Field: prefix + ".id", Message: fmt.Sprintf("与 samples[%d].id 重复", j)})
			conflict = true
		}
		if j, ok := seenCode[code]; code != "" && ok {
			details = append(details, ErrorDetail{Field: prefix + ".internalCode", Message: fmt.Sprintf("与 samples[%d].internalCode 重复", j)})
			conflict = true
		}
		if j, ok := seenName[name]; name != "" && ok {
			details = append(details, ErrorDetail{Field: prefix + ".displayName", Message: fmt.Sprintf("与 samples[%d].displayName 重复", j)})
			conflict = true
		}
		seenID[in.ID], seenCode[code], seenName[name] = i, i, i
		for _, existing := range s.Samples {
			if in.ID != "" && existing.ID == in.ID {
				details = append(details, ErrorDetail{Field: prefix + ".id", Message: "样品 ID 已存在"})
				conflict = true
			}
			if in.InternalCode != "" && strings.EqualFold(strings.TrimSpace(existing.InternalCode), in.InternalCode) {
				details = append(details, ErrorDetail{Field: prefix + ".internalCode", Message: "内部编码已存在"})
				conflict = true
			}
			if in.DisplayName != "" && strings.EqualFold(strings.TrimSpace(existing.DisplayName), in.DisplayName) {
				details = append(details, ErrorDetail{Field: prefix + ".displayName", Message: "显示名称已存在"})
				conflict = true
			}
		}
	}
	if len(details) > 0 {
		code := CodeValidation
		if conflict {
			code = CodeConflict
		}
		return NewDetailedError(code, "批量样品校验失败", details)
	}
	ids := make([]string, 0, len(inputs))
	for _, raw := range inputs {
		in := raw
		in.ID, in.InternalCode, in.DisplayName = strings.TrimSpace(in.ID), strings.TrimSpace(in.InternalCode), strings.TrimSpace(in.DisplayName)
		s.Samples = append(s.Samples, Sample{ID: in.ID, SessionID: s.ID, InternalCode: in.InternalCode, DisplayName: in.DisplayName, BatchRef: strings.TrimSpace(in.BatchRef), Notes: strings.TrimSpace(in.Notes), CreatedAt: now.UTC()})
		ids = append(ids, in.ID)
	}
	s.touch(actor, "samples_batch_added", map[string]string{"count": fmt.Sprintf("%d", len(ids)), "sampleIDs": strings.Join(ids, ",")}, now)
	return nil
}

func (s *TastingSession) AddSample(in SampleInput, actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	in.ID = strings.TrimSpace(in.ID)
	in.InternalCode = strings.TrimSpace(in.InternalCode)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.ID == "" || in.InternalCode == "" || in.DisplayName == "" {
		return NewError(CodeValidation, "样品 ID、内部编码和名称不能为空")
	}
	for _, sample := range s.Samples {
		if sample.ID == in.ID || strings.EqualFold(sample.InternalCode, in.InternalCode) {
			return NewError(CodeConflict, "样品 ID 或内部编码重复")
		}
	}
	s.Samples = append(s.Samples, Sample{ID: in.ID, SessionID: s.ID, InternalCode: in.InternalCode, DisplayName: in.DisplayName, BatchRef: strings.TrimSpace(in.BatchRef), Notes: strings.TrimSpace(in.Notes), CreatedAt: now.UTC()})
	s.touch(actor, "sample_added", map[string]string{"sampleID": in.ID}, now)
	return nil
}

func (s *TastingSession) RemoveSample(sampleID, actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	for i := range s.Samples {
		if s.Samples[i].ID == sampleID {
			s.Samples = append(s.Samples[:i], s.Samples[i+1:]...)
			s.touch(actor, "sample_removed", map[string]string{"sampleID": sampleID}, now)
			return nil
		}
	}
	return NewError(CodeNotFound, "样品不存在")
}

func (s *TastingSession) ReplaceSample(sampleID string, in SampleInput, actor string, now time.Time) error {
	if err := s.ensureMutableDraft(); err != nil {
		return err
	}
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	for i := range s.Samples {
		if s.Samples[i].ID != sampleID {
			continue
		}
		for j := range s.Samples {
			if i != j && strings.EqualFold(s.Samples[j].InternalCode, strings.TrimSpace(in.InternalCode)) {
				return NewError(CodeConflict, "样品内部编码重复")
			}
		}
		if strings.TrimSpace(in.InternalCode) == "" || strings.TrimSpace(in.DisplayName) == "" {
			return NewError(CodeValidation, "样品内部编码和名称不能为空")
		}
		s.Samples[i].InternalCode = strings.TrimSpace(in.InternalCode)
		s.Samples[i].DisplayName = strings.TrimSpace(in.DisplayName)
		s.Samples[i].BatchRef = strings.TrimSpace(in.BatchRef)
		s.Samples[i].Notes = strings.TrimSpace(in.Notes)
		s.touch(actor, "sample_updated", map[string]string{"sampleID": sampleID}, now)
		return nil
	}
	return NewError(CodeNotFound, "样品不存在")
}
