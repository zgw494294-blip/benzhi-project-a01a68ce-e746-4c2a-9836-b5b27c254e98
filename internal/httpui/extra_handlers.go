package httpui

import (
	"net/http"
	"strings"

	"sensory-blind-review/internal/domain"
)

func (s *Server) updateSample(w http.ResponseWriter, r *http.Request) {
	var in domain.SampleInput
	if err := decode(r, &in); err != nil {
		handleError(w, domain.NewError(domain.CodeValidation, "请求 JSON 无效"))
		return
	}
	view, err := s.app.UpdateSample(s.id(r), r.PathValue("sampleID"), in, actor(r), version(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) progress(w http.ResponseWriter, r *http.Request) {
	value, err := s.app.Progress(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) reviewerTasks(w http.ResponseWriter, r *http.Request) {
	value, err := s.app.ReviewerTasks(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"tasks": value})
}
func (s *Server) reverify(w http.ResponseWriter, r *http.Request) {
	view, err := s.app.Reverify(s.id(r), actor(r), version(r), idem(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) integrity(w http.ResponseWriter, r *http.Request) {
	issues, err := s.app.Integrity(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"valid": len(issues) == 0, "issues": issues})
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	include := strings.EqualFold(r.URL.Query().Get("includeEvents"), "true")
	view, err := s.app.Audit(s.id(r), actor(r), r.URL.Query().Get("actor"), r.URL.Query().Get("action"), include)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (s *Server) readiness(w http.ResponseWriter, r *http.Request) {
	health := s.app.StoreHealth()
	status := http.StatusOK
	if !health.Valid {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, health)
}
func (s *Server) listArchives(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"archives": s.app.ListReceipts()})
}
func (s *Server) actions(w http.ResponseWriter, r *http.Request) {
	actions, err := s.app.Actions(s.id(r), actor(r))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"actions": actions})
}
func (s *Server) exportPackage(w http.ResponseWriter, r *http.Request) {
	body, err := s.app.ExportPackage(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=archive-package.json")
	w.Write(body)
}

func (s *Server) validateArchive(w http.ResponseWriter, r *http.Request) {
	report, err := s.app.ValidateArchive(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, 200, report)
}
