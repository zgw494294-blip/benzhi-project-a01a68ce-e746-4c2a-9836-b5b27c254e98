package eventqueryrace_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

func TestEventQueriesSynchronizeWithCommits(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	session, err := domain.NewSession("session-race", domain.SessionConfig{
		Name:            "并发查询复现",
		ProductCategory: "饮品",
		HostUserID:      "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"},
		ScheduledAt:     now,
		Scales:          []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10, Order: 1}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	firstCommit := make(chan struct{})
	errorsSeen := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		for i := 0; i < 24; i++ {
			if err := store.SaveSession(session, session.Version, "session_configured", "host"); err != nil {
				select {
				case errorsSeen <- err:
				default:
				}
				return
			}
			if i == 0 {
				close(firstCommit)
			}
		}
	}()

	queries := []func(){
		func() { _ = store.EventChainHash() },
		func() { _ = store.Health() },
		func() { _ = store.EventsForSession(session.ID) },
	}
	for _, query := range queries {
		query := query
		go func() {
			defer wg.Done()
			<-firstCommit
			for i := 0; i < 128; i++ {
				query()
			}
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("提交失败: %v", err)
	}
}
