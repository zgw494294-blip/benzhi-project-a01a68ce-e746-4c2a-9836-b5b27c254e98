package archive_receipt_restart_test

import (
	"testing"
	"time"

	"sensory-blind-review/internal/archive"
)

func TestArchiveReceiptSurvivesRegistryRestart(t *testing.T) {
	dir := t.TempDir()
	registry, err := archive.NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	receipt := &archive.ArchiveReceipt{
		ID:               "arc-restart-proof",
		SessionID:        "session-restart-proof",
		SessionVersion:   7,
		EventChainHash:   "event-chain-proof",
		PlanHash:         "plan-proof",
		ConclusionDigest: "conclusion-proof",
		SealedBy:         "quality-user",
		SealedAt:         time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		SchemaVersion:    1,
	}

	if err := registry.Put(receipt); err != nil {
		t.Fatalf("put receipt: %v", err)
	}
	if _, ok := registry.Get(receipt.ID); !ok {
		t.Fatal("receipt was not published to the current process")
	}

	reopened, err := archive.NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("reopen registry: %v", err)
	}
	got, ok := reopened.Get(receipt.ID)
	if !ok {
		t.Fatalf("TestArchiveReceiptSurvivesRegistryRestart: Put returned success but receipt %q disappeared after restart", receipt.ID)
	}
	if got.SessionID != receipt.SessionID || got.EventChainHash != receipt.EventChainHash {
		t.Fatalf("TestArchiveReceiptSurvivesRegistryRestart: restored receipt changed: %#v", got)
	}
}
