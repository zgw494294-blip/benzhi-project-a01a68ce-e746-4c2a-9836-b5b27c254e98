package archive_registry_race_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"sensory-blind-review/internal/archive"
)

func TestArchiveRegistrySynchronizesConcurrentAccess(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	registry := archive.NewRegistry()
	base := &archive.ArchiveReceipt{ID: "arc-base", SessionID: "session-base", SealedAt: time.Unix(1, 0).UTC()}
	if err := registry.Put(base); err != nil {
		t.Fatalf("准备初始归档凭据失败: %v", err)
	}

	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(3)

	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 4000; i++ {
			receipt := &archive.ArchiveReceipt{
				ID:        fmt.Sprintf("arc-%05d", i),
				SessionID: fmt.Sprintf("session-%05d", i),
				SealedAt:  time.Unix(int64(i+2), 0).UTC(),
			}
			if err := registry.Put(receipt); err != nil {
				t.Errorf("写入归档凭据失败: %v", err)
				return
			}
		}
	}()

	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 12000; i++ {
			if receipt, ok := registry.Get("arc-base"); !ok || receipt.ID != "arc-base" {
				t.Errorf("并发查询丢失初始归档凭据")
				return
			}
		}
	}()

	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 800; i++ {
			if len(registry.List()) == 0 {
				t.Errorf("并发列表查询返回空结果")
				return
			}
		}
	}()

	close(start)
	workers.Wait()
}
