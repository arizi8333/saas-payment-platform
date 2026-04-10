import { PaginationResponse } from './common';

export interface CreateWebhookRequest {
  url: string;
  events: string[];
}

export interface WebhookResponse {
  id: string;
  url: string;
  secret?: string;
  events: string[];
  is_active: boolean;
  created_at: string;
}

export interface WebhookDeliveryResponse {
  id: string;
  event_type: string;
  status: string;
  response_code: number | null;
  retry_count: number;
  delivered_at: string | null;
  created_at: string;
}

export interface WebhookListResponse {
  endpoints: WebhookResponse[];
  pagination: PaginationResponse;
}

export interface WebhookDeliveryListResponse {
  deliveries: WebhookDeliveryResponse[];
  pagination: PaginationResponse;
}
