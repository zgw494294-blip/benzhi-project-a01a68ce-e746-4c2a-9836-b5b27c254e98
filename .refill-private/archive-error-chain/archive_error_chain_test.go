package archive_error_chain_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/httpui"
	"sensory-blind-review/internal/repository"
)

func TestArchiveErrorsPreserveNotFoundChain(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpui.New(application.New(store, archive.NewService(), archive.NewRegistry())).Handler()
	paths := []string{
		"/api/archives/missing-receipt/validate",
		"/api/archives/missing-receipt/package",
	}
	for _, path := range paths {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		var payload struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("%s 返回无效 JSON: %v", path, err)
		}
		if response.Code != http.StatusNotFound || payload.Error.Code != "not_found" {
			t.Fatalf("%s 应保持 404/not_found，实际为 %d/%s", path, response.Code, payload.Error.Code)
		}
	}
}
