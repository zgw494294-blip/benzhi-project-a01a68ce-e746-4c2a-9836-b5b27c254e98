package domain

import (
	"testing"
	"time"
)

func configured(t *testing.T) *TastingSession {
	t.Helper()
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	s, err := NewSession("test", SessionConfig{Name: "评审", ProductCategory: "饮料", HostUserID: "host", ReviewerUserIDs: []string{"r1", "r2"}, ScheduledAt: now, Seed: "fixed", Scales: []ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []SampleInput{{ID: "s1", InternalCode: "A", DisplayName: "甲"}, {ID: "s2", InternalCode: "B", DisplayName: "乙"}} {
		if err := s.AddSample(in, "host", now); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestPlanIsDeterministicAndFrozen(t *testing.T) {
	a, b := configured(t), configured(t)
	now := time.Now()
	if err := a.FreezePlan("host", now); err != nil {
		t.Fatal(err)
	}
	if err := b.FreezePlan("host", now); err != nil {
		t.Fatal(err)
	}
	if a.PlanHash != b.PlanHash {
		t.Fatalf("计划哈希不稳定: %s != %s", a.PlanHash, b.PlanHash)
	}
	if err := a.AddSample(SampleInput{ID: "s3", InternalCode: "C", DisplayName: "丙"}, "host", now); err == nil {
		t.Fatal("冻结后仍能添加样品")
	}
}

func TestEvaluationOrderAndRevealRoles(t *testing.T) {
	s := configured(t)
	now := time.Now()
	_ = s.FreezePlan("host", now)
	_ = s.StartCollection("host", now)
	tasks, _ := s.ReviewerAssignments("r1")
	if _, err := s.SubmitEvaluation(EvaluationInput{AssignmentID: tasks[1].ID, Scores: map[string]float64{"taste": 5}}, "r1", now); err == nil {
		t.Fatal("允许跳过呈样顺序")
	}
	for _, reviewer := range []string{"r1", "r2"} {
		list, _ := s.ReviewerAssignments(reviewer)
		for _, task := range list {
			if _, err := s.SubmitEvaluation(EvaluationInput{AssignmentID: task.ID, Scores: map[string]float64{"taste": 7}, StartedAt: now.Add(-20 * time.Second)}, reviewer, now); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.CloseCollection("host", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveReveal("host", "quality", now); err == nil {
		t.Fatal("主持人可以批准解盲")
	}
	if _, err := s.ApproveReveal("quality", "quality", now); err != nil {
		t.Fatal(err)
	}
	revealed, err := s.ApproveReveal("auditor", "independent", now)
	if err != nil || !revealed {
		t.Fatalf("双角色解盲失败: %v", err)
	}
}

func TestBatchSamplesIsAtomicAndIdempotentAtDomainBoundary(t *testing.T) {
	s := configured(t)
	now := time.Now().UTC()
	before := len(s.Samples)
	auditBefore := len(s.Audit)
	err := s.AddSamplesBatch([]SampleInput{{ID: "s3", InternalCode: "B", DisplayName: "重复"}, {ID: "s4", InternalCode: "C", DisplayName: "新"}}, "host", now)
	if err == nil {
		t.Fatal("预期会话内编码冲突")
	}
	if len(s.Samples) != before || len(s.Audit) != auditBefore {
		t.Fatalf("失败批量产生了部分写入: samples=%d audit=%d", len(s.Samples), len(s.Audit))
	}
	if err := s.AddSamplesBatch([]SampleInput{{ID: "s3", InternalCode: "C", DisplayName: "丙"}, {ID: "s4", InternalCode: "D", DisplayName: "丁"}}, "host", now); err != nil {
		t.Fatal(err)
	}
	if len(s.Samples) != before+2 || s.Audit[len(s.Audit)-1].Action != "samples_batch_added" {
		t.Fatalf("批量登记结果错误: %#v", s.Audit[len(s.Audit)-1])
	}
}

func TestScaleOrderAndPrecisionValidation(t *testing.T) {
	s := configured(t)
	err := s.Configure(SessionConfig{Name: s.Name, ProductCategory: s.ProductCategory, HostUserID: s.HostUserID, ReviewerUserIDs: s.ReviewerUserIDs, ScheduledAt: s.ScheduledAt, Scales: []ScaleDimension{{Key: "a", Name: "A", Min: 0, Max: 10, Order: 2}, {Key: "b", Name: "B", Min: 0, Max: 10.001, Order: 1}}}, "host", time.Now())
	if err == nil {
		t.Fatal("超出量表精度仍被接受")
	}
	if err := s.Configure(SessionConfig{Name: s.Name, ProductCategory: s.ProductCategory, HostUserID: s.HostUserID, ReviewerUserIDs: s.ReviewerUserIDs, ScheduledAt: s.ScheduledAt, Scales: []ScaleDimension{{Key: "a", Name: "A", Min: 0, Max: 10, Order: 2}, {Key: "b", Name: "B", Min: 0, Max: 5, Order: 1}}}, "host", time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.Scales[0].Order != 1 || s.Scales[1].Order != 2 {
		t.Fatalf("量表未规范排序: %#v", s.Scales)
	}
}
