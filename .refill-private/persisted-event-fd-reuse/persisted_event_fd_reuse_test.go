package persistedeventfdreuse_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sensory-blind-review/internal/application"
	"sensory-blind-review/internal/archive"
	"sensory-blind-review/internal/httpui"
	"sensory-blind-review/internal/repository"
)

func TestArchiveValidationReopensReplacedEventLog(t *testing.T) {
	dir := t.TempDir()
	store, err := repository.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := archive.NewRegistryAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store, archive.NewService(), receipts)
	if err := httpui.RunSelfcheck(app); err != nil {
		t.Fatalf("准备封存会话失败: %v", err)
	}

	registered := app.ListReceipts()
	if len(registered) != 1 {
		t.Fatalf("归档凭据数量=%d，期望 1", len(registered))
	}
	eventPath := filepath.Join(dir, "events.jsonl")
	original, err := os.ReadFile(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(original, []byte(`"actor":"host"`), []byte(`"actor":"tampered"`), 1)
	if bytes.Equal(tampered, original) {
		t.Fatal("事件日志中没有可替换的主持人事件")
	}
	replacement := filepath.Join(dir, "events.replacement.jsonl")
	if err := os.WriteFile(replacement, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, eventPath); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/archives/"+registered[0].ID+"/validate", nil)
	response := httptest.NewRecorder()
	httpui.New(app).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("校验响应状态=%d，响应=%s", response.Code, response.Body.String())
	}
	var report archive.ValidationReport
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatalf("解析校验响应失败: %v", err)
	}
	if report.Valid {
		t.Fatalf("事件日志被原子替换并篡改后仍返回 valid=true")
	}
}
