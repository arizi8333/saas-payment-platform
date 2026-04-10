package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/service"
)

// AdminHandler handles HTTP requests for admin dashboard operations.
type AdminHandler struct {
	adminService service.AdminService
}

// NewAdminHandler creates a new AdminHandler with the given AdminService.
func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// RegisterRoutes registers all admin-related routes on the given router.
func (h *AdminHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	admin := router.Group("/admin", authMW.JWTProtected(), authMW.AdminOnly())
	admin.Get("/stats", h.GetDashboardStats)
}

// GetDashboardStats godoc
// @Summary      Get admin dashboard statistics
// @Description  Returns aggregated platform statistics including transaction counts by status, active user count, and webhook delivery success rate. Requires admin role.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.APIResponse{data=dto.DashboardStatsResponse}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      403  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/admin/stats [get]
func (h *AdminHandler) GetDashboardStats(c *fiber.Ctx) error {
	result, err := h.adminService.GetDashboardStats(c.UserContext())
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}
