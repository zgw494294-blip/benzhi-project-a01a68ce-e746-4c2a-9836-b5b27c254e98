package session_view_cache_scope_test

import (
	"testing"
	"time"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

func TestSessionViewCacheSeparatesOwnershipVersionAndActor(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store, archive.NewService(), archive.NewRegistry())
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	created, err := app.CreateSession(domain.SessionConfig{
		Name: "缓存隔离评审", ProductCategory: "饮料", HostUserID: "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"}, ScheduledAt: now,
		Seed: "cache-scope", Scales: []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10}},
	}, "host", "create-cache-scope")
	if err != nil {
		t.Fatal(err)
	}

	withFirstSample, err := app.AddSample(created.ID, domain.SampleInput{
		ID: "sample-a", InternalCode: "INTERNAL-A", DisplayName: "样品甲",
	}, "host", created.Version)
	if err != nil {
		t.Fatal(err)
	}
	hostView, err := app.GetSession(created.ID, "host")
	if err != nil {
		t.Fatal(err)
	}
	hostView.Name = "调用方污染值"

	updated, err := app.AddSample(created.ID, domain.SampleInput{
		ID: "sample-b", InternalCode: "INTERNAL-B", DisplayName: "样品乙",
	}, "host", withFirstSample.Version)
	if err != nil {
		t.Fatal(err)
	}
	hostAgain, err := app.GetSession(created.ID, "host")
	if err != nil {
		t.Fatal(err)
	}
	reviewerView, err := app.GetSession(created.ID, "reviewer-a")
	if err != nil {
		t.Fatal(err)
	}

	if hostAgain.Name == "调用方污染值" {
		t.Error("调用方修改反向污染了后续查询")
	}
	if hostAgain.Version != updated.Version || len(hostAgain.Samples) != 2 {
		t.Errorf("状态更新后仍返回旧视图: version=%d samples=%d", hostAgain.Version, len(hostAgain.Samples))
	}
	if len(reviewerView.Samples) != 0 {
		t.Errorf("评审员复用了主持人视图并看到 %d 个内部样品", len(reviewerView.Samples))
	}
}
