package repository

import (
	"sensory-blind-review/internal/domain"
	"testing"
	"time"
)

func TestStorePersistsAndRestoresHashChain(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s, err := domain.NewSession("s", domain.SessionConfig{Name: "评审", ProductCategory: "食品", HostUserID: "h", ReviewerUserIDs: []string{"a", "b"}, ScheduledAt: now, Scales: []domain.ScaleDimension{{Key: "x", Name: "香气", Min: 0, Max: 5}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(s); err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 {
		t.Fatalf("版本=%d", s.Version)
	}
	if err := store.SaveSession(s, 0, "bad", "h"); err != ErrVersionConflict {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Validate(); err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.GetSession("s")
	if err != nil || loaded.Version != 1 {
		t.Fatalf("恢复失败: %#v %v", loaded, err)
	}
}
