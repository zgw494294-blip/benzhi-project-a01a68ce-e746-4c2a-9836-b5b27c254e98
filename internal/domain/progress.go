package domain

import "sort"

type ReviewerProgress struct {
	ReviewerUserID string `json:"reviewerUserID"`
	Assigned       int    `json:"assigned"`
	Submitted      int    `json:"submitted"`
	ReworkPending  int    `json:"reworkPending"`
	Completed      bool   `json:"completed"`
}

type SessionProgress struct {
	SessionID        string             `json:"sessionID"`
	Status           SessionStatus      `json:"status"`
	TotalTasks       int                `json:"totalTasks"`
	ValidScores      int                `json:"validScores"`
	OpenFindings     int                `json:"openFindings"`
	ResolvedFindings int                `json:"resolvedFindings"`
	Reviewers        []ReviewerProgress `json:"reviewers"`
}

func (s *TastingSession) Progress() SessionProgress {
	result := SessionProgress{SessionID: s.ID, Status: s.Status, TotalTasks: len(s.Assignments)}
	byReviewer := map[string]*ReviewerProgress{}
	for _, reviewer := range s.ReviewerUserIDs {
		byReviewer[reviewer] = &ReviewerProgress{ReviewerUserID: reviewer}
	}
	for _, assignment := range s.Assignments {
		if p := byReviewer[assignment.ReviewerUserID]; p != nil {
			p.Assigned++
		}
	}
	for _, evaluation := range s.Evaluations {
		p := byReviewer[evaluation.ReviewerUserID]
		if p == nil {
			continue
		}
		if evaluation.ValidityStatus == ValidityValid {
			p.Submitted++
			result.ValidScores++
		}
		if evaluation.ValidityStatus == ValidityRework {
			p.ReworkPending++
		}
	}
	for _, finding := range s.Findings {
		if finding.Status == FindingOpen {
			result.OpenFindings++
		} else {
			result.ResolvedFindings++
		}
	}
	for _, p := range byReviewer {
		p.Completed = p.Assigned > 0 && p.Submitted == p.Assigned && p.ReworkPending == 0
		result.Reviewers = append(result.Reviewers, *p)
	}
	sort.Slice(result.Reviewers, func(i, j int) bool { return result.Reviewers[i].ReviewerUserID < result.Reviewers[j].ReviewerUserID })
	return result
}

type ReviewerTask struct {
	AssignmentID  string             `json:"assignmentID"`
	BlindCode     string             `json:"blindCode"`
	Sequence      int                `json:"sequence"`
	Scales        []ScaleDimension   `json:"scales"`
	Submitted     bool               `json:"submitted"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	Comment       string             `json:"comment,omitempty"`
	Revision      int                `json:"revision,omitempty"`
	ReworkPending bool               `json:"reworkPending,omitempty"`
}

func (s *TastingSession) ReviewerTaskView(reviewer string) ([]ReviewerTask, error) {
	assignments, err := s.ReviewerAssignments(reviewer)
	if err != nil {
		return nil, err
	}
	tasks := make([]ReviewerTask, 0, len(assignments))
	for _, assignment := range assignments {
		task := ReviewerTask{AssignmentID: assignment.ID, BlindCode: assignment.BlindCode, Sequence: assignment.Sequence, Scales: append([]ScaleDimension(nil), s.Scales...)}
		var latest *Evaluation
		for _, evaluation := range s.Evaluations {
			if evaluation.AssignmentID != assignment.ID || evaluation.ReviewerUserID != reviewer {
				continue
			}
			copy := evaluation
			if latest == nil || copy.Revision > latest.Revision {
				latest = &copy
			}
		}
		if latest != nil {
			task.Revision = latest.Revision
			task.ReworkPending = latest.ValidityStatus == ValidityRework
			task.Submitted = latest.ValidityStatus == ValidityValid
			if task.Submitted {
				task.Scores, task.Comment = cloneScores(latest.Scores), latest.Comment
			}
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}
