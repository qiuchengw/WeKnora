package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubKnowledgeDataQuery stands in for the capability service: it records the
// arguments the handler passed down and returns a canned result or error.
type stubKnowledgeDataQuery struct {
	interfaces.KnowledgeDataQueryService
	schemaKnowledgeID string
	analysisKnowledge string
	analysisSQL       string
	result            *types.ToolResult
	err               error
}

func (s *stubKnowledgeDataQuery) DataSchema(_ context.Context, knowledgeID string) (*types.ToolResult, error) {
	s.schemaKnowledgeID = knowledgeID
	return s.result, s.err
}

func (s *stubKnowledgeDataQuery) DataAnalysis(_ context.Context, knowledgeID string, sqlText string) (*types.ToolResult, error) {
	s.analysisKnowledge = knowledgeID
	s.analysisSQL = sqlText
	return s.result, s.err
}

func newDataQueryTestEngine(svc interfaces.KnowledgeDataQueryService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The production envelope/status for recorded c.Error() values comes from
	// ErrorHandler; reusing it keeps this test honest about status codes.
	r.Use(middleware.ErrorHandler())
	h := NewKnowledgeDataQueryHandler(svc)
	r.GET("/knowledge/:id/data-schema", h.DataSchema)
	r.POST("/knowledge/:id/data-analysis", h.DataAnalysis)
	return r
}

func okToolResult() *types.ToolResult {
	return &types.ToolResult{
		Success: true,
		Output:  "table: d1\ncolumns: region, amount",
		Data:    map[string]interface{}{"row_count": 2},
	}
}

func TestDataSchemaReturnsToolResultForRequestedKnowledge(t *testing.T) {
	stub := &stubKnowledgeDataQuery{result: okToolResult()}
	w := httptest.NewRecorder()
	newDataQueryTestEngine(stub).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/knowledge/k1/data-schema", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if stub.schemaKnowledgeID != "k1" {
		t.Fatalf("service received knowledge %q, want %q", stub.schemaKnowledgeID, "k1")
	}
	var body struct {
		Success bool              `json:"success"`
		Data    *types.ToolResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v (body %s)", err, w.Body.String())
	}
	if !body.Success || body.Data == nil || body.Data.Output != "table: d1\ncolumns: region, amount" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDataAnalysisRejectsMissingSQL(t *testing.T) {
	stub := &stubKnowledgeDataQuery{result: okToolResult()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge/k1/data-analysis", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	newDataQueryTestEngine(stub).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	if stub.analysisSQL != "" {
		t.Fatalf("service must not be called without sql, saw %q", stub.analysisSQL)
	}
}

func TestDataAnalysisPassesSQLAndKnowledgeThrough(t *testing.T) {
	stub := &stubKnowledgeDataQuery{result: okToolResult()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge/k7/data-analysis", strings.NewReader(`{"sql":"select region, sum(amount) from d7 group by 1"}`))
	req.Header.Set("Content-Type", "application/json")
	newDataQueryTestEngine(stub).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if stub.analysisKnowledge != "k7" || stub.analysisSQL != "select region, sum(amount) from d7 group by 1" {
		t.Fatalf("service received (%q, %q)", stub.analysisKnowledge, stub.analysisSQL)
	}
}

func TestDataAnalysisReportsUnanalyzableDocumentAsBadRequest(t *testing.T) {
	stub := &stubKnowledgeDataQuery{err: fmt.Errorf("document k9 cannot be loaded for analysis: unsupported file type")}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/knowledge/k9/data-analysis", strings.NewReader(`{"sql":"select 1"}`))
	req.Header.Set("Content-Type", "application/json")
	newDataQueryTestEngine(stub).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "cannot be analyzed") {
		t.Fatalf("body should explain the document cannot be analyzed: %s", w.Body.String())
	}
}
