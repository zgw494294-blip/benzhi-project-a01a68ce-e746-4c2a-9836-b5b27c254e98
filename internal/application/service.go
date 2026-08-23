package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

type Service struct {
	Store    *repository.Store
	Archive  *archive.Service
	Receipts *archive.Registry
	now      func() time.Time
}

func New(store *repository.Store, arch *archive.Service, receipts *archive.Registry) *Service {
	return &Service{Store: store, Archive: arch, Receipts: receipts, now: func() time.Time { return time.Now().UTC() }}
}
func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
func (s *Service) timestamp() time.Time { return s.now().UTC() }

type SessionView struct {
	*domain.TastingSession
	Mapping map[string]map[string]string `json:"mapping,omitempty"`
	Summary *ResolutionSummary           `json:"summary,omitempty"`
}

type ResolutionSummary struct {
	Open          int  `json:"open"`
	Resolved      int  `json:"resolved"`
	Rework        int  `json:"rework"`
	AllowReverify bool `json:"allowReverify"`
}

func (a *Service) CreateSession(cfg domain.SessionConfig, actor, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		var view SessionView
		if err := json.Unmarshal(result.Body, &view); err == nil {
			return &view, nil
		}
	}
	session, err := domain.NewSession(newID("ses"), cfg, a.timestamp())
	if err != nil {
		return nil, err
	}
	if session.HostUserID != actor {
		return nil, domain.NewError(domain.CodeForbidden, "只能由配置中的主持人创建会话")
	}
	if err := a.Store.CreateSession(session); err != nil {
		return nil, err
	}
	view := a.view(session, actor)
	_ = a.Store.PutIdempotency(key, 201, view)
	return view, nil
}

func (a *Service) Configure(id string, cfg domain.SessionConfig, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "session_configured", func(s *domain.TastingSession) error { return s.Configure(cfg, actor, a.timestamp()) })
}
func (a *Service) AddSample(id string, in domain.SampleInput, actor string, version int64) (*SessionView, error) {
	return a.AddSampleContext(context.Background(), id, in, actor, version)
}
func (a *Service) AddSampleContext(ctx context.Context, id string, in domain.SampleInput, actor string, version int64) (*SessionView, error) {
	return a.mutateWithContext(ctx, id, actor, version, "sample_added", func(s *domain.TastingSession) error { return s.AddSample(in, actor, a.timestamp()) })
}
func (a *Service) AddSamplesBatch(id string, inputs []domain.SampleInput, actor string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		return decodeView(result.Body)
	}
	view, err := a.mutate(id, actor, version, "samples_batch_added", func(s *domain.TastingSession) error { return s.AddSamplesBatch(inputs, actor, a.timestamp()) })
	if err == nil {
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	return view, err
}
func (a *Service) RemoveSample(id, sampleID, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "sample_removed", func(s *domain.TastingSession) error { return s.RemoveSample(sampleID, actor, a.timestamp()) })
}
func (a *Service) Freeze(id, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "plan_frozen", func(s *domain.TastingSession) error { return s.FreezePlan(actor, a.timestamp()) })
}
func (a *Service) Start(id, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "collection_started", func(s *domain.TastingSession) error { return s.StartCollection(actor, a.timestamp()) })
}
func (a *Service) Close(id, actor string, version int64) (*SessionView, error) {
	return a.mutate(id, actor, version, "collection_closed", func(s *domain.TastingSession) error { return s.CloseCollection(actor, a.timestamp()) })
}
func (a *Service) Submit(id string, in domain.EvaluationInput, actor string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		var view SessionView
		if err := json.Unmarshal(result.Body, &view); err == nil {
			return &view, nil
		}
	}
	var submitted *domain.Evaluation
	view, err := a.mutateWith(id, actor, version, "evaluation_submitted", func(s *domain.TastingSession) error {
		var err error
		submitted, err = s.SubmitEvaluation(in, actor, a.timestamp())
		return err
	})
	if err == nil {
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	_ = submitted
	return view, err
}
func (a *Service) Resolve(id, findingID string, resolution domain.Resolution, actor string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		var view SessionView
		if err := json.Unmarshal(result.Body, &view); err == nil {
			return &view, nil
		}
	}
	view, err := a.mutate(id, actor, version, "finding_resolved", func(s *domain.TastingSession) error {
		return s.ResolveFinding(findingID, resolution, actor, a.timestamp())
	})
	if err == nil {
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	return view, err
}
func (a *Service) ResolveBatch(id string, items []domain.FindingResolution, actor string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		return decodeView(result.Body)
	}
	view, err := a.mutate(id, actor, version, "findings_batch_resolved", func(s *domain.TastingSession) error { return s.ResolveFindingsBatch(items, actor, a.timestamp()) })
	if err == nil {
		view.Summary = resolutionSummary(view)
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	return view, err
}

