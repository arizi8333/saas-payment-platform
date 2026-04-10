import { api } from './api';
import { SuccessResponse, WebhookResponse, WebhookListResponse, WebhookDeliveryListResponse, CreateWebhookRequest } from '@/types';

export const webhooksService = {
  create: (data: CreateWebhookRequest) =>
    api.post<SuccessResponse<WebhookResponse>>('/webhooks/endpoints', data),

  list: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<WebhookListResponse>>(`/webhooks/endpoints?page=${page}&page_size=${pageSize}`),

  remove: (id: string) =>
    api.delete<SuccessResponse<null>>(`/webhooks/endpoints/${id}`),

  deliveries: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<WebhookDeliveryListResponse>>(`/webhooks/deliveries?page=${page}&page_size=${pageSize}`),
};
