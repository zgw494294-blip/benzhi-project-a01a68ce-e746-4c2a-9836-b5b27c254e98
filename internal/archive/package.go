package archive

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"sensory-blind-review/internal/domain"
)

type ArchivedSample struct {
	ID           string `json:"id"`
	InternalCode string `json:"internalCode"`
	DisplayName  string `json:"displayName"`
	BatchRef     string `json:"batchRef,omitempty"`
}

type ArchivedAssignment struct {
	ReviewerUserID string `json:"reviewerUserID"`
	BlindCode      string `json:"blindCode"`
	SampleID       string `json:"sampleID"`
	Sequence       int    `json:"sequence"`
}

type ArchiveManifest struct {
	Format           string    `json:"format"`
	SchemaVersion    int       `json:"schemaVersion"`
	GeneratedAt      time.Time `json:"generatedAt"`
	SessionID        string    `json:"sessionID"`
	SessionVersion   int64     `json:"sessionVersion"`
	ReceiptID        string    `json:"receiptID"`
	EventChainHash   string    `json:"eventChainHash"`
	PlanHash         string    `json:"planHash"`
	ConclusionDigest string    `json:"conclusionDigest"`
}

type SealedPackage struct {
	Manifest    ArchiveManifest              `json:"manifest"`
	Receipt     ArchiveReceipt               `json:"receipt"`
	SessionName string                       `json:"sessionName"`
	Category    string                       `json:"productCategory"`
	ScheduledAt time.Time                    `json:"scheduledAt"`
	Samples     []ArchivedSample             `json:"samples"`
	Assignments []ArchivedAssignment         `json:"assignments"`
	Conclusions []domain.DimensionConclusion `json:"conclusions"`
	Audit       []domain.AuditEntry          `json:"audit"`
}

func (s *Service) BuildPackage(receipt *ArchiveReceipt, session *domain.TastingSession) (*SealedPackage, error) {
	if receipt == nil || session == nil {
		return nil, fmt.Errorf("归档凭据或会话为空")
	}
	if session.Status != domain.StatusSealed {
		return nil, domain.NewError(domain.CodeState, "仅已封存会话可以导出封存包")
	}
	if session.ArchiveReceiptID != receipt.ID || receipt.SessionID != session.ID {
		return nil, fmt.Errorf("归档凭据与会话不匹配")
	}
	if receipt.PlanHash != session.PlanHash || receipt.ConclusionDigest != digest(session.Conclusions) {
		return nil, fmt.Errorf("归档摘要校验失败")
	}
	pkg := &SealedPackage{Manifest: ArchiveManifest{Format: "sensory-blind-review/archive", SchemaVersion: 1, GeneratedAt: receipt.SealedAt, SessionID: session.ID, SessionVersion: session.Version, ReceiptID: receipt.ID, EventChainHash: receipt.EventChainHash, PlanHash: receipt.PlanHash, ConclusionDigest: receipt.ConclusionDigest}, Receipt: *receipt, SessionName: session.Name, Category: session.ProductCategory, ScheduledAt: session.ScheduledAt, Conclusions: cloneConclusions(session.Conclusions), Audit: cloneAudit(session.Audit)}
	for _, sample := range session.Samples {
		pkg.Samples = append(pkg.Samples, ArchivedSample{ID: sample.ID, InternalCode: sample.InternalCode, DisplayName: sample.DisplayName, BatchRef: sample.BatchRef})
	}
	for _, assignment := range session.Assignments {
		pkg.Assignments = append(pkg.Assignments, ArchivedAssignment{ReviewerUserID: assignment.ReviewerUserID, BlindCode: assignment.BlindCode, SampleID: assignment.SampleID, Sequence: assignment.Sequence})
	}
	sort.Slice(pkg.Samples, func(i, j int) bool { return pkg.Samples[i].ID < pkg.Samples[j].ID })
	sort.Slice(pkg.Assignments, func(i, j int) bool {
		if pkg.Assignments[i].ReviewerUserID == pkg.Assignments[j].ReviewerUserID {
			return pkg.Assignments[i].Sequence < pkg.Assignments[j].Sequence
		}
		return pkg.Assignments[i].ReviewerUserID < pkg.Assignments[j].ReviewerUserID
	})
	return pkg, nil
}

func (s *Service) ExportPackage(pkg *SealedPackage) ([]byte, error) {
	if pkg == nil {
		return nil, fmt.Errorf("封存包为空")
	}
	if err := s.ValidatePackage(pkg); err != nil {
		return nil, err
	}
	return json.MarshalIndent(pkg, "", "  ")
}

func (s *Service) ValidatePackage(pkg *SealedPackage) error {
	if pkg.Manifest.Format != "sensory-blind-review/archive" || pkg.Manifest.SchemaVersion != 1 {
		return fmt.Errorf("封存包格式不受支持")
	}
	if pkg.Manifest.SessionID == "" || pkg.Manifest.ReceiptID == "" || pkg.Manifest.PlanHash == "" || pkg.Manifest.EventChainHash == "" {
		return fmt.Errorf("封存包清单不完整")
	}
	if pkg.Receipt.ID != pkg.Manifest.ReceiptID || pkg.Receipt.SessionID != pkg.Manifest.SessionID {
		return fmt.Errorf("封存包凭据与清单不一致")
	}
	if pkg.Receipt.ConclusionDigest != digest(pkg.Conclusions) {
		return fmt.Errorf("封存包结论摘要不一致")
	}
	sampleIDs := map[string]bool{}
	for _, sample := range pkg.Samples {
		if sample.ID == "" || sampleIDs[sample.ID] {
			return fmt.Errorf("封存包样品无效")
		}
		sampleIDs[sample.ID] = true
	}
	for _, assignment := range pkg.Assignments {
		if !sampleIDs[assignment.SampleID] || assignment.BlindCode == "" || assignment.ReviewerUserID == "" {
			return fmt.Errorf("封存包盲码任务引用无效")
		}
	}
	return nil
}

func cloneConclusions(in []domain.DimensionConclusion) []domain.DimensionConclusion {
	b, _ := json.Marshal(in)
	var out []domain.DimensionConclusion
	_ = json.Unmarshal(b, &out)
	return out
}
func cloneAudit(in []domain.AuditEntry) []domain.AuditEntry {
	b, _ := json.Marshal(in)
	var out []domain.AuditEntry
	_ = json.Unmarshal(b, &out)
	return out
}
