export interface PaginationRequest {
  page: number;
  page_size: number;
}

export interface PaginationResponse {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface ErrorResponse {
  message: string;
  code: number;
}

export interface SuccessResponse<T> {
  success: boolean;
  data: T;
}
