package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/domain"
	"sensory-blind-review/internal/repository"
)

type Server struct {
	app *application.Service
	mux *http.ServeMux
}

func New(app *application.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	s.registerAssets()
	return s
}
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /readyz", s.readiness)
	s.mux.HandleFunc("GET /api/sessions", s.listSessions)
	s.mux.HandleFunc("POST /api/sessions", s.createSession)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	s.mux.HandleFunc("PATCH /api/sessions/{id}", s.configureSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/samples", s.addSample)
	s.mux.HandleFunc("POST /api/sessions/{id}/samples/batch", s.addSamplesBatch)
	s.mux.HandleFunc("PATCH /api/sessions/{id}/samples/{sampleID}", s.updateSample)
	s.mux.HandleFunc("DELETE /api/sessions/{id}/samples/{sampleID}", s.removeSample)
	s.mux.HandleFunc("POST /api/sessions/{id}/freeze", s.freeze)
	s.mux.HandleFunc("POST /api/sessions/{id}/start", s.start)
	s.mux.HandleFunc("GET /api/sessions/{id}/tasks", s.tasks)
	s.mux.HandleFunc("GET /api/sessions/{id}/reviewer-tasks", s.reviewerTasks)
	s.mux.HandleFunc("GET /api/sessions/{id}/progress", s.progress)
	s.mux.HandleFunc("GET /api/sessions/{id}/actions", s.actions)
	s.mux.HandleFunc("POST /api/sessions/{id}/evaluations", s.submit)
	s.mux.HandleFunc("POST /api/sessions/{id}/close", s.close)
	s.mux.HandleFunc("POST /api/sessions/{id}/findings/{findingID}/resolve", s.resolve)
	s.mux.HandleFunc("POST /api/sessions/{id}/findings/batch-resolve", s.resolveBatch)
	s.mux.HandleFunc("POST /api/sessions/{id}/findings/resolve-batch", s.resolveBatch)
	s.mux.HandleFunc("POST /api/sessions/{id}/findings/resolve", s.resolveBatch)
	s.mux.HandleFunc("POST /api/sessions/{id}/reverify", s.reverify)
	s.mux.HandleFunc("GET /api/sessions/{id}/integrity", s.integrity)
	s.mux.HandleFunc("GET /api/sessions/{id}/audit", s.audit)
	s.mux.HandleFunc("POST /api/sessions/{id}/reveal/approve", s.approve)
	s.mux.HandleFunc("POST /api/sessions/{id}/seal", s.seal)
	s.mux.HandleFunc("GET /api/archives/{id}", s.archive)
	s.mux.HandleFunc("GET /api/archives/{id}/validate", s.validateArchive)
	s.mux.HandleFunc("GET /api/archives/{id}/integrity", s.validateArchive)
	s.mux.HandleFunc("GET /api/archives", s.listArchives)
	s.mux.HandleFunc("GET /api/archives/{id}/export", s.exportArchive)
	s.mux.HandleFunc("GET /api/archives/{id}/package", s.exportPackage)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, _ := assetFS.ReadFile("assets/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, string(data))
}
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func actor(r *http.Request) string {
	value := r.Header.Get("X-User-ID")
	if value == "" {
		value = r.URL.Query().Get("user")
	}
	return strings.TrimSpace(value)
}
func version(r *http.Request) int64 {
	value, _ := strconv.ParseInt(r.Header.Get("X-Expected-Version"), 10, 64)
	if value == 0 {
		value, _ = strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	}
	return value
}
func idem(r *http.Request) string { return strings.TrimSpace(r.Header.Get("Idempotency-Key")) }
func decode(r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(target)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func handleError(w http.ResponseWriter, err error) {
	status := 500
	switch {
	case errors.Is(err, repository.ErrVersionConflict):
		status = 409
	case domain.ErrorCodeOf(err) == domain.CodeValidation:
		status = 400
	case domain.ErrorCodeOf(err) == domain.CodeConflict:
		status = 409
	case domain.ErrorCodeOf(err) == domain.CodeForbidden:
		status = 403
	case domain.ErrorCodeOf(err) == domain.CodeNotFound:
		status = 404
	case domain.ErrorCodeOf(err) == domain.CodeState:
		status = 422
	}
	payload := map[string]any{"code": domain.ErrorCodeOf(err), "message": err.Error()}
	var detailed *domain.DomainError
	if errors.As(err, &detailed) && len(detailed.Details) > 0 {
		payload["details"] = detailed.Details
	}
	writeJSON(w, status, map[string]any{"error": payload})
}
func (s *Server) id(r *http.Request) string { return r.PathValue("id") }

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"sessions": s.app.ListSessions(actor(r))})
}
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var cfg domain.SessionConfig
	if err := decode(r, &cfg); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.CreateSession(cfg, actor(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 201, view)
}
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.GetSession(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) configureSession(w http.ResponseWriter, r *http.Request) {
	var cfg domain.SessionConfig
	if err := decode(r, &cfg); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.Configure(s.id(r), cfg, actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) addSample(w http.ResponseWriter, r *http.Request) {
	var in domain.SampleInput
	if err := decode(r, &in); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.AddSampleContext(r.Context(), s.id(r), in, actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) addSamplesBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Samples []domain.SampleInput `json:"samples"`
	}
	if err := decode(r, &body); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.AddSamplesBatch(s.id(r), body.Samples, actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) removeSample(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.RemoveSample(s.id(r), r.PathValue("sampleID"), actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) freeze(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.Freeze(s.id(r), actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.Start(s.id(r), actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.app.Tasks(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"tasks": tasks})
}
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	var in domain.EvaluationInput
	if err := decode(r, &in); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.Submit(s.id(r), in, actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) close(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.Close(s.id(r), actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) resolve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Resolution domain.Resolution `json:"resolution"`
	}
	if err := decode(r, &body); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.Resolve(s.id(r), r.PathValue("findingID"), body.Resolution, actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) resolveBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Resolutions []domain.FindingResolution `json:"resolutions"`
		Findings    []domain.FindingResolution `json:"findings"`
	}
	if err := decode(r, &body); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	items := body.Resolutions
	if len(items) == 0 {
		items = body.Findings
	}
	view, err := s.app.ResolveBatch(s.id(r), items, actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if err := decode(r, &body); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.Approve(s.id(r), actor(r), body.Role, version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) seal(w http.ResponseWriter, r *http.Request) {
	view, receipt, err := s.app.Seal(s.id(r), actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"session": view, "receipt": receipt})
}
func (s *Server) archive(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.app.Receipt(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, receipt)
}
func (s *Server) exportArchive(w http.ResponseWriter, r *http.Request) {
	body, err := s.app.ExportReceipt(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}
