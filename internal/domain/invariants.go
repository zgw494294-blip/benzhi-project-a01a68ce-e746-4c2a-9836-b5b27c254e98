package domain

import (
	"fmt"
	"sort"
)

type IntegrityIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *TastingSession) ValidateIntegrity() []IntegrityIssue {
	var issues []IntegrityIssue
	if s.ID == "" {
		issues = append(issues, IntegrityIssue{Code: "session_id", Message: "会话 ID 为空"})
	}
	if s.Name == "" || s.ProductCategory == "" {
		issues = append(issues, IntegrityIssue{Code: "session_config", Message: "会话基础配置不完整"})
	}
	if s.HostUserID == "" {
		issues = append(issues, IntegrityIssue{Code: "host", Message: "主持人为空"})
	}
	reviewers := map[string]bool{}
	for _, reviewer := range s.ReviewerUserIDs {
		if reviewer == "" || reviewers[reviewer] {
			issues = append(issues, IntegrityIssue{Code: "reviewer_duplicate", Message: "评审员为空或重复"})
		}
		if reviewer == s.HostUserID {
			issues = append(issues, IntegrityIssue{Code: "reviewer_host_conflict", Message: "主持人与评审员冲突"})
		}
		reviewers[reviewer] = true
	}
	scaleKeys := map[string]bool{}
	for _, scale := range s.Scales {
		if scale.Key == "" || scaleKeys[scale.Key] || scale.Max <= scale.Min {
			issues = append(issues, IntegrityIssue{Code: "scale", Message: "量表维度配置无效"})
		}
		scaleKeys[scale.Key] = true
	}
	samples := map[string]bool{}
	internalCodes := map[string]bool{}
	for _, sample := range s.Samples {
		if sample.SessionID != s.ID {
			issues = append(issues, IntegrityIssue{Code: "sample_session", Message: fmt.Sprintf("样品 %s 所属会话错误", sample.ID)})
		}
		if samples[sample.ID] || internalCodes[sample.InternalCode] {
			issues = append(issues, IntegrityIssue{Code: "sample_duplicate", Message: "样品 ID 或内部编码重复"})
		}
		samples[sample.ID], internalCodes[sample.InternalCode] = true, true
	}
	assignmentIDs := map[string]bool{}
	sequenceByReviewer := map[string][]int{}
	for _, assignment := range s.Assignments {
		if assignment.SessionID != s.ID || !reviewers[assignment.ReviewerUserID] || !samples[assignment.SampleID] {
			issues = append(issues, IntegrityIssue{Code: "assignment_reference", Message: fmt.Sprintf("盲码任务 %s 引用无效", assignment.ID)})
		}
		if assignmentIDs[assignment.ID] || assignment.PlanHash != s.PlanHash {
			issues = append(issues, IntegrityIssue{Code: "assignment_proof", Message: "盲码任务 ID 重复或计划哈希不一致"})
		}
		assignmentIDs[assignment.ID] = true
		sequenceByReviewer[assignment.ReviewerUserID] = append(sequenceByReviewer[assignment.ReviewerUserID], assignment.Sequence)
	}
	for reviewer, sequences := range sequenceByReviewer {
		sort.Ints(sequences)
		if len(sequences) != len(s.Samples) {
			issues = append(issues, IntegrityIssue{Code: "assignment_count", Message: fmt.Sprintf("评审员 %s 任务数量错误", reviewer)})
		}
		for i, sequence := range sequences {
			if sequence != i+1 {
				issues = append(issues, IntegrityIssue{Code: "assignment_sequence", Message: fmt.Sprintf("评审员 %s 呈样顺序不连续", reviewer)})
				break
			}
		}
	}
	evaluationIDs := map[string]bool{}
	for _, evaluation := range s.Evaluations {
		if evaluation.SessionID != s.ID || !assignmentIDs[evaluation.AssignmentID] || !reviewers[evaluation.ReviewerUserID] {
			issues = append(issues, IntegrityIssue{Code: "evaluation_reference", Message: fmt.Sprintf("评分 %s 引用无效", evaluation.ID)})
		}
		if evaluationIDs[evaluation.ID] {
			issues = append(issues, IntegrityIssue{Code: "evaluation_duplicate", Message: "评分 ID 重复"})
		}
		evaluationIDs[evaluation.ID] = true
		if err := s.validateScores(evaluation.Scores); err != nil {
			issues = append(issues, IntegrityIssue{Code: "evaluation_score", Message: err.Error()})
		}
	}
	if s.Status == StatusDraft && len(s.Assignments) > 0 {
		issues = append(issues, IntegrityIssue{Code: "draft_assignments", Message: "草稿会话不应包含冻结任务"})
	}
	if (s.Status == StatusFrozen || s.Status == StatusCollecting || s.Status == StatusVerifying || s.Status == StatusRevealed || s.Status == StatusSealed) && (s.PlanHash == "" || len(s.Assignments) == 0) {
		issues = append(issues, IntegrityIssue{Code: "missing_plan", Message: "非草稿会话缺少盲码计划"})
	}
	if (s.Status == StatusRevealed || s.Status == StatusSealed) && len(s.RevealApprovals) != 2 {
		issues = append(issues, IntegrityIssue{Code: "reveal_approval", Message: "已解盲会话缺少双角色批准"})
	}
	if s.Status == StatusSealed && s.ArchiveReceiptID == "" {
		issues = append(issues, IntegrityIssue{Code: "archive_receipt", Message: "已封存会话缺少归档凭据"})
	}
	return issues
}

func (s *TastingSession) EnsureIntegrity() error {
	issues := s.ValidateIntegrity()
	if len(issues) == 0 {
		return nil
	}
	return NewError(CodeValidation, "会话完整性校验失败：%s", issues[0].Message)
}
