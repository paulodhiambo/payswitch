import { apiClient } from './client';
import type {
  Bank,
  Certificate,
  PaginatedResult,
  TransactionSummary,
  TransactionDetail,
  TransactionEvent,
  DashboardSummary,
  AbortReasonBreakdown,
  SettlementWindowSummary,
  SettlementWindow,
  AuditLogEntry,
  ExportJob,
  MeResponse,
} from './types';

export const api = {
  getMe: () =>
    apiClient.get<MeResponse>('/me').then(r => r.data),
  getCsrfToken: () =>
    apiClient.get<{ token: string }>('/csrf-token').then(r => r.data.token),

  listBanks: (params?: { status?: string; page?: number; pageSize?: number }) =>
    apiClient.get<PaginatedResult<Bank>>('/banks', { params }).then(r => r.data),
  getBank: (bankId: string) =>
    apiClient.get<Bank>(`/banks/${bankId}`).then(r => r.data),
  createBank: (data: { bic: string; name: string; country: string; settlementAccount: string; notes?: string }) =>
    apiClient.post<Bank>('/banks', data).then(r => r.data),
  updateBankStatus: (bankId: string, status: string, reason?: string) =>
    apiClient.patch<Bank>(`/banks/${bankId}/status`, { status, reason }).then(r => r.data),
  updateBankAPI: (bankId: string, data: { apiBaseUrl: string; apiEnabled: boolean; lookupApiUrl: string; paymentApiUrl: string; statusCheckApiUrl: string }) =>
    apiClient.patch<Bank>(`/banks/${bankId}/api`, data).then(r => r.data),
  updateBankCallback: (bankId: string, callbackUrl: string) =>
    apiClient.patch<Bank>(`/banks/${bankId}/callback`, { callbackUrl }).then(r => r.data),

  listCertificates: (bankId: string) =>
    apiClient.get<Certificate[]>(`/banks/${bankId}/certificates`).then(r => r.data),
  createCertificate: (bankId: string, pem: string) =>
    apiClient.post<Certificate>(`/banks/${bankId}/certificates`, { pem }).then(r => r.data),
  revokeCertificate: (bankId: string, certId: string) =>
    apiClient.delete(`/banks/${bankId}/certificates/${certId}`),

  listTransactions: (params?: {
    status?: string; bic?: string; from?: string; to?: string;
    minAmount?: number; maxAmount?: number; page?: number; pageSize?: number;
  }) =>
    apiClient.get<PaginatedResult<TransactionSummary>>('/transactions', { params }).then(r => r.data),
  getTransaction: (paymentId: string) =>
    apiClient.get<TransactionDetail>(`/transactions/${paymentId}`).then(r => r.data),
  getTransactionTimeline: (paymentId: string) =>
    apiClient.get<TransactionEvent[]>(`/transactions/${paymentId}/timeline`).then(r => r.data),

  createExport: (params?: { format?: string; status?: string; bic?: string; from?: string; to?: string }) =>
    apiClient.get<ExportJob>('/transactions/export', { params }).then(r => r.data),
  getExportJob: (jobId: string) =>
    apiClient.get<ExportJob>(`/export-jobs/${jobId}`).then(r => r.data),

  getDashboardSummary: (range?: string) =>
    apiClient.get<DashboardSummary>('/dashboard/summary', { params: { range } }).then(r => r.data),
  getAbortReasons: (range?: string) =>
    apiClient.get<AbortReasonBreakdown>('/dashboard/abort-reasons', { params: { range } }).then(r => r.data),

  listSettlementWindows: (params?: { bic?: string; from?: string; to?: string; page?: number; pageSize?: number }) =>
    apiClient.get<PaginatedResult<SettlementWindowSummary>>('/settlement/windows', { params }).then(r => r.data),
  getSettlementWindow: (windowId: string) =>
    apiClient.get<SettlementWindow>(`/settlement/windows/${windowId}`).then(r => r.data),

  listAuditLog: (params?: { actorId?: string; action?: string; from?: string; to?: string; page?: number; pageSize?: number }) =>
    apiClient.get<PaginatedResult<AuditLogEntry>>('/audit-log', { params }).then(r => r.data),
};
