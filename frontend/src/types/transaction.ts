import { PaginationResponse } from './common';

export interface CreateTransactionRequest {
  amount: number;
  currency: string;
  payment_method: string;
  external_id: string;
  description?: string;
  customer_email?: string;
  metadata?: Record<string, unknown>;
}

export interface TransactionResponse {
  id: string;
  external_id: string;
  amount: number;
  currency: string;
  status: string;
  payment_method: string;
  description: string;
  customer_email: string;
  metadata: Record<string, unknown>;
  paid_at: string | null;
  created_at: string;
}

export interface TransactionListResponse {
  transactions: TransactionResponse[];
  pagination: PaginationResponse;
}
