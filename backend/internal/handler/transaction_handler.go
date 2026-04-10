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

// TransactionHandler handles HTTP requests for payment transactions.
type TransactionHandler struct {
	transactionService service.TransactionService
}

// NewTransactionHandler creates a new TransactionHandler with the given TransactionService.
func NewTransactionHandler(transactionService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{transactionService: transactionService}
}

// RegisterRoutes registers all transaction-related routes on the given router.
func (h *TransactionHandler) RegisterRoutes(router fiber.Router, apiKeyMW *middleware.APIKeyMiddleware) {
	txns := router.Group("/transactions", apiKeyMW.ValidateAPIKey())
	txns.Post("/", h.CreateTransaction)
	txns.Get("/", h.ListTransactions)
	txns.Get("/:id", h.GetTransaction)
}

// CreateTransaction godoc
// @Summary      Create a new transaction
// @Description  Creates a new payment transaction with status "pending". After creation, payment simulation is triggered asynchronously.
// @Tags         Transactions
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateTransactionRequest  true  "Transaction data"
// @Success      201   {object}  response.APIResponse{data=dto.TransactionResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      409   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Security     ApiKeyAuth
// @Router       /api/v1/transactions [post]
func (h *TransactionHandler) CreateTransaction(c *fiber.Ctx) error {
	var req dto.CreateTransactionRequest
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

	result, err := h.transactionService.CreateTransaction(c.UserContext(), userID, &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	// Trigger payment simulation asynchronously.
	txID, parseErr := uuid.Parse(result.ID)
	if parseErr == nil {
		go h.transactionService.SimulatePayment(c.UserContext(), txID)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// ListTransactions godoc
// @Summary      List transactions
// @Description  Returns a paginated list of transactions belonging to the authenticated API key owner.
// @Tags         Transactions
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.TransactionListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     ApiKeyAuth
// @Router       /api/v1/transactions [get]
func (h *TransactionHandler) ListTransactions(c *fiber.Ctx) error {
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

	result, err := h.transactionService.ListTransactions(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// GetTransaction godoc
// @Summary      Get transaction detail
// @Description  Returns the detail of a specific transaction by ID.
// @Tags         Transactions
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Transaction ID (UUID)"
// @Success      200  {object}  response.APIResponse{data=dto.TransactionResponse}
// @Failure      400  {object}  response.APIResponse{error=response.APIError}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     ApiKeyAuth
// @Router       /api/v1/transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(c *fiber.Ctx) error {
	txIDStr := c.Params("id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid transaction id format")
	}

	result, err := h.transactionService.GetTransaction(c.UserContext(), txID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}
