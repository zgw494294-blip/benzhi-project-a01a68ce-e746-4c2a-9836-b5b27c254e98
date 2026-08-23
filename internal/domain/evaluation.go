package domain

import (
	"fmt"
	"strings"
	"time"
)

type EvaluationInput struct {
	AssignmentID string             `json:"assignmentID"`
	Scores       map[string]float64 `json:"scores"`
	Comment      string             `json:"comment"`
	StartedAt    time.Time          `json:"startedAt"`
}

func (s *TastingSession) SubmitEvaluation(in EvaluationInput, reviewer string, now time.Time) (*Evaluation, error) {
	if s.Status != StatusCollecting && s.Status != StatusVerifying {
		return nil, NewError(CodeState, "当前会话不接受评分")
	}
	if !s.IsReviewer(reviewer) {
		return nil, NewError(CodeForbidden, "用户不是本会话评审员")
	}
	assignment, err := s.assignmentFor(in.AssignmentID, reviewer)
	if err != nil {
		return nil, err
	}
	if err := s.validateScores(in.Scores); err != nil {
		return nil, err
	}
	if s.Status == StatusVerifying && !s.reworkAllowed(in.AssignmentID) {
		return nil, NewError(CodeConflict, "该评审任务未被退回重评")
	}
	latestRevision := 0
	for i := range s.Evaluations {
		e := &s.Evaluations[i]
		if e.AssignmentID != in.AssignmentID || e.ReviewerUserID != reviewer {
			continue
		}
		if e.Revision > latestRevision {
			latestRevision = e.Revision
		}
		if e.ValidityStatus == ValidityValid {
			return nil, NewError(CodeConflict, "该盲样已经提交评分")
		}
		if e.ValidityStatus == ValidityRework && s.Status == StatusVerifying {
			continue
		}
		if e.ValidityStatus != ValidityVoided {
			return nil, NewError(CodeConflict, "该评分尚未允许重评")
		}
	}
	if err := s.ensureSequence(reviewer, assignment.Sequence); err != nil {
		return nil, err
	}
	started := in.StartedAt.UTC()
	if started.IsZero() || started.After(now) {
		started = now.Add(-30 * time.Second).UTC()
	}
	e := Evaluation{
		ID: fmt.Sprintf("eval-%s-r%d", assignment.ID, latestRevision+1), SessionID: s.ID,
		AssignmentID: assignment.ID, ReviewerUserID: reviewer, Scores: cloneScores(in.Scores),
		Comment: strings.TrimSpace(in.Comment), StartedAt: started, SubmittedAt: now.UTC(),
		ValidityStatus: ValidityValid, Revision: latestRevision + 1,
	}
	s.Evaluations = append(s.Evaluations, e)
	s.touch(reviewer, "evaluation_submitted", map[string]string{"evaluationID": e.ID, "assignmentID": assignment.ID}, now)
	return &s.Evaluations[len(s.Evaluations)-1], nil
}

func (s *TastingSession) reworkAllowed(assignmentID string) bool {
	for _, finding := range s.Findings {
		if finding.AssignmentID == assignmentID && finding.Status == FindingResolved && finding.Resolution == ResolutionRework {
			return true
		}
	}
	return false
}

func (s *TastingSession) assignmentFor(id, reviewer string) (*BlindAssignment, error) {
	for i := range s.Assignments {
		a := &s.Assignments[i]
		if a.ID == id && a.ReviewerUserID == reviewer {
			return a, nil
		}
	}
	return nil, NewError(CodeNotFound, "评审任务不存在")
}

func (s *TastingSession) validateScores(scores map[string]float64) error {
	if len(scores) != len(s.Scales) {
		return NewError(CodeValidation, "必须提交全部量表维度")
	}
	for _, scale := range s.Scales {
		value, ok := scores[scale.Key]
		if !ok {
			return NewError(CodeValidation, "缺少量表维度 %s", scale.Name)
		}
		if value < scale.Min || value > scale.Max {
			return NewError(CodeValidation, "量表 %s 分值必须在 %.2f 到 %.2f 之间", scale.Name, scale.Min, scale.Max)
		}
	}
	return nil
}

func (s *TastingSession) ensureSequence(reviewer string, sequence int) error {
	expected := 1
	for _, a := range s.Assignments {
		if a.ReviewerUserID != reviewer || a.Sequence >= sequence {
			continue
		}
		found := false
		for _, e := range s.Evaluations {
			if e.AssignmentID == a.ID && e.ValidityStatus == ValidityValid {
				found = true
				break
			}
		}
		if found {
			expected++
		}
	}
	if sequence != expected {
		return NewError(CodeConflict, "必须按呈样顺序提交，当前应提交第 %d 个任务", expected)
	}
	return nil
}

func cloneScores(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
