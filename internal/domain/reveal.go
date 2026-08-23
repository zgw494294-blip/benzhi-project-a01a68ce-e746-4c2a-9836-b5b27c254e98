package domain

import (
	"sort"
	"time"
)

func (s *TastingSession) ApproveReveal(userID, role string, now time.Time) (bool, error) {
	if s.Status != StatusVerifying {
		return false, NewError(CodeState, "仅核验中会话可以批准解盲")
	}
	if !s.AllFindingsResolved() {
		return false, NewError(CodeConflict, "仍有未解决核验发现")
	}
	if userID == "" || userID == s.HostUserID || s.IsReviewer(userID) {
		return false, NewError(CodeForbidden, "批准人必须独立于主持人和评审员")
	}
	if role != "quality" && role != "independent" {
		return false, NewError(CodeValidation, "批准角色必须为 quality 或 independent")
	}
	for _, approval := range s.RevealApprovals {
		if approval.UserID == userID {
			return false, NewError(CodeConflict, "同一用户不能重复批准")
		}
		if approval.Role == role {
			return false, NewError(CodeConflict, "该批准角色已经完成")
		}
	}
	s.RevealApprovals = append(s.RevealApprovals, RevealApproval{UserID: userID, Role: role, ApprovedAt: now.UTC()})
	s.touch(userID, "reveal_approved", map[string]string{"role": role}, now)
	if len(s.RevealApprovals) < 2 {
		return false, nil
	}
	s.Conclusions = s.BuildConclusions()
	s.Status = StatusRevealed
	s.touch(userID, "session_revealed", nil, now)
	return true, nil
}

func (s *TastingSession) BuildConclusions() []DimensionConclusion {
	active := activeEvaluations(s.Evaluations)
	result := make([]DimensionConclusion, 0, len(s.Scales))
	for _, scale := range s.Scales {
		totals := map[string]float64{}
		counts := map[string]int{}
		for _, e := range active {
			a, err := s.assignmentFor(e.AssignmentID, e.ReviewerUserID)
			if err != nil {
				continue
			}
			totals[a.SampleID] += e.Scores[scale.Key]
			counts[a.SampleID]++
		}
		means := map[string]float64{}
		stats := map[string]SampleStatistic{}
		best := -1e308
		ids := make([]string, 0, len(totals))
		for id := range totals {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			values := make([]float64, 0, counts[id])
			for _, e := range active {
				a, err := s.assignmentFor(e.AssignmentID, e.ReviewerUserID)
				if err == nil && a.SampleID == id {
					values = append(values, e.Scores[scale.Key])
				}
			}
			min, max := values[0], values[0]
			for _, value := range values[1:] {
				if value < min {
					min = value
				}
				if value > max {
					max = value
				}
			}
			means[id] = rounded(totals[id] / float64(counts[id]))
			stats[id] = SampleStatistic{Mean: means[id], Count: counts[id], Min: rounded(min), Max: rounded(max)}
			if means[id] > best {
				best = means[id]
			}
		}
		winners := make([]string, 0)
		for _, id := range ids {
			if means[id] == best {
				winners = append(winners, id)
			}
		}
		winner := ""
		if len(winners) > 0 {
			winner = winners[0]
		}
		result = append(result, DimensionConclusion{DimensionKey: scale.Key, Dimension: scale.Name, SampleMeans: means, SampleStats: stats, WinnerID: winner, WinnerIDs: winners, Count: len(active)})
	}
	return result
}

func (s *TastingSession) RevealMapping() (map[string]map[string]string, error) {
	if s.Status != StatusRevealed && s.Status != StatusSealed {
		return nil, NewError(CodeForbidden, "会话尚未解盲")
	}
	sampleNames := map[string]string{}
	for _, sample := range s.Samples {
		sampleNames[sample.ID] = sample.DisplayName
	}
	out := map[string]map[string]string{}
	for _, a := range s.Assignments {
		if out[a.ReviewerUserID] == nil {
			out[a.ReviewerUserID] = map[string]string{}
		}
		out[a.ReviewerUserID][a.BlindCode] = sampleNames[a.SampleID]
	}
	return out, nil
}

func (s *TastingSession) MarkSealed(receiptID, actor string, now time.Time) error {
	if s.Status != StatusRevealed {
		return NewError(CodeState, "仅已解盲会话可以封存")
	}
	if actor == "" || actor == s.HostUserID || s.IsReviewer(actor) {
		return NewError(CodeForbidden, "封存必须由独立质量复核员执行")
	}
	if receiptID == "" {
		return NewError(CodeValidation, "归档凭据 ID 不能为空")
	}
	s.ArchiveReceiptID = receiptID
	s.Status = StatusSealed
	s.touch(actor, "session_sealed", map[string]string{"receiptID": receiptID}, now)
	return nil
}
