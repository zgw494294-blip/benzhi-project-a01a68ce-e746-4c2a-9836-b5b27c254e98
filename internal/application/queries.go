package application

import (
	"fmt"
	"sort"

	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

type AuditView struct {
	SessionID string              `json:"sessionID"`
	Summary   domain.AuditSummary `json:"summary"`
	Entries   []domain.AuditEntry `json:"entries"`
	Events    []repository.Event  `json:"events,omitempty"`
}

func (a *Service) Progress(id, actor string) (domain.SessionProgress, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return domain.SessionProgress{}, err
	}
	if actor == "" || (actor != session.HostUserID && !session.IsReviewer(actor) && session.Status == domain.StatusDraft) {
		return domain.SessionProgress{}, domain.NewError(domain.CodeForbidden, "无权查看会话进度")
	}
	progress := session.Progress()
	if session.IsReviewer(actor) {
		for _, item := range progress.Reviewers {
			if item.ReviewerUserID == actor {
				progress.Reviewers = []domain.ReviewerProgress{item}
				break
			}
		}
	}
	return progress, nil
}

func (a *Service) ReviewerTasks(id, actor string) ([]domain.ReviewerTask, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	return session.ReviewerTaskView(actor)
}

func (a *Service) Audit(id, actor, actorFilter, actionFilter string, includeEvents bool) (*AuditView, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if actor == "" || session.IsReviewer(actor) {
		return nil, domain.NewError(domain.CodeForbidden, "评审员无权查看完整审计轨迹")
	}
	view := &AuditView{SessionID: id, Summary: session.AuditSummary(), Entries: session.AuditEntries(actorFilter, actionFilter)}
	if includeEvents {
		view.Events = a.Store.EventsForSession(id)
	}
	return view, nil
}

func (a *Service) Reverify(id, actor string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		return decodeView(result.Body)
	}
	view, err := a.mutate(id, actor, version, "verification_rebuilt", func(session *domain.TastingSession) error { return session.Reverify(actor, a.timestamp()) })
	if err == nil {
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	return view, err
}

func (a *Service) UpdateSample(id, sampleID string, in domain.SampleInput, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "sample_updated", func(session *domain.TastingSession) error {
		return session.ReplaceSample(sampleID, in, actor, a.timestamp())
	})
}

func (a *Service) Integrity(id, actor string) ([]domain.IntegrityIssue, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if actor == "" || session.IsReviewer(actor) {
		return nil, domain.NewError(domain.CodeForbidden, "无权执行完整性检查")
	}
	return session.ValidateIntegrity(), nil
}

func (a *Service) StoreHealth() repository.StoreHealth { return a.Store.Health() }

func (a *Service) ListReceipts() []*archive.ArchiveReceipt {
	receipts := a.Receipts.List()
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].SealedAt.Before(receipts[j].SealedAt) })
	return receipts
}

func (a *Service) ExportPackage(receiptID string) ([]byte, error) {
	if _, err := a.ValidateArchive(receiptID); err != nil {
		return nil, fmt.Errorf("验证归档凭据 %s: %w", receiptID, err)
	}
	receipt, err := a.Receipt(receiptID)
	if err != nil {
		return nil, fmt.Errorf("读取归档凭据 %s: %w", receiptID, err)
	}
	session, err := a.Store.GetSession(receipt.SessionID)
	if err != nil {
		return nil, fmt.Errorf("读取归档会话 %s: %w", receipt.SessionID, err)
	}
	pkg, err := a.Archive.BuildPackage(receipt, session)
	if err != nil {
		return nil, fmt.Errorf("构建归档封装 %s: %w", receiptID, err)
	}
	body, err := a.Archive.ExportPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("导出归档封装 %s: %w", receiptID, err)
	}
	return body, nil
}

func (a *Service) ValidateArchive(receiptID string) (archive.ValidationReport, error) {
	receipt, err := a.Receipt(receiptID)
	if err != nil {
		return archive.ValidationReport{}, fmt.Errorf("读取归档凭据 %s: %w", receiptID, err)
	}
	session, err := a.Store.GetSession(receipt.SessionID)
	if err != nil {
		return archive.ValidationReport{}, fmt.Errorf("读取归档会话 %s: %w", receipt.SessionID, err)
	}
	report := a.Archive.ValidateReport(receipt, session, a.Store)
	return report, nil
}
