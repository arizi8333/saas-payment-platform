import { PaginationResponse } from './common';

export interface InvoiceResponse {
  id: string;
  invoice_number: string;
  amount: number;
  currency: string;
  status: string;
  due_date: string;
  paid_at: string | null;
  transaction_id: string | null;
  subscription_id: string | null;
  created_at: string;
}

export interface InvoiceListResponse {
  invoices: InvoiceResponse[];
  pagination: PaginationResponse;
}
