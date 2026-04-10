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

// APIKeyHandler handles HTTP requests for API key management.
type APIKeyHandler struct {
	apiKeyService service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler with the given APIKeyService.
func NewAPIKeyHandler(apiKeyService service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeyService: apiKeyService}
}

// RegisterRoutes registers all API key routes on the given router.
func (h *APIKeyHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	keys := router.Group("/api-keys", authMW.JWTProtected())
	keys.Post("/", h.CreateKey)
	keys.Get("/", h.ListKeys)
	keys.Delete("/:id", h.RevokeKey)
}

// CreateKey godoc
// @Summary      Create a new API key
// @Description  Generates a new API key for the authenticated user. The full key is returned only once.
// @Tags         API Keys
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateAPIKeyRequest  true  "API key creation data"
// @Success      201   {object}  response.APIResponse{data=dto.APIKeyCreatedResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/api-keys [post]
func (h *APIKeyHandler) CreateKey(c *fiber.Ctx) error {
	var req dto.CreateAPIKeyRequest
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

	result, err := h.apiKeyService.CreateKey(c.UserContext(), userID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// ListKeys godoc
// @Summary      List API keys
// @Description  Returns a paginated list of API keys belonging to the authenticated user. Full keys are never exposed.
// @Tags         API Keys
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.APIKeyListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/api-keys [get]
func (h *APIKeyHandler) ListKeys(c *fiber.Ctx) error {
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

	result, err := h.apiKeyService.ListKeys(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// RevokeKey godoc
// @Summary      Revoke an API key
// @Description  Deactivates an API key owned by the authenticated user. Subsequent requests using this key will be rejected.
// @Tags         API Keys
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "API Key ID (UUID)"
// @Success      200  {object}  response.APIResponse{data=map[string]string}
// @Failure      400  {object}  response.APIResponse{error=response.APIError}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      403  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) RevokeKey(c *fiber.Ctx) error {
	keyIDStr := c.Params("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid key id format")
	}

	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	if err := h.apiKeyService.RevokeKey(c.UserContext(), userID, keyID); err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, map[string]string{
		"message": "api key revoked successfully",
	})
}
