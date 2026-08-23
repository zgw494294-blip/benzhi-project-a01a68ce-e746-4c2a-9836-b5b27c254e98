package httpui

import (
	"net/http"
	"net/http/httptest"
	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/repository"
	"strings"
	"testing"
)

func TestIndexAndHealth(t *testing.T) {
	store, _ := repository.Open(t.TempDir())
	handler := New(application.New(store, archive.NewService(), archive.NewRegistry())).Handler()
	for _, path := range []string{"/", "/healthz", "/assets/app.js"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s 状态=%d", path, w.Code)
		}
		if strings.TrimSpace(w.Body.String()) == "" {
			t.Fatalf("%s 返回空正文", path)
		}
	}
}
