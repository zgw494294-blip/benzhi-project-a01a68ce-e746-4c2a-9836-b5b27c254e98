package commit_append_failure_state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

func TestCommitDoesNotPublishStateWhenEventAppendFails(t *testing.T) {
	dir := t.TempDir()
	store, err := repository.Open(dir)
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "events.jsonl"), 0o755); err != nil {
		t.Fatalf("制造事件日志资源失效: %v", err)
	}
	now := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	session, err := domain.NewSession("append-failure-session", domain.SessionConfig{
		Name:            "事件追加失败复现",
		ProductCategory: "饮料",
		HostUserID:      "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"},
		ScheduledAt:     now,
		Scales:          []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10}},
	}, now)
	if err != nil {
		t.Fatalf("创建领域会话: %v", err)
	}

	if err := store.CreateSession(session); err == nil {
		t.Fatal("事件日志不可写时提交意外成功")
	}
	if session.Version != 0 {
		t.Errorf("失败提交修改了调用方版本: version=%d", session.Version)
	}
	if _, err := store.GetSession(session.ID); domain.ErrorCodeOf(err) != domain.CodeNotFound {
		t.Errorf("失败提交向查询侧发布了未持久化会话: err=%v", err)
	}
	if health := store.Health(); health.EventCount != 0 || health.SessionCount != 0 {
		t.Errorf("失败提交污染仓储内存状态: events=%d sessions=%d", health.EventCount, health.SessionCount)
	}
}
