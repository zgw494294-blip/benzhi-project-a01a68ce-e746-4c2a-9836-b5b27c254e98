package restart_snapshot_replay_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

func TestRestartReplaysEventsNewerThanSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := repository.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	initial := domain.SessionConfig{
		Name: "初始评审", ProductCategory: "饮料", HostUserID: "host",
		ReviewerUserIDs: []string{"reviewer-a", "reviewer-b"}, ScheduledAt: now,
		Scales: []domain.ScaleDimension{{Key: "taste", Name: "口感", Min: 0, Max: 10, Order: 1}},
	}
	session, err := domain.NewSession("session-recovery", initial, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(dir, "sessions.snapshot.json")
	staleSnapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}

	updated := initial
	updated.Name = "更新后的评审"
	if err := session.Configure(updated, "host", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(session, 1, "session_configured", "host"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, staleSnapshot, 0o644); err != nil {
		t.Fatal(err)
	}

	reopened, err := repository.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Version != 2 || recovered.Name != updated.Name {
		t.Fatalf("重启恢复丢失已持久化事件: version=%d name=%q", recovered.Version, recovered.Name)
	}
}
