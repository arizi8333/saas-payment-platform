package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/pkg/validator"
	"github.com/saas-payment-platform/backend/internal/service"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// UserHandler handles HTTP requests for user authentication and profile.
type UserHandler struct {
	userService service.UserService
}

// NewUserHandler creates a new UserHandler with the given UserService.
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// RegisterRoutes registers all user-related routes on the given router.
func (h *UserHandler) RegisterRoutes(router fiber.Router, authMW *middleware.AuthMiddleware) {
	auth := router.Group("/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)

	users := router.Group("/users", authMW.JWTProtected())
	users.Get("/profile", h.GetProfile)
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account with the default "developer" role. Password is stored as bcrypt hash.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.RegisterRequest  true  "Registration data"
// @Success      201   {object}  response.APIResponse{data=dto.UserProfileResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      409   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Router       /api/v1/auth/register [post]
func (h *UserHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := validator.ValidateStruct(&req); err != nil {
		messages := validator.FormatValidationErrors(err)
		return response.ErrorResponse(c, fiber.StatusBadRequest, messages[0])
	}

	result, err := h.userService.Register(c.UserContext(), &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusCreated, result)
}

// Login godoc
// @Summary      Login user
// @Description  Authenticates a user with email and password, returns a JWT token containing user_id and role.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.LoginRequest  true  "Login credentials"
// @Success      200   {object}  response.APIResponse{data=dto.LoginResponse}
// @Failure      400   {object}  response.APIResponse{error=response.APIError}
// @Failure      401   {object}  response.APIResponse{error=response.APIError}
// @Failure      500   {object}  response.APIResponse{error=response.APIError}
// @Router       /api/v1/auth/login [post]
func (h *UserHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.ErrorResponse(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := validator.ValidateStruct(&req); err != nil {
		messages := validator.FormatValidationErrors(err)
		return response.ErrorResponse(c, fiber.StatusBadRequest, messages[0])
	}

	result, err := h.userService.Login(c.UserContext(), &req)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// GetProfile godoc
// @Summary      Get user profile
// @Description  Returns the authenticated user's profile data. Password hash is never exposed.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.APIResponse{data=dto.UserProfileResponse}
// @Failure      401  {object}  response.APIResponse{error=response.APIError}
// @Failure      404  {object}  response.APIResponse{error=response.APIError}
// @Failure      500  {object}  response.APIResponse{error=response.APIError}
// @Security     BearerAuth
// @Router       /api/v1/users/profile [get]
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	if !ok {
		return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid user identity")
	}

	result, err := h.userService.GetProfile(c.UserContext(), userID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return response.SuccessResponse(c, fiber.StatusOK, result)
}

// handleServiceError checks if the error is an *apierror.APIError and returns
// the appropriate HTTP status code; otherwise returns 500.
func handleServiceError(c *fiber.Ctx, err error) error {
	var apiErr *apierror.APIError
	if errors.As(err, &apiErr) {
		return response.ErrorResponse(c, apiErr.StatusCode, apiErr.Message)
	}
	return response.ErrorResponse(c, fiber.StatusInternalServerError, "internal server error")
}
