package domain

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

const fastThreshold = 8 * time.Second

func (s *TastingSession) CloseCollection(actor string, now time.Time) error {
	if err := s.RequireHost(actor); err != nil {
		return err
	}
	if s.Status != StatusCollecting {
		return NewError(CodeState, "仅采集中会话可以关闭采集")
	}
	s.Status = StatusVerifying
	s.Findings = s.buildFindings()
	s.touch(actor, "collection_closed", map[string]string{"findingCount": strconv.Itoa(len(s.Findings))}, now)
	return nil
}

func (s *TastingSession) Reverify(actor string, now time.Time) error {
	if s.Status != StatusVerifying {
		return NewError(CodeState, "仅核验中会话可以重新核验")
	}
	if actor == "" || s.IsReviewer(actor) || actor == s.HostUserID {
		return NewError(CodeForbidden, "重新核验必须由独立质量复核员执行")
	}
	for _, f := range s.Findings {
		if f.Status == FindingOpen {
			return NewError(CodeConflict, "仍有未裁定发现，不能重新核验")
		}
	}
	s.Findings = s.buildFindings()
	s.touch(actor, "verification_rebuilt", map[string]string{"findingCount": strconv.Itoa(len(s.Findings))}, now)
	return nil
}

func (s *TastingSession) buildFindings() []VerificationFinding {
	var findings []VerificationFinding
	active := activeEvaluations(s.Evaluations)
	byAssignment := map[string][]Evaluation{}
	for _, e := range active {
		byAssignment[e.AssignmentID] = append(byAssignment[e.AssignmentID], e)
	}
	for _, a := range s.Assignments {
		list := byAssignment[a.ID]
		if len(list) == 0 {
			findings = append(findings, newFinding(s.ID, "missing", a.ID, "", "high", map[string]string{"reviewerUserID": a.ReviewerUserID, "blindCode": a.BlindCode}))
		}
		if len(list) > 1 {
			for _, e := range list[1:] {
				findings = append(findings, newFinding(s.ID, "duplicate", a.ID, e.ID, "high", map[string]string{"count": strconv.Itoa(len(list))}))
			}
		}
	}
	for _, e := range active {
		if e.SubmittedAt.Sub(e.StartedAt) < fastThreshold {
			findings = append(findings, newFinding(s.ID, "too_fast", e.AssignmentID, e.ID, "medium", map[string]string{"durationSeconds": fmt.Sprintf("%.2f", e.SubmittedAt.Sub(e.StartedAt).Seconds()), "thresholdSeconds": "8"}))
		}
	}
	for _, scale := range s.Scales {
		values := make([]float64, 0, len(active))
		for _, e := range active {
			values = append(values, e.Scores[scale.Key])
		}
		if len(values) < 4 {
			continue
		}
		center := median(values)
		spread := mad(values, center)
		if spread == 0 {
			continue
		}
		for _, e := range active {
			distance := abs(e.Scores[scale.Key] - center)
			if distance > 3*spread {
				findings = append(findings, newFinding(s.ID, "mad_outlier", e.AssignmentID, e.ID, "medium", map[string]string{"dimension": scale.Key, "value": fmt.Sprintf("%.2f", e.Scores[scale.Key]), "median": fmt.Sprintf("%.2f", center), "mad": fmt.Sprintf("%.2f", spread)}))
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i].RuleCode + findings[i].AssignmentID + findings[i].EvaluationID
		right := findings[j].RuleCode + findings[j].AssignmentID + findings[j].EvaluationID
		return left < right
	})
	for i := range findings {
		findings[i].ID = fmt.Sprintf("finding-%03d", i+1)
	}
	return findings
}

func newFinding(sessionID, rule, assignment, evaluation, severity string, evidence map[string]string) VerificationFinding {
	return VerificationFinding{SessionID: sessionID, AssignmentID: assignment, EvaluationID: evaluation, RuleCode: rule, Severity: severity, Evidence: evidence, Status: FindingOpen}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
