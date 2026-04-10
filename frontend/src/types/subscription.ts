import { PaginationResponse } from './common';

export interface CreateProductRequest {
  name: string;
  description?: string;
}

export interface ProductResponse {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  created_at: string;
}

export interface CreatePlanRequest {
  name: string;
  amount: number;
  currency: string;
  billing_interval: 'monthly' | 'yearly';
}

export interface PlanResponse {
  id: string;
  product_id: string;
  name: string;
  amount: number;
  currency: string;
  billing_interval: string;
  is_active: boolean;
  created_at: string;
}

export interface CreateSubscriptionRequest {
  plan_id: string;
}

export interface SubscriptionResponse {
  id: string;
  plan_id: string;
  status: string;
  current_period_start: string;
  current_period_end: string;
  cancelled_at: string | null;
  created_at: string;
}

export interface ProductListResponse {
  products: ProductResponse[];
  pagination: PaginationResponse;
}

export interface PlanListResponse {
  plans: PlanResponse[];
  pagination: PaginationResponse;
}

export interface SubscriptionListResponse {
  subscriptions: SubscriptionResponse[];
  pagination: PaginationResponse;
}
