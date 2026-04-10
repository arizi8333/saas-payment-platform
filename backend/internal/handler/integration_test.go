package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
)

// --- Shared integration test helpers ---

// integrationSecret is a test-only JWT signing key (not a real credential).
const integrationSecret = "test-only-integration-secret"

func intToJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func intParseResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

func intGenerateToken(t *testing.T, userID uuid.UUID, role string) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, role, integrationSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

// =============================================================================
// Integration Test 1: Full Auth Flow
// Register → Login → Access Protected Endpoint (Profile)
// Validates: Requirements 1.1, 1.4
// =============================================================================

func TestIntegration_AuthFlow_RegisterLoginProfile(t *testing.T) {
	userID := uuid.New()
	var registeredEmail string

	// Mock user service that tracks state across calls.
	userSvc := &mockUserService{
		registerFn: func(_ context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error) {
			registeredEmail = req.Email
			return &dto.UserProfileResponse{
				ID:        userID.String(),
				Email:     req.Email,
				FullName:  req.FullName,
				Role:      "developer",
				IsActive:  true,
				CreatedAt: time.Now(),
			}, nil
		},
		loginFn: func(_ context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
			// Generate a real JWT so the auth middleware can validate it.
			token, err := auth.GenerateToken(userID, "developer", integrationSecret, 24)
			if err != nil {
				return nil, err
			}
			return &dto.LoginResponse{Token: token, ExpiresIn: 86400}, nil
		},
		getProfileFn: func(_ context.Context, id uuid.UUID) (*dto.UserProfileResponse, error) {
			return &dto.UserProfileResponse{
				ID:        id.String(),
				Email:     registeredEmail,
				FullName:  "Integration User",
				Role:      "developer",
				IsActive:  true,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	// Setup Fiber app with user handler and auth middleware.
	app := fiber.New()
	userHandler := NewUserHandler(userSvc)
	authMW := middleware.NewAuthMiddleware(integrationSecret)
	api := app.Group("/api/v1")
	userHandler.RegisterRoutes(api, authMW)

	// Step 1: Register a new user.
	regBody := intToJSON(t, dto.RegisterRequest{
		Email:    "integ@example.com",
		Password: "testpass1234",
		FullName: "Integration User",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", regBody)
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := app.Test(regReq)
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", regResp.StatusCode)
	}
	regResult := intParseResponse(t, regResp)
	if !regResult.Success {
		t.Fatal("register: expected success=true")
	}

	// Step 2: Login with the registered credentials.
	loginBody := intToJSON(t, dto.LoginRequest{
		Email:    "integ@example.com",
		Password: "testpass1234",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", loginResp.StatusCode)
	}
	loginResult := intParseResponse(t, loginResp)
	if !loginResult.Success {
		t.Fatal("login: expected success=true")
	}

	// Extract token from login response.
	dataMap, ok := loginResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("login: expected data to be a map")
	}
	token, ok := dataMap["token"].(string)
	if !ok || token == "" {
		t.Fatal("login: expected non-empty token in response")
	}

	// Step 3: Access protected profile endpoint with the token.
	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	profileReq.Header.Set("Authorization", "Bearer "+token)
	profileResp, err := app.Test(profileReq)
	if err != nil {
		t.Fatalf("profile request failed: %v", err)
	}
	if profileResp.StatusCode != http.StatusOK {
		t.Fatalf("profile: expected 200, got %d", profileResp.StatusCode)
	}
	profileResult := intParseResponse(t, profileResp)
	if !profileResult.Success {
		t.Fatal("profile: expected success=true")
	}

	// Verify profile data matches registration.
	profileData, ok := profileResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("profile: expected data to be a map")
	}
	if profileData["email"] != "integ@example.com" {
		t.Errorf("profile: expected email=integ@example.com, got %v", profileData["email"])
	}
	if profileData["role"] != "developer" {
		t.Errorf("profile: expected role=developer, got %v", profileData["role"])
	}
}

// TestIntegration_AuthFlow_ProfileWithoutToken verifies that accessing a
// protected endpoint without a token returns 401.
func TestIntegration_AuthFlow_ProfileWithoutToken(t *testing.T) {
	userSvc := &mockUserService{}
	app := fiber.New()
	userHandler := NewUserHandler(userSvc)
	authMW := middleware.NewAuthMiddleware(integrationSecret)
	api := app.Group("/api/v1")
	userHandler.RegisterRoutes(api, authMW)

	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	profileResp, err := app.Test(profileReq)
	if err != nil {
		t.Fatalf("profile request failed: %v", err)
	}
	if profileResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("profile without token: expected 401, got %d", profileResp.StatusCode)
	}
}

// =============================================================================
// Integration Test 2: Transaction Flow
// Create Transaction → Verify Response → List Transactions
// Validates: Requirements 3.1, 3.3, 4.1, 4.4, 5.2, 5.3
// =============================================================================

func TestIntegration_TransactionFlow_CreateAndList(t *testing.T) {
	userID := uuid.New()
	txID := uuid.New()
	var createdTransactions []dto.TransactionResponse

	// Mock transaction service that accumulates created transactions.
	txSvc := &mockTransactionService{
		createTransactionFn: func(_ context.Context, uid uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
			if uid != userID {
				t.Errorf("expected userID %s, got %s", userID, uid)
			}
			txResp := &dto.TransactionResponse{
				ID:            txID.String(),
				ExternalID:    req.ExternalID,
				Amount:        req.Amount,
				Currency:      req.Currency,
				Status:        "pending",
				PaymentMethod: req.PaymentMethod,
				Description:   req.Description,
				CustomerEmail: req.CustomerEmail,
				CreatedAt:     time.Now(),
			}
			createdTransactions = append(createdTransactions, *txResp)
			return txResp, nil
		},
		simulatePaymentFn: func(_ context.Context, _ uuid.UUID) error {
			return nil // no-op for integration test
		},
		listTransactionsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error) {
			return &dto.TransactionListResponse{
				Transactions: createdTransactions,
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: len(createdTransactions),
					TotalPages: 1,
				},
			}, nil
		},
		getTransactionFn: func(_ context.Context, id uuid.UUID) (*dto.TransactionResponse, error) {
			for _, tx := range createdTransactions {
				if tx.ID == id.String() {
					return &tx, nil
				}
			}
			return nil, nil
		},
	}

	// Mock API key service that validates any key and returns our test user.
	apiKeySvc := &mockAPIKeyServiceForTx{userID: userID}

	// Setup Fiber app with transaction handler and API key middleware.
	app := fiber.New()
	txHandler := NewTransactionHandler(txSvc)
	apiKeyMW := middleware.NewAPIKeyMiddleware(apiKeySvc)
	api := app.Group("/api/v1")
	txHandler.RegisterRoutes(api, apiKeyMW)

	// Step 1: Create a transaction via API key-authenticated endpoint.
	createBody := intToJSON(t, dto.CreateTransactionRequest{
		Amount:        100000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-integration-001",
		Description:   "Integration test payment",
		CustomerEmail: "customer@test.com",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-API-Key", "test_key_testintegration")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create transaction request failed: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create transaction: expected 201, got %d", createResp.StatusCode)
	}
	createResult := intParseResponse(t, createResp)
	if !createResult.Success {
		t.Fatal("create transaction: expected success=true")
	}

	// Verify transaction data in response.
	txData, ok := createResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("create transaction: expected data to be a map")
	}
	if txData["status"] != "pending" {
		t.Errorf("create transaction: expected status=pending, got %v", txData["status"])
	}
	if txData["external_id"] != "ext-integration-001" {
		t.Errorf("create transaction: expected external_id=ext-integration-001, got %v", txData["external_id"])
	}

	// Step 2: List transactions and verify the created one appears.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/?page=1&page_size=10", nil)
	listReq.Header.Set("X-API-Key", "test_key_testintegration")
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list transactions request failed: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list transactions: expected 200, got %d", listResp.StatusCode)
	}
	listResult := intParseResponse(t, listResp)
	if !listResult.Success {
		t.Fatal("list transactions: expected success=true")
	}

	// Verify pagination data.
	listData, ok := listResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("list transactions: expected data to be a map")
	}
	pagination, ok := listData["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("list transactions: expected pagination in data")
	}
	totalItems, ok := pagination["total_items"].(float64)
	if !ok || int(totalItems) != 1 {
		t.Errorf("list transactions: expected total_items=1, got %v", pagination["total_items"])
	}

	// Step 3: Get the specific transaction by ID.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID.String(), nil)
	getReq.Header.Set("X-API-Key", "test_key_testintegration")
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("get transaction request failed: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get transaction: expected 200, got %d", getResp.StatusCode)
	}
	getResult := intParseResponse(t, getResp)
	if !getResult.Success {
		t.Fatal("get transaction: expected success=true")
	}
}

