package audit_detail_alias_test

import (
	"testing"
	"time"

	"sensory-blind-review/internal/domain"
)

func TestAuditEntriesDoNotAliasAggregateDetails(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	session, err := domain.NewSession("audit-alias", domain.SessionConfig{
		Name:            "审计隔离评审",
		ProductCategory: "饮料",
		HostUserID:      "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"},
		ScheduledAt:     now,
		Scales: []domain.ScaleDimension{
			{Key: "aroma", Name: "香气", Min: 0, Max: 10},
		},
	}, now)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	entries := session.AuditEntries("", "session_configured")
	if len(entries) != 1 || entries[0].Detail["scaleCount"] != "1" {
		t.Fatalf("审计准备结果异常: %#v", entries)
	}
	entries[0].Detail["scaleCount"] = "tampered"

	again := session.AuditEntries("", "session_configured")
	if got := again[0].Detail["scaleCount"]; got != "1" {
		t.Fatalf("查询结果污染了聚合审计详情: got %q, want %q", got, "1")
	}
}
