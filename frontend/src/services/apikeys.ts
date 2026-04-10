import { api } from './api';
import { SuccessResponse, APIKeyCreatedResponse, APIKeyListResponse, CreateAPIKeyRequest } from '@/types';

export const apiKeysService = {
  create: (data: CreateAPIKeyRequest) =>
    api.post<SuccessResponse<APIKeyCreatedResponse>>('/api-keys', data),

  list: (page = 1, pageSize = 10) =>
    api.get<SuccessResponse<APIKeyListResponse>>(`/api-keys?page=${page}&page_size=${pageSize}`),

  revoke: (id: string) =>
    api.delete<SuccessResponse<null>>(`/api-keys/${id}`),
};
