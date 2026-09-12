package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// KnowledgeDataQueryHandler serves the agent-less tabular-data capability
// endpoints (ADR-0019 D-18 E2/E3):
//
//	POST /knowledge/:id/data-schema    — table metadata recorded at ingestion
//	POST /knowledge/:id/data-analysis  — read-only SQL over the document's data
//
// Authorization reuses the knowledge RBAC chain (viewer + KB read), so the
// endpoints grant nothing the caller could not already read.
type KnowledgeDataQueryHandler struct {
	service interfaces.KnowledgeDataQueryService
}

// NewKnowledgeDataQueryHandler wires the capability handler.
func NewKnowledgeDataQueryHandler(service interfaces.KnowledgeDataQueryService) *KnowledgeDataQueryHandler {
	return &KnowledgeDataQueryHandler{service: service}
}

type dataAnalysisRequest struct {
	SQL string `json:"sql" binding:"required"`
}

// DataSchema returns the table shape of one document so a caller can plan SQL.
func (h *KnowledgeDataQueryHandler) DataSchema(c *gin.Context) {
	knowledgeID := c.Param("id")
	result, err := h.service.DataSchema(c.Request.Context(), knowledgeID)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError("Failed to read table metadata").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// DataAnalysis executes a read-only statement over one document.
func (h *KnowledgeDataQueryHandler) DataAnalysis(c *gin.Context) {
	knowledgeID := c.Param("id")
	var req dataAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("sql is required").WithDetails(err.Error()))
		return
	}
	result, err := h.service.DataAnalysis(c.Request.Context(), knowledgeID, req.SQL)
	if err != nil {
		logger.Warnf(c.Request.Context(), "Tabular data analysis failed for knowledge %s: %v", knowledgeID, err)
		// The document could not be materialized/loaded (not tabular, missing
		// file): a caller-visible condition rather than an internal failure.
		_ = c.Error(apperrors.NewBadRequestError("Document cannot be analyzed").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
