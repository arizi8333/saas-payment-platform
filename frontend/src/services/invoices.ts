import { api } from './api';
import { SuccessResponse, InvoiceResponse, InvoiceListResponse } from '@/types';

export const invoicesService = {
  list: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<InvoiceListResponse>>(`/invoices?page=${page}&page_size=${pageSize}`),

  get: (id: string) =>
    api.get<SuccessResponse<InvoiceResponse>>(`/invoices/${id}`),
};
