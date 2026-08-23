package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const schemaVersion = 1

type Event struct {
	SchemaVersion int             `json:"schemaVersion"`
	Sequence      int64           `json:"sequence"`
	SessionID     string          `json:"sessionID"`
	Action        string          `json:"action"`
	Actor         string          `json:"actor"`
	Version       int64           `json:"version"`
	Payload       json.RawMessage `json:"payload"`
	PreviousHash  string          `json:"previousHash"`
	Hash          string          `json:"hash"`
	At            time.Time       `json:"at"`
}

func eventHash(e Event) string {
	e.Hash = ""
	b, _ := json.Marshal(e)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func verifyEventChain(events []Event) error {
	previous := ""
	for i, e := range events {
		if e.SchemaVersion != schemaVersion || e.Sequence != int64(i+1) || e.PreviousHash != previous {
			return fmt.Errorf("事件链结构校验失败: sequence=%d", e.Sequence)
		}
		if eventHash(e) != e.Hash {
			return fmt.Errorf("事件链哈希校验失败: sequence=%d", e.Sequence)
		}
		previous = e.Hash
	}
	return nil
}
