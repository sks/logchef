package server

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mr-karan/logchef/internal/clickhouse"
	"github.com/mr-karan/logchef/internal/core"
	"github.com/mr-karan/logchef/pkg/models"

	// "github.com/mr-karan/logchef/internal/logs" // Removed

	"github.com/gofiber/fiber/v2"
)

// TimeSeriesRequest - consider if this is still needed or replaced by core/handler specific structs
// type TimeSeriesRequest struct {
// 	StartTimestamp int64                 `query:"start_timestamp"`
// 	EndTimestamp   int64                 `query:"end_timestamp"`
// 	Window         clickhouse.TimeWindow `query:"window"`
// }

// handleQueryLogs handles requests to query logs for a specific source.
// Access is controlled by the requireSourceAccess middleware.
func (s *Server) handleQueryLogs(c *fiber.Ctx) error {
	sourceIDStr := c.Params("sourceID")
	sourceID, err := core.ParseSourceID(sourceIDStr)
	if err != nil {
		return SendErrorWithType(c, fiber.StatusBadRequest, "Invalid source ID format", models.ValidationErrorType)
	}

	var req models.APIQueryRequest
	if err := c.BodyParser(&req); err != nil {
		return SendErrorWithType(c, fiber.StatusBadRequest, "Invalid request body", models.ValidationErrorType)
	}

	// Apply default limit if not specified.
	if req.Limit <= 0 {
		req.Limit = 100 // Consider making this configurable.
	}

	// Prepare parameters for the core query function.
	params := clickhouse.LogQueryParams{
		RawSQL: req.RawSQL,
		Limit:  req.Limit,
	}
	// StartTime, EndTime, and Timezone are no longer passed here;
	// they are expected to be baked into the RawSQL by the frontend.

	// Execute query via core function.
	result, err := core.QueryLogs(c.Context(), s.sqlite, s.clickhouse, s.log, sourceID, params)
	if err != nil {
		if errors.Is(err, core.ErrSourceNotFound) {
			return SendErrorWithType(c, fiber.StatusNotFound, "Source not found", models.NotFoundErrorType)
		}
		// Handle invalid query syntax errors specifically if core.QueryLogs returns them.
		// if errors.Is(err, core.ErrInvalidQuery) ...
		s.log.Error("failed to query logs via core function", slog.Any("error", err), "source_id", sourceID)
		// Pass the actual error message to the client for better debugging
		return SendErrorWithType(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to query logs: %v", err), models.DatabaseErrorType)
	}

	return SendSuccess(c, fiber.StatusOK, result)
}

// handleGetSourceSchema retrieves the schema (column names and types) for a specific source.
// Access is controlled by the requireSourceAccess middleware.
func (s *Server) handleGetSourceSchema(c *fiber.Ctx) error {
	sourceIDStr := c.Params("sourceID")
	sourceID, err := core.ParseSourceID(sourceIDStr)
	if err != nil {
		return SendErrorWithType(c, fiber.StatusBadRequest, "Invalid source ID format", models.ValidationErrorType)
	}

	// Get schema via core function.
	schema, err := core.GetSourceSchema(c.Context(), s.sqlite, s.clickhouse, s.log, sourceID)
	if err != nil {
		if errors.Is(err, core.ErrSourceNotFound) {
			return SendErrorWithType(c, fiber.StatusNotFound, "Source not found", models.NotFoundErrorType)
		}
		s.log.Error("failed to get source schema via core function", slog.Any("error", err), "source_id", sourceID)
		return SendErrorWithType(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to retrieve source schema: %v", err), models.DatabaseErrorType)
	}

	return SendSuccess(c, fiber.StatusOK, schema)
}

// handleGetHistogram generates histogram data (log counts over time intervals) for a specific source.
// Access is controlled by the requireSourceAccess middleware.
func (s *Server) handleGetHistogram(c *fiber.Ctx) error {
	sourceIDStr := c.Params("sourceID")
	sourceID, err := core.ParseSourceID(sourceIDStr)
	if err != nil {
		return SendErrorWithType(c, fiber.StatusBadRequest, "Invalid source ID format", models.ValidationErrorType)
	}

	// Parse request body containing time range, window, groupBy and optional filter query
	var req models.APIHistogramRequest
	if err := c.BodyParser(&req); err != nil {
		return SendErrorWithType(c, fiber.StatusBadRequest, "Invalid request body", models.ValidationErrorType)
	}

	// Use window from the request body or default to 1 minute
	window := req.Window
	if window == "" {
		window = "1m" // Default to 1 minute if not specified
	}

	// Prepare parameters for the core histogram function.
	params := core.HistogramParams{
		Window: window,
		Query:  req.RawSQL, // Pass raw SQL containing filters and time conditions
	}

	// Only add groupBy if it's not empty
	if req.GroupBy != "" && strings.TrimSpace(req.GroupBy) != "" {
		params.GroupBy = req.GroupBy
	}

	// Use the provided timezone or default to UTC
	if req.Timezone != "" {
		params.Timezone = req.Timezone
	} else {
		params.Timezone = "UTC"
	}

	// Execute histogram query via core function.
	result, err := core.GetHistogramData(c.Context(), s.sqlite, s.clickhouse, s.log, sourceID, params)
	if err != nil {
		if errors.Is(err, core.ErrSourceNotFound) {
			return SendErrorWithType(c, fiber.StatusNotFound, "Source not found", models.NotFoundErrorType)
		}

		// Check if the error is related to an invalid window parameter
		if strings.Contains(err.Error(), "invalid histogram window") {
			return SendErrorWithType(c, fiber.StatusBadRequest, err.Error(), models.ValidationErrorType)
		}

		// Handle other errors
		s.log.Error("failed to get histogram data via core function", slog.Any("error", err), "source_id", sourceID)
		// Pass the actual error message to the client for better debugging
		return SendErrorWithType(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to generate histogram data: %v", err), models.DatabaseErrorType)
	}

	return SendSuccess(c, fiber.StatusOK, result)
}
