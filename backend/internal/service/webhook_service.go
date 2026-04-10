package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// randRead wraps crypto/rand.Read for testability.
var randRead = rand.Read

// WebhookService defines the interface for webhook business logic operations.
type WebhookService interface {
	// RegisterEndpoint creates a new webhook endpoint with URL, secret, and event subscriptions.
	RegisterEndpoint(ctx context.Context, userID uuid.UUID, req *dto.CreateWebhookRequest) (*dto.WebhookResponse, error)
	// ListEndpoints returns paginated webhook endpoints for a user.
	ListEndpoints(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error)
	// DeleteEndpoint soft-deletes a webhook endpoint owned by the user.
	DeleteEndpoint(ctx context.Context, userID uuid.UUID, endpointID uuid.UUID) error
	// TriggerWebhook finds all active endpoints for a user subscribed to the event,
	// creates delivery records with status "pending", and enqueues them for processing.
	TriggerWebhook(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error
	// GetDeliveries returns paginated delivery history for a user's endpoints.
	GetDeliveries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error)
	// GenerateSignature creates an HMAC-SHA256 signature for the given payload using the secret.
	GenerateSignature(payload []byte, secret string) string
}

// webhookService implements WebhookService.
type webhookService struct {
	endpointRepo repository.WebhookEndpointRepository
	deliveryRepo repository.WebhookDeliveryRepository
	txManager    database.TransactionManager
}

// NewWebhookService creates a new WebhookService with all required dependencies.
func NewWebhookService(
	endpointRepo repository.WebhookEndpointRepository,
	deliveryRepo repository.WebhookDeliveryRepository,
	txManager database.TransactionManager,
) WebhookService {
	return &webhookService{
		endpointRepo: endpointRepo,
		deliveryRepo: deliveryRepo,
		txManager:    txManager,
	}
}

// generateWebhookSecret creates a cryptographically random secret for HMAC signing.
func generateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := randRead(bytes); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(bytes), nil
}

// RegisterEndpoint creates a new webhook endpoint with a generated HMAC secret.
func (s *webhookService) RegisterEndpoint(ctx context.Context, userID uuid.UUID, req *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
	if req.URL == "" {
		return nil, apierror.NewBadRequest("webhook URL is required")
	}
	if len(req.Events) == 0 {
		return nil, apierror.NewBadRequest("at least one event type is required")
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, apierror.NewInternalError("failed to generate webhook secret")
	}

	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return nil, apierror.NewInternalError("failed to marshal events")
	}

	endpoint := &model.WebhookEndpoint{
		UserID:   userID,
		URL:      req.URL,
		Secret:   secret,
		Events:   datatypes.JSON(eventsJSON),
		IsActive: true,
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.endpointRepo.Create(txCtx, endpoint)
	})
	if err != nil {
		return nil, err
	}

	resp := toWebhookResponse(endpoint)
	resp.Secret = secret // Return secret once on creation.
	return resp, nil
}