func resolutionSummary(view *SessionView) *ResolutionSummary {
	result := &ResolutionSummary{}
	for _, finding := range view.Findings {
		if finding.Status == domain.FindingOpen {
			result.Open++
		} else {
			result.Resolved++
			if finding.Resolution == domain.ResolutionRework {
				result.Rework++
			}
		}
	}
	result.AllowReverify = view.Status == domain.StatusVerifying && result.Open == 0
	return result
}
func (a *Service) Approve(id, actor, role string, version int64, key string) (*SessionView, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		var view SessionView
		if err := json.Unmarshal(result.Body, &view); err == nil {
			return &view, nil
		}
	}
	view, err := a.mutate(id, actor, version, "reveal_approved", func(s *domain.TastingSession) error {
		_, err := s.ApproveReveal(actor, role, a.timestamp())
		return err
	})
	if err == nil {
		_ = a.Store.PutIdempotency(key, 200, view)
	}
	return view, err
}
func (a *Service) Seal(id, actor string, version int64, key string) (*SessionView, *archive.ArchiveReceipt, error) {
	if result, ok := a.Store.GetIdempotency(key); ok {
		var payload struct {
			View    SessionView            `json:"view"`
			Receipt archive.ArchiveReceipt `json:"receipt"`
		}
		if json.Unmarshal(result.Body, &payload) == nil {
			return &payload.View, &payload.Receipt, nil
		}
	}
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, nil, err
	}
	if session.Version != version {
		return nil, nil, repository.ErrVersionConflict
	}
	receipt, err := a.Archive.Create(session, a.Store, actor, a.timestamp())
	if err != nil {
		return nil, nil, err
	}
	if err := session.MarkSealed(receipt.ID, actor, a.timestamp()); err != nil {
		return nil, nil, err
	}
	if err := a.Store.SaveSession(session, version, "session_sealed", actor); err != nil {
		return nil, nil, err
	}
	receipt.EventChainHash = a.Store.EventChainHash()
	receipt.SessionVersion = session.Version
	if err := a.Receipts.Put(receipt); err != nil {
		return nil, nil, err
	}
	view := a.view(session, actor)
	payload := struct {
		View    *SessionView            `json:"view"`
		Receipt *archive.ArchiveReceipt `json:"receipt"`
	}{view, receipt}
	_ = a.Store.PutIdempotency(key, 200, payload)
	return view, receipt, nil
}

func (a *Service) mutate(id, actor string, version int64, action string, fn func(*domain.TastingSession) error) (*SessionView, error) {
	return a.mutateWith(id, actor, version, action, fn)
}
func (a *Service) mutateWith(id, actor string, version int64, action string, fn func(*domain.TastingSession) error) (*SessionView, error) {
	return a.mutateWithContext(context.Background(), id, actor, version, action, fn)
}
func (a *Service) mutateWithContext(ctx context.Context, id, actor string, version int64, action string, fn func(*domain.TastingSession) error) (*SessionView, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if session.Version != version {
		return nil, repository.ErrVersionConflict
	}
	if err := fn(session); err != nil {
		return nil, err
	}
	if err := a.Store.SaveSessionContext(ctx, session, version, action, actor); err != nil {
		return nil, err
	}
	return a.view(session, actor), nil
}

func (a *Service) GetSession(id, actor string) (*SessionView, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	return a.view(session, actor), nil
}
func (a *Service) ListSessions(actor string) []*SessionView {
	sessions := a.Store.ListSessions()
	out := make([]*SessionView, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, a.view(session, actor))
	}
	return out
}
func (a *Service) Tasks(id, actor string) ([]domain.BlindAssignment, error) {
	s, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	return s.ReviewerAssignments(actor)
}
func (a *Service) Receipt(id string) (*archive.ArchiveReceipt, error) {
	if r, ok := a.Receipts.Get(id); ok {
		return r, nil
	}
	return nil, domain.NewError(domain.CodeNotFound, "归档凭据不存在")
}
func (a *Service) ExportReceipt(id string) ([]byte, error) {
	r, err := a.Receipt(id)
	if err != nil {
		return nil, err
	}
	return a.Archive.Export(r)
}

func (a *Service) view(session *domain.TastingSession, actor string) *SessionView {
	copy := sessionViewClone(session)
	var mapping map[string]map[string]string
	if copy.Status == domain.StatusRevealed || copy.Status == domain.StatusSealed {
		mapping, _ = copy.RevealMapping()
	}
	if copy.Status != domain.StatusRevealed && copy.Status != domain.StatusSealed {
		for i := range copy.Assignments {
			copy.Assignments[i].SampleID = ""
		}
	}
	if copy.IsReviewer(actor) {
		copy.Samples = nil
		copy.Assignments = nil
		copy.Findings = nil
		copy.RevealApprovals = nil
	}
	return &SessionView{TastingSession: copy, Mapping: mapping}
}

func sessionViewClone(in *domain.TastingSession) *domain.TastingSession {
	b, _ := json.Marshal(in)
	var out domain.TastingSession
	_ = json.Unmarshal(b, &out)
	return &out
}
