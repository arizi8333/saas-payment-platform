import { api } from './api';
import { SuccessResponse, LoginResponse, UserProfileResponse, RegisterRequest, LoginRequest } from '@/types';

export const authService = {
  register: (data: RegisterRequest) =>
    api.post<SuccessResponse<UserProfileResponse>>('/auth/register', data),

  login: (data: LoginRequest) =>
    api.post<SuccessResponse<LoginResponse>>('/auth/login', data),

  getProfile: () =>
    api.get<SuccessResponse<UserProfileResponse>>('/users/profile'),
};
