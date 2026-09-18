package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// The projection exists so `hybrid-search` can hand out `content_revision` (the
// citation backlink field) without serializing types.SearchResult.ContentRevision,
// which stored message payloads must keep at zero. These tests pin the three
// properties that make it safe: every original field survives, the new field is
// always present (0 included), and nil slots stay nil.

func decodeProjection(t *testing.T, results []*types.SearchResult) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(projectHybridSearchResults(results))
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal projection: %v", err)
	}
	return decoded
}

func TestProjectHybridSearchResultsKeepsEveryOriginalField(t *testing.T) {
	result := &types.SearchResult{
		ID:             "chunk-1",
		Content:        "正文",
		KnowledgeID:    "knowledge-1",
		KnowledgeTitle: "标题",
		ChunkIndex:     2,
		StartAt:        10,
		EndAt:          42,
		Score:          0.75,
		MatchType:      types.MatchTypeEmbedding,
	}
	original, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal original: %v", err)
	}
	var originalFields map[string]any
	if err := json.Unmarshal(original, &originalFields); err != nil {
		t.Fatalf("unmarshal original: %v", err)
	}

	projected := decodeProjection(t, []*types.SearchResult{result})
	if len(projected) != 1 {
		t.Fatalf("expected 1 projected result, got %d", len(projected))
	}
	for field, want := range originalFields {
		got, ok := projected[0][field]
		if !ok {
			t.Errorf("projection dropped field %q", field)
			continue
		}
		if got != want {
			t.Errorf("field %q changed: got %v, want %v", field, got, want)
		}
	}
	// 内部字段仍然不外泄（它的 tag 没被改）
	if _, leaked := projected[0]["ContentRevision"]; leaked {
		t.Error("internal ContentRevision must not be serialized")
	}
}

func TestProjectHybridSearchResultsAlwaysEmitsContentRevision(t *testing.T) {
	projected := decodeProjection(t, []*types.SearchResult{
		{ID: "never-edited", ContentRevision: 0},
		{ID: "edited", ContentRevision: 3},
	})
	if len(projected) != 2 {
		t.Fatalf("expected 2 results, got %d", len(projected))
	}
	// 0 必须出现：消费方要能区分"从未编辑"与"服务端没说"
	if got, ok := projected[0]["content_revision"]; !ok || got != float64(0) {
		t.Errorf("revision 0 must be emitted as 0, got %v (present=%v)", got, ok)
	}
	if got := projected[1]["content_revision"]; got != float64(3) {
		t.Errorf("revision passthrough failed: got %v", got)
	}
}

func TestProjectHybridSearchResultsKeepsNilSlots(t *testing.T) {
	raw, err := json.Marshal(projectHybridSearchResults([]*types.SearchResult{
		{ID: "a"},
		nil,
	}))
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	if !strings.Contains(string(raw), "null") {
		t.Errorf("nil slot must stay null (byte-faithful to the pre-projection response), got %s", raw)
	}
}

// Endpoint-level guard: the HTTP response actually carries the field (the unit
// tests above only cover the projection function).
func TestHybridSearchEndpointExposesContentRevision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &hybridSearchTestService{
		results: []*types.SearchResult{{ID: "chunk-1", Content: "正文", ContentRevision: 2}},
	}
	response := performHybridSearchRequest(svc, `{"query_text":"MiniMax","match_count":3}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", response.Code, response.Body.String())
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(envelope.Data))
	}
	if got := envelope.Data[0]["content_revision"]; got != float64(2) {
		t.Errorf("hybrid-search response must expose content_revision, got %v", got)
	}
	if got := envelope.Data[0]["content"]; got != "正文" {
		t.Errorf("existing fields must survive the projection, got %v", got)
	}
}