// TestIntegration_TransactionFlow_NoAPIKey verifies that transaction endpoints
// reject requests without an API key.
func TestIntegration_TransactionFlow_NoAPIKey(t *testing.T) {
	txSvc := &mockTransactionService{}
	apiKeySvc := &mockAPIKeyServiceForTx{userID: uuid.New()}

	app := fiber.New()
	txHandler := NewTransactionHandler(txSvc)
	apiKeyMW := middleware.NewAPIKeyMiddleware(apiKeySvc)
	api := app.Group("/api/v1")
	txHandler.RegisterRoutes(api, apiKeyMW)

	createBody := intToJSON(t, dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "credit_card",
		ExternalID:    "ext-nokey",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	// No X-API-Key header.
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if createResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", createResp.StatusCode)
	}
}

// =============================================================================
// Integration Test 3: Subscription Flow
// Create Product → Create Plan → Create Subscription
// Validates: Requirements 6.3, 6.4, 7.1
// =============================================================================

func TestIntegration_SubscriptionFlow_ProductPlanSubscribe(t *testing.T) {
	userID := uuid.New()
	productID := uuid.New()
	planID := uuid.New()
	subID := uuid.New()
	now := time.Now()

	// Track state across chained requests.
	var createdProductID string
	var createdPlanID string

	subSvc := &mockSubscriptionService{
		createProductFn: func(_ context.Context, uid uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
			if uid != userID {
				t.Errorf("createProduct: expected userID %s, got %s", userID, uid)
			}
			createdProductID = productID.String()
			return &dto.ProductResponse{
				ID:          productID.String(),
				Name:        req.Name,
				Description: req.Description,
				IsActive:    true,
				CreatedAt:   now,
			}, nil
		},
		listProductsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error) {
			products := []dto.ProductResponse{}
			if createdProductID != "" {
				products = append(products, dto.ProductResponse{
					ID:        createdProductID,
					Name:      "SaaS Pro",
					IsActive:  true,
					CreatedAt: now,
				})
			}
			return &dto.ProductListResponse{
				Products: products,
				Pagination: dto.PaginationResponse{
					Page: page, PageSize: pageSize,
					TotalItems: len(products), TotalPages: 1,
				},
			}, nil
		},
		createPlanFn: func(_ context.Context, uid uuid.UUID, pid uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error) {
			if uid != userID {
				t.Errorf("createPlan: expected userID %s, got %s", userID, uid)
			}
			if pid.String() != createdProductID {
				t.Errorf("createPlan: expected productID %s, got %s", createdProductID, pid)
			}
			createdPlanID = planID.String()
			return &dto.PlanResponse{
				ID:              planID.String(),
				ProductID:       pid.String(),
				Name:            req.Name,
				Amount:          req.Amount,
				Currency:        req.Currency,
				BillingInterval: req.BillingInterval,
				IsActive:        true,
				CreatedAt:       now,
			}, nil
		},
		createSubscriptionFn: func(_ context.Context, uid uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error) {
			if uid != userID {
				t.Errorf("createSubscription: expected userID %s, got %s", userID, uid)
			}
			if req.PlanID != createdPlanID {
				t.Errorf("createSubscription: expected planID %s, got %s", createdPlanID, req.PlanID)
			}
			return &dto.SubscriptionResponse{
				ID:                 subID.String(),
				PlanID:             req.PlanID,
				Status:             "pending_payment",
				CurrentPeriodStart: now,
				CurrentPeriodEnd:   now.AddDate(0, 1, 0),
				CreatedAt:          now,
			}, nil
		},
		listSubsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error) {
			return &dto.SubscriptionListResponse{
				Subscriptions: []dto.SubscriptionResponse{
					{
						ID:                 subID.String(),
						PlanID:             planID.String(),
						Status:             "pending_payment",
						CurrentPeriodStart: now,
						CurrentPeriodEnd:   now.AddDate(0, 1, 0),
						CreatedAt:          now,
					},
				},
				Pagination: dto.PaginationResponse{
					Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1,
				},
			}, nil
		},
		cancelSubFn: func(_ context.Context, id uuid.UUID) (*dto.SubscriptionResponse, error) {
			return nil, nil
		},
	}

	// Setup Fiber app with subscription handler and auth middleware.
	app := fiber.New()
	subHandler := NewSubscriptionHandler(subSvc)
	authMW := middleware.NewAuthMiddleware(integrationSecret)
	api := app.Group("/api/v1")
	subHandler.RegisterRoutes(api, authMW)

	token := intGenerateToken(t, userID, "developer")

	// Step 1: Create a product.
	prodBody := intToJSON(t, dto.CreateProductRequest{
		Name:        "SaaS Pro",
		Description: "Professional SaaS subscription",
	})
	prodReq := httptest.NewRequest(http.MethodPost, "/api/v1/products/", prodBody)
	prodReq.Header.Set("Content-Type", "application/json")
	prodReq.Header.Set("Authorization", "Bearer "+token)
	prodResp, err := app.Test(prodReq)
	if err != nil {
		t.Fatalf("create product request failed: %v", err)
	}
	if prodResp.StatusCode != http.StatusCreated {
		t.Fatalf("create product: expected 201, got %d", prodResp.StatusCode)
	}
	prodResult := intParseResponse(t, prodResp)
	if !prodResult.Success {
		t.Fatal("create product: expected success=true")
	}
	prodData, ok := prodResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("create product: expected data to be a map")
	}
	if prodData["name"] != "SaaS Pro" {
		t.Errorf("create product: expected name=SaaS Pro, got %v", prodData["name"])
	}
	if prodData["is_active"] != true {
		t.Errorf("create product: expected is_active=true, got %v", prodData["is_active"])
	}

	// Step 2: Create a plan for the product.
	planBody := intToJSON(t, dto.CreatePlanRequest{
		Name:            "Monthly Pro",
		Amount:          99000,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	planReq := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+productID.String()+"/plans", planBody)
	planReq.Header.Set("Content-Type", "application/json")
	planReq.Header.Set("Authorization", "Bearer "+token)
	planResp, err := app.Test(planReq)
	if err != nil {
		t.Fatalf("create plan request failed: %v", err)
	}
	if planResp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan: expected 201, got %d", planResp.StatusCode)
	}
	planResult := intParseResponse(t, planResp)
	if !planResult.Success {
		t.Fatal("create plan: expected success=true")
	}
	planData, ok := planResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("create plan: expected data to be a map")
	}
	if planData["billing_interval"] != "monthly" {
		t.Errorf("create plan: expected billing_interval=monthly, got %v", planData["billing_interval"])
	}
	if planData["product_id"] != productID.String() {
		t.Errorf("create plan: expected product_id=%s, got %v", productID, planData["product_id"])
	}

	// Step 3: Create a subscription for the plan.
	subBody := intToJSON(t, dto.CreateSubscriptionRequest{
		PlanID: planID.String(),
	})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", subBody)
	subReq.Header.Set("Content-Type", "application/json")
	subReq.Header.Set("Authorization", "Bearer "+token)
	subResp, err := app.Test(subReq)
	if err != nil {
		t.Fatalf("create subscription request failed: %v", err)
	}
	if subResp.StatusCode != http.StatusCreated {
		t.Fatalf("create subscription: expected 201, got %d", subResp.StatusCode)
	}
	subResult := intParseResponse(t, subResp)
	if !subResult.Success {
		t.Fatal("create subscription: expected success=true")
	}
	subData, ok := subResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("create subscription: expected data to be a map")
	}
	if subData["status"] != "pending_payment" {
		t.Errorf("create subscription: expected status=pending_payment, got %v", subData["status"])
	}
	if subData["plan_id"] != planID.String() {
		t.Errorf("create subscription: expected plan_id=%s, got %v", planID, subData["plan_id"])
	}

	// Step 4: List subscriptions and verify the created one appears.
	listSubReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/", nil)
	listSubReq.Header.Set("Authorization", "Bearer "+token)
	listSubResp, err := app.Test(listSubReq)
	if err != nil {
		t.Fatalf("list subscriptions request failed: %v", err)
	}
	if listSubResp.StatusCode != http.StatusOK {
		t.Fatalf("list subscriptions: expected 200, got %d", listSubResp.StatusCode)
	}
	listSubResult := intParseResponse(t, listSubResp)
	if !listSubResult.Success {
		t.Fatal("list subscriptions: expected success=true")
	}
	listSubData, ok := listSubResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("list subscriptions: expected data to be a map")
	}
	subPagination, ok := listSubData["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("list subscriptions: expected pagination in data")
	}
	subTotal, ok := subPagination["total_items"].(float64)
	if !ok || int(subTotal) != 1 {
		t.Errorf("list subscriptions: expected total_items=1, got %v", subPagination["total_items"])
	}
}

// TestIntegration_SubscriptionFlow_NoToken verifies that subscription endpoints
// reject requests without a JWT token.
func TestIntegration_SubscriptionFlow_NoToken(t *testing.T) {
	subSvc := &mockSubscriptionService{}
	app := fiber.New()
	subHandler := NewSubscriptionHandler(subSvc)
	authMW := middleware.NewAuthMiddleware(integrationSecret)
	api := app.Group("/api/v1")
	subHandler.RegisterRoutes(api, authMW)

	// Try to create a product without auth.
	prodBody := intToJSON(t, dto.CreateProductRequest{Name: "Test"})
	prodReq := httptest.NewRequest(http.MethodPost, "/api/v1/products/", prodBody)
	prodReq.Header.Set("Content-Type", "application/json")
	prodResp, err := app.Test(prodReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if prodResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", prodResp.StatusCode)
	}

	// Try to create a subscription without auth.
	subBody := intToJSON(t, dto.CreateSubscriptionRequest{PlanID: uuid.New().String()})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", subBody)
	subReq.Header.Set("Content-Type", "application/json")
	subResp, err := app.Test(subReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if subResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", subResp.StatusCode)
	}
}
