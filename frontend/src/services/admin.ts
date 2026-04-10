import { api } from './api';
import { SuccessResponse, DashboardStatsResponse } from '@/types';

export const adminService = {
  getStats: () =>
    api.get<SuccessResponse<DashboardStatsResponse>>('/admin/stats'),
};
