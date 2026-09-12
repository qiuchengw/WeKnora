package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// KnowledgeDataQueryService exposes the engine's tabular-data capabilities for a
// single ingested document, without an agent in the call path.
//
// The endpoints that back this service exist so an external orchestrator can plan
// a query itself and still have the engine do the execution — keeping exactly one
// implementation of the data stack (ADR-0019 D-16/D-18 E2/E3).
//
//   - DataSchema returns what ingestion recorded about the document's table
//     (table name, columns, row count) so a caller can write SQL against it.
//   - DataAnalysis executes a read-only SQL statement over that document's data
//     in the process-wide DuckDB instance.
//
// Both are the same code paths the built-in agent's data_analysis / data_schema
// tools use; the service only supplies the caller's scope and removes the agent
// planning loop.
type KnowledgeDataQueryService interface {
	// DataSchema returns the table metadata of one document.
	DataSchema(ctx context.Context, knowledgeID string) (*types.ToolResult, error)
	// DataAnalysis executes sqlText (read-only) over one document.
	DataAnalysis(ctx context.Context, knowledgeID string, sqlText string) (*types.ToolResult, error)
}
