package repository

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sensory-blind-review/internal/domain"
)

func (s *Store) PersistedEventChainHash() (string, error) {
	s.mu.RLock()
	path := filepath.Join(s.dir, "events.jsonl")
	s.mu.RUnlock()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return "", err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if err := verifyEventChain(events); err != nil {
		return "", err
	}
	return chainHead(events), nil
}

func (s *Store) PersistedSession(sessionID string) (*domain.TastingSession, error) {
	s.mu.RLock()
	path := filepath.Join(s.dir, "sessions.snapshot.json")
	s.mu.RUnlock()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot struct {
		SchemaVersion int                               `json:"schemaVersion"`
		Sessions      map[string]*domain.TastingSession `json:"sessions"`
	}
	if err := json.Unmarshal(b, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("快照 schemaVersion 不一致")
	}
	session := snapshot.Sessions[sessionID]
	if session == nil {
		return nil, domain.NewError(domain.CodeNotFound, "评审会话不存在")
	}
	return cloneSession(session), nil
}

func (s *Store) VerifyPersistedProjection(sessionID string) error {
	persisted, err := s.PersistedSession(sessionID)
	if err != nil {
		return err
	}
	s.mu.RLock()
	path := filepath.Join(s.dir, "events.jsonl")
	s.mu.RUnlock()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var latest *domain.TastingSession
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		if event.SessionID == sessionID && event.Action == "session_saved" {
			var candidate domain.TastingSession
			if err := json.Unmarshal(event.Payload, &candidate); err != nil {
				return err
			}
			latest = &candidate
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if latest == nil {
		return fmt.Errorf("会话 %s 没有可重建事件", sessionID)
	}
	left, _ := json.Marshal(persisted)
	right, _ := json.Marshal(latest)
	if string(left) != string(right) {
		return fmt.Errorf("会话 %s 快照与事件不一致", sessionID)
	}
	return nil
}

type StoreHealth struct {
	SchemaVersion    int    `json:"schemaVersion"`
	EventCount       int    `json:"eventCount"`
	SessionCount     int    `json:"sessionCount"`
	IdempotencyCount int    `json:"idempotencyCount"`
	ChainHead        string `json:"chainHead"`
	Valid            bool   `json:"valid"`
}

func (s *Store) Health() StoreHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StoreHealth{SchemaVersion: schemaVersion, EventCount: len(s.events), SessionCount: len(s.sessions), IdempotencyCount: len(s.idempotent), ChainHead: chainHead(s.events), Valid: verifyEventChain(s.events) == nil}
}

func chainHead(events []Event) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].Hash
}

func (s *Store) EventsForSession(sessionID string) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, 0)
	for _, event := range s.events {
		if event.SessionID != sessionID {
			continue
		}
		clone := event
		clone.Payload = append(json.RawMessage(nil), event.Payload...)
		out = append(out, clone)
	}
	return out
}

func (s *Store) VerifySessionProjection(sessionID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, exists := s.sessions[sessionID]
	if !exists {
		return domain.NewError(domain.CodeNotFound, "评审会话不存在")
	}
	var rebuilt *domain.TastingSession
	for _, event := range s.events {
		if event.SessionID == sessionID && event.Action == "session_saved" {
			var candidate domain.TastingSession
			if err := json.Unmarshal(event.Payload, &candidate); err != nil {
				return err
			}
			rebuilt = &candidate
		}
	}
	if rebuilt == nil {
		return fmt.Errorf("会话 %s 没有可重建事件", sessionID)
	}
	left, _ := json.Marshal(current)
	right, _ := json.Marshal(rebuilt)
	if string(left) != string(right) {
		return fmt.Errorf("会话 %s 投影与事件不一致", sessionID)
	}
	return nil
}

func (s *Store) VerifyAllProjections() error {
	s.mu.RLock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	sort.Strings(ids)
	for _, id := range ids {
		if err := s.VerifySessionProjection(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DataFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			files = append(files, filepath.Join(s.dir, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}
