package canceled_sample_commit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/httpui"
	"sensory-blind-review/internal/repository"
)

func TestCanceledSampleRequestDoesNotCommit(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store, archive.NewService(), archive.NewRegistry())
	created, err := app.CreateSession(domain.SessionConfig{
		Name:            "取消传播复现",
		ProductCategory: "饮品",
		HostUserID:      "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"},
		ScheduledAt:     time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC),
		Scales:          []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10, Order: 1}},
	}, "host", "create-canceled-sample-case")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/"+created.ID+"/samples", strings.NewReader(`{"id":"sample-canceled","internalCode":"I-C","displayName":"不应保存"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "host")
	req.Header.Set("X-Expected-Version", "1")
	response := httptest.NewRecorder()
	httpui.New(app).Handler().ServeHTTP(response, req)

	persisted, err := store.GetSession(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code < http.StatusBadRequest || persisted.Version != created.Version || len(persisted.Samples) != 0 {
		t.Fatalf("已取消请求仍被提交: status=%d version=%d samples=%d", response.Code, persisted.Version, len(persisted.Samples))
	}
}
