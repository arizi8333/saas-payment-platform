package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/service"
)

// InvoiceHandler handles HTTP requests for invoice management.
type InvoiceHandler struct {
	invoiceService service.InvoiceService
}

// NewInvoiceHandler creates a new InvoiceHandler with the given InvoiceService.
func NewInvoiceHandler(invoiceService service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService}
}

// RegisterRoutes registers all invoice-related routes on the given router.
func (h *InvoiceHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	invoices := router.Group("/invoices", authMW.JWTProtected())
	invoices.Get("/", h.ListInvoices)
	invoices.Get("/:id", h.GetInvoice)
}

// ListInvoices godoc
// @Summary      List invoices
// @Description  Returns a paginated list of invoices belonging to the authenticated user.
// @Tags         Invoices
// @Accept       json
// @Produce      json
// @Param        page       query     int  false  "Page number"      default(1)
// @Param        page_size  query     int  false  "Items per page"   default(10)
// @Success      200        {object}  response.APIResponse{data=dto.InvoiceListResponse}
// @Failure      401        {object}  response.APIResponse{error=response.APIError}
// @Failure      500        {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/invoices [get]
func (h *InvoiceHandler) ListInvoices(c *fiber.Ctx) error {
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

	result, err := h.invoiceService.ListInvoices(c.UserContext(), userID, page, pageSize)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// GetInvoice godoc
// @Summary      Get invoice detail
// @Description  Returns the detail of a specific invoice by ID.
// @Tags         Invoices
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Invoice ID (UUID)"
// @Success      200  {object}  response.APIResponse{data=dto.InvoiceResponse}
// @Failure      400  {object}  response.APIResponse{error=response.APIError}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/invoices/{id} [get]
func (h *InvoiceHandler) GetInvoice(c *fiber.Ctx) error {
	invoiceIDStr := c.Params("id")
	invoiceID, err := uuid.Parse(invoiceIDStr)
	if err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid invoice id format")
	}

	result, err := h.invoiceService.GetInvoice(c.UserContext(), invoiceID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}
