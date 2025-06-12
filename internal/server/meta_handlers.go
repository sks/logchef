package server

import (
	"github.com/gofiber/fiber/v2"
)

// --- Meta Handlers ---

// MetaResponse represents the server metadata response
type MetaResponse struct {
	Version           string `json:"version"`
	HTTPServerTimeout string `json:"http_server_timeout"`
}

// handleGetMeta returns server metadata including version and configuration
// URL: GET /api/v1/meta
// Public endpoint - no authentication required
// @Summary Get server metadata
// @Description Returns server metadata including version and configuration information
// @Tags meta
// @Accept json
// @Produce json
// @Success 200 {object} MetaResponse "Server metadata"
// @Router /meta [get]
func (s *Server) handleGetMeta(c *fiber.Ctx) error {
	meta := MetaResponse{
		Version:           s.version,
		HTTPServerTimeout: s.config.Server.HTTPServerTimeout.String(),
	}

	return SendSuccess(c, fiber.StatusOK, meta)
}
