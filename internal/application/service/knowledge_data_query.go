package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// knowledgeDataQueryService implements the agent-less tabular-data capability
// endpoints (ADR-0019 D-18 E2/E3). It deliberately reuses the built-in agent's
// tools instead of reimplementing DuckDB access, so there is a single execution
// path for data questions whether they arrive through the agent or an HTTP
// capability call.
type knowledgeDataQueryService struct {
	knowledgeBaseService interfaces.KnowledgeBaseService
	knowledgeService     interfaces.KnowledgeService
	tenantService        interfaces.TenantService
	fileService          interfaces.FileService
	chunkService         interfaces.ChunkService
	duckdb               *sql.DB
	storageResolver      interfaces.StorageBackendResolver
}

// NewKnowledgeDataQueryService wires the tabular-data capability service.
func NewKnowledgeDataQueryService(
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	tenantService interfaces.TenantService,
	fileService interfaces.FileService,
	chunkService interfaces.ChunkService,
	duckdb *sql.DB,
	storageResolver interfaces.StorageBackendResolver,
) interfaces.KnowledgeDataQueryService {
	return &knowledgeDataQueryService{
		knowledgeBaseService: knowledgeBaseService,
		knowledgeService:     knowledgeService,
		tenantService:        tenantService,
		fileService:          fileService,
		chunkService:         chunkService,
		duckdb:               duckdb,
		storageResolver:      storageResolver,
	}
}

// searchTargetsForKnowledge builds the same request scope the agent path would
// pass to these tools: exactly one KB and one document. The tools then authorize
// against that scope, so an id from another tenant cannot be reached even if the
// handler-level RBAC guard were ever bypassed.
func searchTargetsForKnowledge(knowledge *types.Knowledge) types.SearchTargets {
	return types.SearchTargets{{
		Type:            types.SearchTargetTypeKnowledge,
		TenantID:        knowledge.TenantID,
		KnowledgeBaseID: knowledge.KnowledgeBaseID,
		KnowledgeIDs:    []string{knowledge.ID},
	}}
}

func (s *knowledgeDataQueryService) newDataAnalysisTool(knowledge *types.Knowledge) *tools.DataAnalysisTool {
	// One DuckDB session per request keeps concurrently analyzed documents from
	// sharing a namespace; Cleanup drops the tables the request created.
	sessionID := fmt.Sprintf("data_query_%s", knowledge.ID)
	return tools.NewDataAnalysisTool(
		s.knowledgeBaseService,
		s.knowledgeService,
		s.tenantService,
		s.fileService,
		s.duckdb,
		sessionID,
		s.storageResolver,
	).WithSearchTargets(searchTargetsForKnowledge(knowledge))
}

// DataSchema returns the table metadata recorded for one document.
func (s *knowledgeDataQueryService) DataSchema(ctx context.Context, knowledgeID string) (*types.ToolResult, error) {
	knowledge, err := s.knowledgeService.GetKnowledgeByID(ctx, knowledgeID)
	if err != nil {
		return nil, err
	}
	tool := tools.NewDataSchemaTool(s.knowledgeService, s.chunkService.GetRepository()).
		WithSearchTargets(searchTargetsForKnowledge(knowledge))
	args, err := json.Marshal(tools.DataSchemaInput{KnowledgeID: knowledgeID})
	if err != nil {
		return nil, err
	}
	return tool.Execute(ctx, args)
}

// DataAnalysis loads one document into DuckDB and executes a read-only statement
// against it. Statement validation (SELECT/SHOW/DESCRIBE/EXPLAIN/PRAGMA only)
// stays inside the shared tool, so the capability endpoint cannot drift from the
// agent path.
func (s *knowledgeDataQueryService) DataAnalysis(ctx context.Context, knowledgeID string, sqlText string) (*types.ToolResult, error) {
	knowledge, err := s.knowledgeService.GetKnowledgeByID(ctx, knowledgeID)
	if err != nil {
		return nil, err
	}
	tool := s.newDataAnalysisTool(knowledge)
	defer tool.Cleanup(ctx)

	if _, err := tool.LoadFromKnowledge(ctx, knowledge); err != nil {
		// A non-tabular or unreadable document is a caller-visible condition, not
		// an internal failure; the handler maps it to 400.
		return nil, fmt.Errorf("document %s cannot be loaded for analysis: %w", knowledgeID, err)
	}
	// 字段名跟随上游（`tools.DataAnalysisInput.SQL`；JSON 键仍是 `sql`）：上游已把同一能力收进主干，
	// 车道这份是残留 ⇒ 用上游命名，避免"文本合并干净、编译才炸"的漂移（2026-09-18 同步实测）。
	args, err := json.Marshal(tools.DataAnalysisInput{KnowledgeID: knowledgeID, SQL: sqlText})
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Tabular data analysis via capability endpoint, knowledge: %s", knowledgeID)
	return tool.Execute(ctx, args)
}
