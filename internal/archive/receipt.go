package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

type ArchiveReceipt struct {
	ID               string    `json:"id"`
	SessionID        string    `json:"sessionID"`
	SessionVersion   int64     `json:"sessionVersion"`
	EventChainHash   string    `json:"eventChainHash"`
	PlanHash         string    `json:"planHash"`
	ConclusionDigest string    `json:"conclusionDigest"`
	SealedBy         string    `json:"sealedBy"`
	SealedAt         time.Time `json:"sealedAt"`
	SchemaVersion    int       `json:"schemaVersion"`
}

type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationReport struct {
	Valid        bool              `json:"valid"`
	CheckedAt    time.Time         `json:"checkedAt"`
	CredentialID string            `json:"credentialID"`
	ReceiptID    string            `json:"receiptID"`
	Issues       []ValidationIssue `json:"issues"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Create(session *domain.TastingSession, events *repository.Store, actor string, now time.Time) (*ArchiveReceipt, error) {
	if session.Status != domain.StatusRevealed {
		return nil, domain.NewError(domain.CodeState, "仅已解盲会话可以生成归档凭据")
	}
	if actor == "" || actor == session.HostUserID || session.IsReviewer(actor) {
		return nil, domain.NewError(domain.CodeForbidden, "封存人必须是独立质量复核员")
	}
	conclusionDigest := digest(session.Conclusions)
	receiptID := "arc-" + shortDigest(session.ID+session.PlanHash+conclusionDigest+events.EventChainHash())
	return &ArchiveReceipt{ID: receiptID, SessionID: session.ID, SessionVersion: session.Version + 1, EventChainHash: events.EventChainHash(), PlanHash: session.PlanHash, ConclusionDigest: conclusionDigest, SealedBy: actor, SealedAt: now.UTC(), SchemaVersion: 1}, nil
}

func (s *Service) Validate(receipt *ArchiveReceipt, session *domain.TastingSession, events *repository.Store) error {
	if receipt == nil || session == nil {
		return fmt.Errorf("归档凭据或会话为空")
	}
	if session.Status != domain.StatusSealed || session.ArchiveReceiptID != receipt.ID {
		return fmt.Errorf("归档状态不一致")
	}
	if receipt.EventChainHash != events.EventChainHash() {
		return fmt.Errorf("事件链哈希不一致")
	}
	if receipt.PlanHash != session.PlanHash || receipt.ConclusionDigest != digest(session.Conclusions) {
		return fmt.Errorf("归档摘要不一致")
	}
	return nil
}

func (s *Service) ValidateReport(receipt *ArchiveReceipt, session *domain.TastingSession, events *repository.Store) ValidationReport {
	report := ValidationReport{CheckedAt: time.Now().UTC(), Issues: []ValidationIssue{}}
	if receipt != nil {
		report.CredentialID = receipt.ID
		report.ReceiptID = receipt.ID
	}
	add := func(code, message string) {
		report.Issues = append(report.Issues, ValidationIssue{Code: code, Message: message})
	}
	if receipt == nil || session == nil {
		add("credential", "归档凭据或会话为空")
		report.Valid = false
		return report
	}
	if persisted, err := events.PersistedSession(session.ID); err != nil {
		add("projection", "无法读取会话快照: "+err.Error())
	} else {
		session = persisted
	}
	if session.Status != domain.StatusSealed || session.ArchiveReceiptID != receipt.ID {
		add("projection", "会话封存状态或凭据引用不一致")
	}
	if receipt.SessionVersion != session.Version {
		add("session_version", "会话版本与归档凭据不一致")
	}
	if receipt.SchemaVersion != 1 {
		add("schema_version", "归档凭据 schemaVersion 不受支持")
	}
	if hash, err := events.PersistedEventChainHash(); err != nil {
		add("event_chain", "事件链校验失败: "+err.Error())
	} else if receipt.EventChainHash != hash {
		add("event_chain", "事件链哈希不一致")
	}
	if receipt.PlanHash != session.PlanHash {
		add("plan", "盲码计划哈希不一致")
	}
	if receipt.ConclusionDigest != digest(session.Conclusions) {
		add("conclusion", "结论摘要不一致")
	}
	if err := events.VerifyPersistedProjection(session.ID); err != nil {
		add("projection", "会话投影校验失败: "+err.Error())
	}
	report.Valid = len(report.Issues) == 0
	return report
}

func (s *Service) Export(receipt *ArchiveReceipt) ([]byte, error) {
	if receipt == nil {
		return nil, fmt.Errorf("归档凭据为空")
	}
	return json.MarshalIndent(receipt, "", "  ")
}

func digest(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func shortDigest(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])[:16]
}
