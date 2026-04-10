import { api } from './api';
import { SuccessResponse, ProductResponse, ProductListResponse, PlanResponse, PlanListResponse, SubscriptionResponse, SubscriptionListResponse, CreateProductRequest, CreatePlanRequest, CreateSubscriptionRequest } from '@/types';

export const subscriptionsService = {
  createProduct: (data: CreateProductRequest) =>
    api.post<SuccessResponse<ProductResponse>>('/products', data),

  listProducts: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<ProductListResponse>>(`/products?page=${page}&page_size=${pageSize}`),

  createPlan: (productId: string, data: CreatePlanRequest) =>
    api.post<SuccessResponse<PlanResponse>>(`/products/${productId}/plans`, data),

  createSubscription: (data: CreateSubscriptionRequest) =>
    api.post<SuccessResponse<SubscriptionResponse>>('/subscriptions', data),

  listSubscriptions: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<SubscriptionListResponse>>(`/subscriptions?page=${page}&page_size=${pageSize}`),

  cancel: (id: string) =>
    api.patch<SuccessResponse<SubscriptionResponse>>(`/subscriptions/${id}/cancel`),
};
