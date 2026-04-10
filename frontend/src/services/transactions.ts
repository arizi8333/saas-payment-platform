import { api } from './api';
import { SuccessResponse, TransactionResponse, TransactionListResponse, CreateTransactionRequest } from '@/types';

export const transactionsService = {
  create: (data: CreateTransactionRequest) =>
    api.post<SuccessResponse<TransactionResponse>>('/transactions', data),

  list: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<TransactionListResponse>>(`/transactions?page=${page}&page_size=${pageSize}`),

  get: (id: string) =>
    api.get<SuccessResponse<TransactionResponse>>(`/transactions/${id}`),
};
