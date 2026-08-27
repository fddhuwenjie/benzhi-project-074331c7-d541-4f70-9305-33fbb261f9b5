package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/workflow"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	store, err := repository.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(workflow.New(store, validation.NewEngine()))
}

func TestWorkbenchAndSecurityHeaders(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "触觉导览图制版校验工作台") {
		t.Fatalf("工作台响应异常: %d", resp.Code)
	}
	if resp.Header().Get("Content-Security-Policy") == "" || resp.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("安全响应头缺失")
	}
}

func TestCreateProjectRejectsUnknownJSONField(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui-api/projects", strings.NewReader(`{"project_id":"x","unknown":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("未知字段应返回 400，得到 %d", resp.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil || payload["error"] == "" {
		t.Fatal("错误响应不是结构化 JSON")
	}
}
