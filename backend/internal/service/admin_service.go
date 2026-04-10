package service

import (
	"context"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
)

// TransactionStatsRepository provides transaction count methods for admin stats.
type TransactionStatsRepository interface {
	CountByStatus(ctx context.Context, status model.TransactionStatus) (int64, error)
	CountAll(ctx context.Context) (int64, error)
}

// UserStatsRepository provides user count methods for admin stats.
type UserStatsRepository interface {
	CountActive(ctx context.Context) (int64, error)
}

// WebhookDeliveryStatsRepository provides webhook delivery count methods for admin stats.
type WebhookDeliveryStatsRepository interface {
	CountByStatus(ctx context.Context, status model.DeliveryStatus) (int64, error)
	CountAll(ctx context.Context) (int64, error)
}

// AdminService defines the interface for admin dashboard operations.
type AdminService interface {
	// GetDashboardStats returns aggregated platform statistics.
	GetDashboardStats(ctx context.Context) (*dto.DashboardStatsResponse, error)
}

// adminService implements AdminService.
type adminService struct {
	txStatsRepo      TransactionStatsRepository
	userStatsRepo    UserStatsRepository
	webhookStatsRepo WebhookDeliveryStatsRepository
}

// NewAdminService creates a new AdminService with all required dependencies.
func NewAdminService(
	txStatsRepo TransactionStatsRepository,
	userStatsRepo UserStatsRepository,
	webhookStatsRepo WebhookDeliveryStatsRepository,
) AdminService {
	return &adminService{
		txStatsRepo:      txStatsRepo,
		userStatsRepo:    userStatsRepo,
		webhookStatsRepo: webhookStatsRepo,
	}
}

// GetDashboardStats aggregates data from multiple repositories to build dashboard statistics.
func (s *adminService) GetDashboardStats(ctx context.Context) (*dto.DashboardStatsResponse, error) {
	// Transaction stats.
	totalTx, err := s.txStatsRepo.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	pendingTx, err := s.txStatsRepo.CountByStatus(ctx, model.TxStatusPending)
	if err != nil {
		return nil, err
	}
	successTx, err := s.txStatsRepo.CountByStatus(ctx, model.TxStatusSuccess)
	if err != nil {
		return nil, err
	}
	failedTx, err := s.txStatsRepo.CountByStatus(ctx, model.TxStatusFailed)
	if err != nil {
		return nil, err
	}

	// Active users.
	activeUsers, err := s.userStatsRepo.CountActive(ctx)
	if err != nil {
		return nil, err
	}

	// Webhook delivery stats.
	totalDeliveries, err := s.webhookStatsRepo.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	deliveredCount, err := s.webhookStatsRepo.CountByStatus(ctx, model.DeliveryDelivered)
	if err != nil {
		return nil, err
	}
	failedDeliveries, err := s.webhookStatsRepo.CountByStatus(ctx, model.DeliveryFailed)
	if err != nil {
		return nil, err
	}
	pendingDeliveries, err := s.webhookStatsRepo.CountByStatus(ctx, model.DeliveryPending)
	if err != nil {
		return nil, err
	}

	var successRate float64
	if totalDeliveries > 0 {
		successRate = float64(deliveredCount) / float64(totalDeliveries) * 100
	}

	return &dto.DashboardStatsResponse{
		Transactions: dto.TransactionStats{
			Total:   totalTx,
			Pending: pendingTx,
			Success: successTx,
			Failed:  failedTx,
		},
		ActiveUsers: activeUsers,
		WebhookStats: dto.WebhookStats{
			TotalDeliveries: totalDeliveries,
			Delivered:       deliveredCount,
			Failed:          failedDeliveries,
			Pending:         pendingDeliveries,
			SuccessRate:     successRate,
		},
	}, nil
}
