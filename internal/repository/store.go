package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sensory-blind-review/internal/domain"
)

var ErrVersionConflict = errors.New("expectedVersion 冲突")

type IdempotentResult struct {
	Key       string          `json:"key"`
	Status    int             `json:"status"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Store struct {
	dir        string
	mu         sync.Mutex
	events     []Event
	sessions   map[string]*domain.TastingSession
	idempotent map[string]IdempotentResult
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, sessions: map[string]*domain.TastingSession{}, idempotent: map[string]IdempotentResult{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	eventPath := filepath.Join(s.dir, "events.jsonl")
	if f, err := os.Open(eventPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var e Event
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				return err
			}
			s.events = append(s.events, e)
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if err := verifyEventChain(s.events); err != nil {
			return err
		}
		for _, e := range s.events {
			if e.Action == "session_saved" {
				var session domain.TastingSession
				if err := json.Unmarshal(e.Payload, &session); err != nil {
					return err
				}
				s.sessions[session.ID] = &session
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(s.dir, "sessions.snapshot.json")); err == nil {
		var snapshot struct {
			SchemaVersion int                               `json:"schemaVersion"`
			Sessions      map[string]*domain.TastingSession `json:"sessions"`
		}
		if err := json.Unmarshal(b, &snapshot); err != nil {
			return err
		}
		if snapshot.SchemaVersion != schemaVersion {
			return fmt.Errorf("快照 schemaVersion 不一致")
		}
		// 事件日志是持久化的事实来源：每次提交都先同步事件再更新快照，
		// 因此当进程在写快照前退出时，快照可能落后于事件日志。这里仅用
		// 快照补充事件日志未能覆盖的会话，绝不用旧快照覆盖事件日志中较
		// 新的会话状态，以避免重启后回退版本号和会话名称。
		for id, snap := range snapshot.Sessions {
			if snap == nil {
				continue
			}
			if existing, ok := s.sessions[id]; ok && existing.Version >= snap.Version {
				continue
			}
			s.sessions[id] = cloneSession(snap)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(s.dir, "idempotency.json")); err == nil {
		_ = json.Unmarshal(b, &s.idempotent)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) ListSessions() []*domain.TastingSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*domain.TastingSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, cloneSession(session))
	}
	return out
}

func (s *Store) GetSession(id string) (*domain.TastingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id]; ok {
		return cloneSession(session), nil
	}
	return nil, domain.NewError(domain.CodeNotFound, "评审会话不存在")
}

func (s *Store) CreateSession(session *domain.TastingSession) error {
	return s.commit(session, 0, "session_created", "system")
}

func (s *Store) SaveSession(session *domain.TastingSession, expectedVersion int64, action, actor string) error {
	return s.commit(session, expectedVersion, action, actor)
}

func (s *Store) commit(session *domain.TastingSession, expectedVersion int64, action, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.sessions[session.ID]
	if exists && current.Version != expectedVersion {
		return ErrVersionConflict
	}
	if !exists && expectedVersion != 0 {
		return ErrVersionConflict
	}
	copy := cloneSession(session)
	copy.Version = expectedVersion + 1
	copy.UpdatedAt = time.Now().UTC()
	payload, _ := json.Marshal(copy)
	e := Event{SchemaVersion: schemaVersion, Sequence: int64(len(s.events) + 1), SessionID: session.ID, Action: "session_saved", Actor: actor, Version: copy.Version, Payload: payload, At: time.Now().UTC()}
	if len(s.events) > 0 {
		e.PreviousHash = s.events[len(s.events)-1].Hash
	}
	e.Hash = eventHash(e)
	if err := appendEvent(filepath.Join(s.dir, "events.jsonl"), e); err != nil {
		return err
	}
	s.events = append(s.events, e)
	s.sessions[session.ID] = copy
	session.Version = copy.Version
	session.UpdatedAt = copy.UpdatedAt
	if err := writeSnapshot(filepath.Join(s.dir, "sessions.snapshot.json"), s.sessions); err != nil {
		return err
	}
	return nil
}

func appendEvent(path string, e Event) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func writeSnapshot(path string, sessions map[string]*domain.TastingSession) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	payload := map[string]any{"schemaVersion": schemaVersion, "sessions": sessions}
	enc := json.NewEncoder(tmp)
	if err := enc.Encode(payload); err != nil {
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
	return os.Rename(name, path)
}

func (s *Store) EventChainHash() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return ""
	}
	return s.events[len(s.events)-1].Hash
}

func (s *Store) PutIdempotency(key string, status int, body any) error {
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	s.idempotent[key] = IdempotentResult{Key: key, Status: status, Body: b, CreatedAt: time.Now().UTC()}
	return writeJSON(filepath.Join(s.dir, "idempotency.json"), s.idempotent)
}

func (s *Store) GetIdempotency(key string) (IdempotentResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.idempotent[key]
	return result, ok
}

func writeJSON(path string, value any) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "data-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := json.NewEncoder(tmp).Encode(value); err != nil {
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
	return os.Rename(name, path)
}

func cloneSession(in *domain.TastingSession) *domain.TastingSession {
	b, _ := json.Marshal(in)
	var out domain.TastingSession
	_ = json.Unmarshal(b, &out)
	return &out
}

func (s *Store) Validate() error { s.mu.Lock(); defer s.mu.Unlock(); return verifyEventChain(s.events) }

func (s *Store) String() string { return fmt.Sprintf("Store(%s)", s.dir) }
