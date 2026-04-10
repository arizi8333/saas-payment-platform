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

// SubscriptionHandler handles HTTP requests for product, plan, and subscription management.
type SubscriptionHandler struct {
	subscriptionService service.SubscriptionService
}

// NewSubscriptionHandler creates a new SubscriptionHandler with the given SubscriptionService.
func NewSubscriptionHandler(subscriptionService service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subscriptionService: subscriptionService}
}

// RegisterRoutes registers all subscription-related routes on the given router.
func (h *SubscriptionHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	products := router.Group("/products", authMW.JWTProtected())
	products.Post("/", h.CreateProduct)
	products.Get("/", h.ListProducts)
	products.Post("/:id/plans", h.CreatePlan)

	subscriptions := router.Group("/subscriptions", authMW.JWTProtected())
	subscriptions.Post("/", h.CreateSubscription)
	subscriptions.Get("/", h.ListSubscriptions)
	subscriptions.Patch("/:id/cancel", h.CancelSubscription)
}

// CreateProduct godoc
// @Summary      Create a new product
// @Description  Creates a new product with name and description. The product is set to active by default.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateProductRequest  true  "Product data"
// @Success      201   {object}  response.APIResponse{data=dto.ProductResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/products [post]
func (h *SubscriptionHandler) CreateProduct(c *fiber.Ctx) error {
	var req dto.CreateProductRequest
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

	result, err := h.subscriptionService.CreateProduct(c.UserContext(), userID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// ListProducts godoc
// @Summary      List products
// @Description  Returns a paginated list of products belonging to the authenticated user.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.ProductListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/products [get]
func (h *SubscriptionHandler) ListProducts(c *fiber.Ctx) error {
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

	result, err := h.subscriptionService.ListProducts(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// CreatePlan godoc
// @Summary      Create a plan for a product
// @Description  Creates a new billing plan for the specified product with amount, currency, and billing interval.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        id    path      string                true  "Product ID (UUID)"
// @Param        body  body      dto.CreatePlanRequest  true  "Plan data"
// @Success      201   {object}  response.APIResponse{data=dto.PlanResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      403   {object}  response.APIResponse{error=response.APIError}
// @Failure      404   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/products/{id}/plans [post]
func (h *SubscriptionHandler) CreatePlan(c *fiber.Ctx) error {
	productIDStr := c.Params("id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid product id format")
	}

	var req dto.CreatePlanRequest
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

	result, err := h.subscriptionService.CreatePlan(c.UserContext(), userID, productID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// CreateSubscription godoc
// @Summary      Create a new subscription
// @Description  Creates a new subscription with status "pending_payment", a related transaction, and an invoice.
// @Tags         Subscriptions
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateSubscriptionRequest  true  "Subscription data"
// @Success      201   {object}  response.APIResponse{data=dto.SubscriptionResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/subscriptions [post]
func (h *SubscriptionHandler) CreateSubscription(c *fiber.Ctx) error {
	var req dto.CreateSubscriptionRequest
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

	result, err := h.subscriptionService.CreateSubscription(c.UserContext(), userID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// ListSubscriptions godoc
// @Summary      List subscriptions
// @Description  Returns a paginated list of subscriptions belonging to the authenticated user.
// @Tags         Subscriptions
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.SubscriptionListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/subscriptions [get]
func (h *SubscriptionHandler) ListSubscriptions(c *fiber.Ctx) error {
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

	result, err := h.subscriptionService.ListSubscriptions(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// CancelSubscription godoc
// @Summary      Cancel a subscription
// @Description  Cancels an active subscription and records the cancellation timestamp.
// @Tags         Subscriptions
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Subscription ID (UUID)"
// @Success      200  {object}  response.APIResponse{data=dto.SubscriptionResponse}
// @Failure      400  {object}  response.APIResponse{error=response.APIError}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/subscriptions/{id}/cancel [patch]
func (h *SubscriptionHandler) CancelSubscription(c *fiber.Ctx) error {
	subIDStr := c.Params("id")
	subID, err := uuid.Parse(subIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid subscription id format")
	}

	result, err := h.subscriptionService.CancelSubscription(c.UserContext(), subID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}
