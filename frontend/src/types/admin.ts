export interface TransactionStats {
  total: number;
  pending: number;
  success: number;
  failed: number;
}

export interface WebhookStats {
  total_deliveries: number;
  delivered: number;
  failed: number;
  pending: number;
  success_rate: number;
}

export interface DashboardStatsResponse {
  transactions: TransactionStats;
  active_users: number;
  webhook_stats: WebhookStats;
}
