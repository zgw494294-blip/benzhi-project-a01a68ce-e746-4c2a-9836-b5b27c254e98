package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sensory-blind-review/internal/domain"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	receipts map[string]*ArchiveReceipt
	path     string
}

func NewRegistry() *Registry { return &Registry{receipts: map[string]*ArchiveReceipt{}} }
func NewRegistryAt(dir string) (*Registry, error) {
	r := &Registry{receipts: map[string]*ArchiveReceipt{}, path: filepath.Join(dir, "archive-receipts.json")}
	b, err := os.ReadFile(r.path)
	if err == nil {
		if err := json.Unmarshal(b, &r.receipts); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return r, nil
}
func (r *Registry) Put(receipt *ArchiveReceipt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.receipts[receipt.ID]; exists {
		return domain.NewError(domain.CodeConflict, "归档凭据已存在")
	}
	clone := *receipt
	r.receipts[receipt.ID] = &clone
	if r.path != "" {
		if err := r.persist(); err != nil {
			delete(r.receipts, receipt.ID)
			return err
		}
	}
	return nil
}
func (r *Registry) Get(id string) (*ArchiveReceipt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.receipts[id]
	if !ok {
		return nil, false
	}
	clone := *value
	return &clone, true
}

func (r *Registry) List() []*ArchiveReceipt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ArchiveReceipt, 0, len(r.receipts))
	for _, value := range r.receipts {
		clone := *value
		out = append(out, &clone)
	}
	return out
}

func (r *Registry) persist() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.path), "receipts-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := json.NewEncoder(tmp).Encode(r.receipts); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, r.path)
}
