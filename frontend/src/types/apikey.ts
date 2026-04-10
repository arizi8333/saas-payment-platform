import { PaginationResponse } from './common';

export interface CreateAPIKeyRequest {
  name: string;
}

export interface APIKeyResponse {
  id: string;
  name: string;
  key_prefix: string;
  is_active: boolean;
  rate_limit: number;
  last_used_at: string | null;
  expires_at: string | null;
  created_at: string;
}

export interface APIKeyCreatedResponse extends APIKeyResponse {
  full_key: string;
}

export interface APIKeyListResponse {
  keys: APIKeyResponse[];
  pagination: PaginationResponse;
}
