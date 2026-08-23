package application

import (
	"encoding/json"
	"sensory-blind-review/internal/repository"
)

func decodeView(body json.RawMessage) (*SessionView, error) {
	var view SessionView
	if err := json.Unmarshal(body, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

func (a *Service) IdempotencyResult(key string) (int, json.RawMessage, bool) {
	result, ok := a.Store.GetIdempotency(key)
	if !ok {
		return 0, nil, false
	}
	return result.Status, append(json.RawMessage(nil), result.Body...), true
}

func IsVersionConflict(err error) bool { return err == repository.ErrVersionConflict }