// ListEndpoints returns paginated webhook endpoints for a user.
func (s *webhookService) ListEndpoints(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error) {
	endpoints, total, err := s.endpointRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.WebhookResponse, len(endpoints))
	for i, ep := range endpoints {
		responses[i] = *toWebhookResponse(&ep)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.WebhookListResponse{
		Endpoints: responses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// DeleteEndpoint soft-deletes a webhook endpoint after verifying ownership.
func (s *webhookService) DeleteEndpoint(ctx context.Context, userID uuid.UUID, endpointID uuid.UUID) error {
	endpoint, err := s.endpointRepo.FindByID(ctx, endpointID)
	if err != nil {
		return err
	}

	if endpoint.UserID != userID {
		return apierror.NewForbidden("you do not have permission to delete this endpoint")
	}

	return s.endpointRepo.Delete(ctx, endpointID)
}

// TriggerWebhook finds all active endpoints for a user that subscribe to the event type,
// then creates a delivery record for each with status "pending".
func (s *webhookService) TriggerWebhook(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error {
	endpoints, err := s.endpointRepo.FindActiveByUserIDAndEvent(ctx, userID, eventType)
	if err != nil {
		return err
	}

	// Filter endpoints that subscribe to this event type.
	var matched []model.WebhookEndpoint
	for _, ep := range endpoints {
		if endpointSubscribesToEvent(&ep, eventType) {
			matched = append(matched, ep)
		}
	}

	if len(matched) == 0 {
		return nil // No endpoints subscribed to this event.
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return apierror.NewInternalError("failed to marshal webhook payload")
	}

	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, ep := range matched {
			delivery := &model.WebhookDelivery{
				WebhookEndpointID: ep.ID,
				EventType:         eventType,
				Payload:           datatypes.JSON(payloadJSON),
				Status:            model.DeliveryPending,
				MaxRetries:        5,
			}
			if err := s.deliveryRepo.Create(txCtx, delivery); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetDeliveries returns paginated delivery history for a user's endpoints.
func (s *webhookService) GetDeliveries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error) {
	// First get all endpoints for this user.
	endpoints, _, err := s.endpointRepo.FindByUserID(ctx, userID, 1, 1000)
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		return &dto.WebhookDeliveryListResponse{
			Deliveries: []dto.WebhookDeliveryResponse{},
			Pagination: dto.PaginationResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: 0,
				TotalPages: 0,
			},
		}, nil
	}

	endpointIDs := make(map[uuid.UUID]bool)
	for _, ep := range endpoints {
		endpointIDs[ep.ID] = true
	}

	allDeliveries, err := s.deliveryRepo.FindPendingDeliveries(ctx, 1000)
	if err != nil {
		return nil, err
	}

	// Filter deliveries belonging to user's endpoints.
	var userDeliveries []model.WebhookDelivery
	for _, d := range allDeliveries {
		if endpointIDs[d.WebhookEndpointID] {
			userDeliveries = append(userDeliveries, d)
		}
	}

	total := len(userDeliveries)
	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	// Apply pagination.
	offset := (page - 1) * pageSize
	if offset >= total {
		return &dto.WebhookDeliveryListResponse{
			Deliveries: []dto.WebhookDeliveryResponse{},
			Pagination: dto.PaginationResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: totalPages,
			},
		}, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	paged := userDeliveries[offset:end]

	responses := make([]dto.WebhookDeliveryResponse, len(paged))
	for i, d := range paged {
		responses[i] = *toWebhookDeliveryResponse(&d)
	}

	return &dto.WebhookDeliveryListResponse{
		Deliveries: responses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}

// GenerateSignature creates an HMAC-SHA256 signature for the given payload.
func (s *webhookService) GenerateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// endpointSubscribesToEvent checks if an endpoint's events JSON array contains the given event type.
func endpointSubscribesToEvent(ep *model.WebhookEndpoint, eventType string) bool {
	var events []string
	if err := json.Unmarshal(ep.Events, &events); err != nil {
		return false
	}
	for _, e := range events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

// toWebhookResponse maps a WebhookEndpoint model to a WebhookResponse DTO.
func toWebhookResponse(ep *model.WebhookEndpoint) *dto.WebhookResponse {
	var events []string
	_ = json.Unmarshal(ep.Events, &events)

	return &dto.WebhookResponse{
		ID:        ep.ID.String(),
		URL:       ep.URL,
		Events:    events,
		IsActive:  ep.IsActive,
		CreatedAt: ep.CreatedAt,
	}
}

// toWebhookDeliveryResponse maps a WebhookDelivery model to a WebhookDeliveryResponse DTO.
func toWebhookDeliveryResponse(d *model.WebhookDelivery) *dto.WebhookDeliveryResponse {
	return &dto.WebhookDeliveryResponse{
		ID:           d.ID.String(),
		EventType:    d.EventType,
		Status:       string(d.Status),
		ResponseCode: d.ResponseCode,
		RetryCount:   d.RetryCount,
		DeliveredAt:  d.DeliveredAt,
		CreatedAt:    d.CreatedAt,
	}
}
