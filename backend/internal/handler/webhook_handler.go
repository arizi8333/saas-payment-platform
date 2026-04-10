package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/pkg/validator"
	"github.com/saas-payment-platform/backend/internal/service"
)

// WebhookHandler handles HTTP requests for webhook management.
type WebhookHandler struct {
	webhookService service.WebhookService
}

// NewWebhookHandler creates a new WebhookHandler with the given WebhookService.
func NewWebhookHandler(webhookService service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

// RegisterRoutes registers all webhook-related routes on the given router.
func (h *WebhookHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	webhooks := router.Group("/webhooks", authMW.JWTProtected())
	webhooks.Post("/endpoints", h.RegisterEndpoint)
	webhooks.Get("/endpoints", h.ListEndpoints)
	webhooks.Delete("/endpoints/:id", h.DeleteEndpoint)
	webhooks.Get("/deliveries", h.ListDeliveries)
}

// RegisterEndpoint godoc
// @Summary      Register a webhook endpoint
// @Description  Registers a new webhook endpoint with URL, secret for HMAC signature, and event subscriptions. The secret is returned only once on creation.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateWebhookRequest  true  "Webhook endpoint data"
// @Success      201   {object}  response.APIResponse{data=dto.WebhookResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/webhooks/endpoints [post]
func (h *WebhookHandler) RegisterEndpoint(c *fiber.Ctx) error {
	var req dto.CreateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := validator.ValidateStruct(&req); err != nil {
		messages := validator.FormatValidationErrors(err)
		return response.ErrorResponse(c, fiber.StatusBadRequest, messages[0])
	}

	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	result, err := h.webhookService.RegisterEndpoint(c.UserContext(), userID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// ListEndpoints godoc
// @Summary      List webhook endpoints
// @Description  Returns a paginated list of webhook endpoints belonging to the authenticated user.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.WebhookListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/webhooks/endpoints [get]
func (h *WebhookHandler) ListEndpoints(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	result, err := h.webhookService.ListEndpoints(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// DeleteEndpoint godoc
// @Summary      Delete a webhook endpoint
// @Description  Soft-deletes a webhook endpoint owned by the authenticated user.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Webhook Endpoint ID (UUID)"
// @Success      200  {object}  response.APIResponse{data=map[string]string}
// @Failure      400  {object}  response.APIResponse{error=response.APIError}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      403  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/webhooks/endpoints/{id} [delete]
func (h *WebhookHandler) DeleteEndpoint(c *fiber.Ctx) error {
	endpointIDStr := c.Params("id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid endpoint id format")
	}

	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	if err := h.webhookService.DeleteEndpoint(c.UserContext(), userID, endpointID); err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, map[string]string{
		"message": "webhook endpoint deleted successfully",
	})
}

// ListDeliveries godoc
// @Summary      List webhook deliveries
// @Description  Returns a paginated list of webhook deliveries for the authenticated user's endpoints.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.WebhookDeliveryListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/webhooks/deliveries [get]
func (h *WebhookHandler) ListDeliveries(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	result, err := h.webhookService.GetDeliveries(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}
