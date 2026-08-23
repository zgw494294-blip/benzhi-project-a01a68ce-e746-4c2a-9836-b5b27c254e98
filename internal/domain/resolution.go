package domain

import "time"

type FindingResolution struct {
	FindingID  string     `json:"findingID"`
	Resolution Resolution `json:"resolution"`
}

func (s *TastingSession) ResolveFindingsBatch(items []FindingResolution, qualityUser string, now time.Time) error {
	if s.Status != StatusVerifying {
		return NewError(CodeState, "仅核验中会话可以裁定发现")
	}
	if qualityUser == "" || qualityUser == s.HostUserID || s.IsReviewer(qualityUser) {
		return NewError(CodeForbidden, "裁定必须由独立质量复核员执行")
	}
	if len(items) == 0 {
		return NewError(CodeValidation, "至少需要一条裁定")
	}
	seen := map[string]bool{}
	for _, item := range items {
		if item.FindingID == "" || seen[item.FindingID] {
			return NewError(CodeValidation, "裁定列表含空白或重复 findingID")
		}
		seen[item.FindingID] = true
		if item.Resolution != ResolutionAccept && item.Resolution != ResolutionVoid && item.Resolution != ResolutionRework {
			return NewError(CodeValidation, "发现 %s 的裁定类型无效", item.FindingID)
		}
		var finding *VerificationFinding
		for i := range s.Findings {
			if s.Findings[i].ID == item.FindingID {
				finding = &s.Findings[i]
				break
			}
		}
		if finding == nil {
			return NewError(CodeNotFound, "核验发现 %s 不存在", item.FindingID)
		}
		if finding.Status != FindingOpen {
			return NewError(CodeConflict, "核验发现 %s 已裁定", item.FindingID)
		}
		if finding.RuleCode == "missing" && item.Resolution != ResolutionRework {
			return NewError(CodeValidation, "缺失评分只能退回重评")
		}
		if (item.Resolution == ResolutionVoid || item.Resolution == ResolutionRework) && finding.EvaluationID == "" && finding.RuleCode != "missing" {
			return NewError(CodeValidation, "发现 %s 未关联可处理评分", item.FindingID)
		}
	}
	for _, item := range items {
		for i := range s.Findings {
			if s.Findings[i].ID == item.FindingID {
				s.applyResolution(&s.Findings[i], item.Resolution, qualityUser, now)
				break
			}
		}
	}
	return nil
}

func (s *TastingSession) applyResolution(f *VerificationFinding, resolution Resolution, qualityUser string, now time.Time) {
	if resolution == ResolutionVoid || resolution == ResolutionRework {
		for i := range s.Evaluations {
			if s.Evaluations[i].ID == f.EvaluationID {
				if resolution == ResolutionVoid {
					s.Evaluations[i].ValidityStatus = ValidityVoided
				} else {
					at := now.UTC()
					s.Evaluations[i].ValidityStatus, s.Evaluations[i].ReworkBy, s.Evaluations[i].ReworkAt = ValidityRework, qualityUser, &at
				}
				break
			}
		}
	}
	resolvedAt := now.UTC()
	f.Status, f.Resolution, f.ResolvedBy, f.ResolvedAt = FindingResolved, resolution, qualityUser, &resolvedAt
	s.touch(qualityUser, "finding_resolved", map[string]string{"findingID": f.ID, "resolution": string(resolution)}, now)
}

func (s *TastingSession) ResolveFinding(findingID string, resolution Resolution, qualityUser string, now time.Time) error {
	if s.Status != StatusVerifying {
		return NewError(CodeState, "仅核验中会话可以裁定发现")
	}
	if qualityUser == "" || qualityUser == s.HostUserID || s.IsReviewer(qualityUser) {
		return NewError(CodeForbidden, "裁定必须由独立质量复核员执行")
	}
	if resolution != ResolutionAccept && resolution != ResolutionVoid && resolution != ResolutionRework {
		return NewError(CodeValidation, "无效的裁定类型")
	}
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.ID != findingID {
			continue
		}
		if f.Status == FindingResolved {
			return NewError(CodeConflict, "该发现已裁定")
		}
		if f.RuleCode == "missing" && resolution != ResolutionRework {
			return NewError(CodeValidation, "缺失评分只能退回重评")
		}
		s.applyResolution(f, resolution, qualityUser, now)
		return nil
	}
	return NewError(CodeNotFound, "核验发现不存在")
}

func (s *TastingSession) AllFindingsResolved() bool {
	for _, f := range s.Findings {
		if f.Status != FindingResolved {
			return false
		}
	}
	return true
}
