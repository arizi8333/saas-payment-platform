package service

import (
	"context"
	"testing"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock repositories for admin stats ---

type mockTxStatsRepo struct {
	counts   map[model.TransactionStatus]int64
	totalAll int64
	countErr error
}

func (m *mockTxStatsRepo) CountByStatus(_ context.Context, status model.TransactionStatus) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.counts[status], nil
}

func (m *mockTxStatsRepo) CountAll(_ context.Context) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.totalAll, nil
}

type mockUserStatsRepo struct {
	activeCount int64
	countErr    error
}

func (m *mockUserStatsRepo) CountActive(_ context.Context) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.activeCount, nil
}

type mockWebhookStatsRepo struct {
	counts   map[model.DeliveryStatus]int64
	totalAll int64
	countErr error
}

func (m *mockWebhookStatsRepo) CountByStatus(_ context.Context, status model.DeliveryStatus) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.counts[status], nil
}

func (m *mockWebhookStatsRepo) CountAll(_ context.Context) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.totalAll, nil
}

// --- Helper ---

func newTestAdminService(
	txRepo *mockTxStatsRepo,
	userRepo *mockUserStatsRepo,
	webhookRepo *mockWebhookStatsRepo,
) AdminService {
	return NewAdminService(txRepo, userRepo, webhookRepo)
}

// --- Tests ---

func TestGetDashboardStats_Success(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		totalAll: 150,
		counts: map[model.TransactionStatus]int64{
			model.TxStatusPending: 30,
			model.TxStatusSuccess: 100,
			model.TxStatusFailed:  20,
		},
	}
	userRepo := &mockUserStatsRepo{activeCount: 42}
	webhookRepo := &mockWebhookStatsRepo{
		totalAll: 200,
		counts: map[model.DeliveryStatus]int64{
			model.DeliveryDelivered: 180,
			model.DeliveryFailed:    10,
			model.DeliveryPending:   10,
		},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	resp, err := svc.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Transactions.Total != 150 {
		t.Errorf("expected total tx 150, got %d", resp.Transactions.Total)
	}
	if resp.Transactions.Pending != 30 {
		t.Errorf("expected pending tx 30, got %d", resp.Transactions.Pending)
	}
	if resp.Transactions.Success != 100 {
		t.Errorf("expected success tx 100, got %d", resp.Transactions.Success)
	}
	if resp.Transactions.Failed != 20 {
		t.Errorf("expected failed tx 20, got %d", resp.Transactions.Failed)
	}
	if resp.ActiveUsers != 42 {
		t.Errorf("expected active users 42, got %d", resp.ActiveUsers)
	}
	if resp.WebhookStats.TotalDeliveries != 200 {
		t.Errorf("expected total deliveries 200, got %d", resp.WebhookStats.TotalDeliveries)
	}
	if resp.WebhookStats.Delivered != 180 {
		t.Errorf("expected delivered 180, got %d", resp.WebhookStats.Delivered)
	}
	// Success rate: 180/200 * 100 = 90.0
	if resp.WebhookStats.SuccessRate != 90.0 {
		t.Errorf("expected success rate 90.0, got %f", resp.WebhookStats.SuccessRate)
	}
}

func TestGetDashboardStats_ZeroData(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		totalAll: 0,
		counts:   map[model.TransactionStatus]int64{},
	}
	userRepo := &mockUserStatsRepo{activeCount: 0}
	webhookRepo := &mockWebhookStatsRepo{
		totalAll: 0,
		counts:   map[model.DeliveryStatus]int64{},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	resp, err := svc.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Transactions.Total != 0 {
		t.Errorf("expected total tx 0, got %d", resp.Transactions.Total)
	}
	if resp.ActiveUsers != 0 {
		t.Errorf("expected active users 0, got %d", resp.ActiveUsers)
	}
	// Success rate should be 0 when no deliveries exist (avoid division by zero).
	if resp.WebhookStats.SuccessRate != 0 {
		t.Errorf("expected success rate 0, got %f", resp.WebhookStats.SuccessRate)
	}
}

func TestGetDashboardStats_TransactionRepoError(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		countErr: apierror.NewInternalError("db error"),
		counts:   map[model.TransactionStatus]int64{},
	}
	userRepo := &mockUserStatsRepo{activeCount: 5}
	webhookRepo := &mockWebhookStatsRepo{
		totalAll: 10,
		counts:   map[model.DeliveryStatus]int64{},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	_, err := svc.GetDashboardStats(context.Background())
	if err == nil {
		t.Fatal("expected error from transaction repo, got nil")
	}
}

func TestGetDashboardStats_UserRepoError(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		totalAll: 10,
		counts: map[model.TransactionStatus]int64{
			model.TxStatusPending: 5,
			model.TxStatusSuccess: 3,
			model.TxStatusFailed:  2,
		},
	}
	userRepo := &mockUserStatsRepo{countErr: apierror.NewInternalError("db error")}
	webhookRepo := &mockWebhookStatsRepo{
		totalAll: 10,
		counts:   map[model.DeliveryStatus]int64{},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	_, err := svc.GetDashboardStats(context.Background())
	if err == nil {
		t.Fatal("expected error from user repo, got nil")
	}
}

func TestGetDashboardStats_WebhookRepoError(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		totalAll: 10,
		counts: map[model.TransactionStatus]int64{
			model.TxStatusPending: 5,
			model.TxStatusSuccess: 3,
			model.TxStatusFailed:  2,
		},
	}
	userRepo := &mockUserStatsRepo{activeCount: 5}
	webhookRepo := &mockWebhookStatsRepo{
		countErr: apierror.NewInternalError("db error"),
		counts:   map[model.DeliveryStatus]int64{},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	_, err := svc.GetDashboardStats(context.Background())
	if err == nil {
		t.Fatal("expected error from webhook repo, got nil")
	}
}

func TestGetDashboardStats_AllDeliveriesFailed(t *testing.T) {
	txRepo := &mockTxStatsRepo{
		totalAll: 5,
		counts: map[model.TransactionStatus]int64{
			model.TxStatusPending: 0,
			model.TxStatusSuccess: 5,
			model.TxStatusFailed:  0,
		},
	}
	userRepo := &mockUserStatsRepo{activeCount: 1}
	webhookRepo := &mockWebhookStatsRepo{
		totalAll: 50,
		counts: map[model.DeliveryStatus]int64{
			model.DeliveryDelivered: 0,
			model.DeliveryFailed:    50,
			model.DeliveryPending:   0,
		},
	}

	svc := newTestAdminService(txRepo, userRepo, webhookRepo)
	resp, err := svc.GetDashboardStats(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.WebhookStats.SuccessRate != 0 {
		t.Errorf("expected success rate 0 when all failed, got %f", resp.WebhookStats.SuccessRate)
	}
	if resp.WebhookStats.Failed != 50 {
		t.Errorf("expected 50 failed deliveries, got %d", resp.WebhookStats.Failed)
	}
}
