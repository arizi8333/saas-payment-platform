package dto

// DashboardStatsResponse contains aggregated statistics for the admin dashboard.
type DashboardStatsResponse struct {
	Transactions TransactionStats `json:"transactions"`
	ActiveUsers  int64            `json:"active_users"`
	WebhookStats WebhookStats     `json:"webhook_stats"`
}

// TransactionStats contains transaction counts grouped by status.
type TransactionStats struct {
	Total   int64 `json:"total"`
	Pending int64 `json:"pending"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
}

// WebhookStats contains webhook delivery statistics.
type WebhookStats struct {
	TotalDeliveries int64   `json:"total_deliveries"`
	Delivered       int64   `json:"delivered"`
	Failed          int64   `json:"failed"`
	Pending         int64   `json:"pending"`
	SuccessRate     float64 `json:"success_rate"`
}
