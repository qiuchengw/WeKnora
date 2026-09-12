package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeRerankModelService is the smallest ModelService subset needed by
// knowledgeBaseService.applyRerank: GetRerankModel. Every other method is
// panic-only via the embedded interface — none are exercised here.
type fakeRerankModelService struct {
	interfaces.ModelService
	model   rerank.Reranker
	failGet bool
}

func (f *fakeRerankModelService) GetRerankModel(_ context.Context, modelID string) (rerank.Reranker, error) {
	if f.failGet {
		return nil, fmt.Errorf("model %s unavailable", modelID)
	}
	return f.model, nil
}

// fakeReranker mimics a cross-encoder: it scores every passage and returns them
// sorted by relevance descending, each carrying its input index.
type fakeReranker struct {
	scores []float64
	err    error
	// seen captures the passages handed to the model (passage-construction assertions).
	seen []string
}

func (f *fakeReranker) Rerank(_ context.Context, _ string, documents []string) ([]rerank.RankResult, error) {
	f.seen = append([]string(nil), documents...)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]rerank.RankResult, 0, len(documents))
	for i := range documents {
		if i < len(f.scores) {
			out = append(out, rerank.RankResult{Index: i, RelevanceScore: f.scores[i]})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].RelevanceScore > out[b].RelevanceScore })
	return out, nil
}

func (f *fakeReranker) GetModelName() string { return "fake-rerank" }
func (f *fakeReranker) GetModelID() string   { return "fake-rerank-id" }

func resultsForRerank(n int) []*types.SearchResult {
	out := make([]*types.SearchResult, n)
	for i := range n {
		out[i] = &types.SearchResult{Content: fmt.Sprintf("passage %d", i)}
	}
	return out
}

func contentsOf(results []*types.SearchResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Content
	}
	return out
}

func TestApplyRerank_ReordersByModelScore(t *testing.T) {
	svc := newKBSvcForValidate(&fakeRerankModelService{
		model: &fakeReranker{scores: []float64{0.9, 0.1, 0.5}},
	})
	cfg := &types.RetrievalConfig{RerankModelID: "r1"}

	got := svc.applyRerank(context.Background(), cfg, "q", resultsForRerank(3))

	want := []string{"passage 0", "passage 2", "passage 1"}
	for i, w := range want {
		if got[i].Content != w {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, got[i].Content, w, contentsOf(got))
		}
	}
}

func TestApplyRerank_PassageIncludesKnowledgeTitle(t *testing.T) {
	model := &fakeReranker{scores: []float64{0.9, 0.9}}
	svc := newKBSvcForValidate(&fakeRerankModelService{model: model})
	results := []*types.SearchResult{
		{KnowledgeTitle: "影子灰度策略", Content: "先让一小部分流量走新链路。"},
		{Content: "无标题片段"},
	}

	svc.applyRerank(context.Background(), &types.RetrievalConfig{RerankModelID: "r1"}, "影子灰度策略", results)

	if len(model.seen) != 2 {
		t.Fatalf("expected 2 passages, got %d: %v", len(model.seen), model.seen)
	}
	if !strings.Contains(model.seen[0], "影子灰度策略") || !strings.Contains(model.seen[0], "先让一小部分流量走新链路") {
		t.Fatalf("passage must carry title + content, got %q", model.seen[0])
	}
	if model.seen[1] != "无标题片段" {
		t.Fatalf("titleless passage must pass through unchanged, got %q", model.seen[1])
	}
}

func TestApplyRerank_DropsScoresBelowThreshold(t *testing.T) {
	svc := newKBSvcForFinding(t)
	svc.modelService = &fakeRerankModelService{
		model: &fakeReranker{scores: []float64{0.9, 0.05}},
	}
	// RerankThreshold is applied as-is for a non-nil config (the 0.2 default only
	// applies to a nil config), so an explicit threshold is required to drop the
	// low-scoring hit.
	cfg := &types.RetrievalConfig{RerankModelID: "r1", RerankThreshold: 0.2}

	got := svc.applyRerank(context.Background(), cfg, "q", resultsForRerank(2))

	if len(got) != 1 || got[0].Content != "passage 0" {
		t.Fatalf("got %v, want only the high-scoring passage", contentsOf(got))
	}
}

func TestApplyRerank_NoModelConfiguredKeepsFusedOrder(t *testing.T) {
	svc := newKBSvcForValidate(&fakeRerankModelService{
		model: &fakeReranker{scores: []float64{0.9, 0.1}},
	})

	for name, cfg := range map[string]*types.RetrievalConfig{
		"nil config":  nil,
		"empty model": {RerankModelID: ""},
	} {
		got := svc.applyRerank(context.Background(), cfg, "q", resultsForRerank(2))
		if contents := contentsOf(got); contents[0] != "passage 0" || contents[1] != "passage 1" {
			t.Fatalf("%s: fused order must be preserved, got %v", name, contents)
		}
	}
}

func TestApplyRerank_FailuresKeepFusedOrder(t *testing.T) {
	cases := map[string]interfaces.ModelService{
		"model lookup failure": &fakeRerankModelService{failGet: true},
		"model call failure":   &fakeRerankModelService{model: &fakeReranker{err: fmt.Errorf("boom")}},
	}
	for name, svcModel := range cases {
		svc := newKBSvcForValidate(svcModel)
		got := svc.applyRerank(context.Background(), &types.RetrievalConfig{RerankModelID: "r1"}, "q", resultsForRerank(2))
		if contents := contentsOf(got); contents[0] != "passage 0" || contents[1] != "passage 1" {
			t.Fatalf("%s: fused order must be preserved, got %v", name, contents)
		}
	}
}

func TestApplyRerank_KeepsTailBeyondRerankTopK(t *testing.T) {
	svc := newKBSvcForValidate(&fakeRerankModelService{
		model: &fakeReranker{scores: []float64{0.9, 0.8}},
	})
	cfg := &types.RetrievalConfig{RerankModelID: "r1", RerankTopK: 2}

	got := svc.applyRerank(context.Background(), cfg, "q", resultsForRerank(4))

	if len(got) != 4 {
		t.Fatalf("got %d results, want 4 (reranked prefix + untouched tail)", len(got))
	}
	if got[2].Content != "passage 2" || got[3].Content != "passage 3" {
		t.Fatalf("tail beyond RerankTopK must keep fused order, got %v", contentsOf(got))
	}
}

// newKBSvcForFinding is newKBSvcForValidate with an explicit name for tests that
// mutate the service after construction.
func newKBSvcForFinding(t *testing.T) *knowledgeBaseService {
	t.Helper()
	return newKBSvcForValidate(nil)
}
