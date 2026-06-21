export interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface BankRef {
  bic: string;
  name: string;
}

export interface Bank {
  id: string;
  bic: string;
  name: string;
  country: string;
  status: 'APPLICATION' | 'SANDBOX' | 'CERTIFICATION' | 'PRODUCTION_ACTIVE' | 'SUSPENDED' | 'DECOMMISSIONED';
  settlementAccount: string;
  notes?: string;
  apiBaseURL?: string;
  apiEnabled?: boolean;
  callbackURL?: string;
  lookupAPIURL?: string;
  paymentAPIURL?: string;
  statusCheckAPIURL?: string;
  onboardedAt: string | null;
  createdAt: string;
}

export interface Certificate {
  id: string;
  bankId: string;
  subject: string;
  fingerprint: string;
  notBefore: string;
  notAfter: string;
  status: 'ACTIVE' | 'REVOKED' | 'EXPIRED';
  issuedAt: string;
  revokedAt: string | null;
}

export interface TransactionEvent {
  event: string;
  occurredAt: string;
  detail: Record<string, any> | null;
}

export interface TransactionSummary {
  paymentId: string;
  endToEndId: string;
  sourceBank: BankRef;
  destinationBank: BankRef;
  amount: number;
  currency: string;
  status: string;
  abortReason: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface TransactionDetail extends TransactionSummary {
  uetr: string | null;
  instructionId: string | null;
  chargeBearer: string | null;
  settlementDate: string | null;
  debtorName: string | null;
  creditorName: string | null;
  purposeCode: string | null;
  remittanceInfo: string | null;
  timeline: TransactionEvent[];
}

export interface DashboardBucket {
  periodStart: string;
  totalTransactions: number;
  successCount: number;
  abortCount: number;
  successRate: number;
  p99LatencyMs: number | null;
  totalAmountMinorUnits: number;
}

export interface DashboardSummary {
  range: string;
  buckets: DashboardBucket[];
}

export interface AbortReason {
  category: string;
  count: number;
  percentage: number;
}

export interface AbortReasonBreakdown {
  range: string;
  total: number;
  reasons: AbortReason[];
}

export interface SettlementWindowSummary {
  id: string;
  openedAt: string;
  closedAt: string | null;
  status: string;
  settlementDate: string;
  currency: string;
  netPositionMinorUnits: number | null;
}

export interface SettlementPosition {
  bic: string;
  bankName: string;
  sentMinorUnits: number;
  receivedMinorUnits: number;
  netMinorUnits: number;
  transactionCount: number;
}

export interface SettlementWindow extends SettlementWindowSummary {
  positions: SettlementPosition[];
}

export interface AuditActor {
  id: string;
  email: string;
  role: string;
}

export interface AuditLogEntry {
  id: string;
  actor: AuditActor;
  action: string;
  targetType: string;
  targetId: string;
  diff: Record<string, any> | null;
  occurredAt: string;
}

export interface ExportJob {
  jobId: string;
  status: string;
  format: string;
  downloadUrl: string | null;
  rowCount: number | null;
  error: string | null;
  createdAt: string;
  completedAt: string | null;
}

export interface MeResponse {
  userId: string;
  username: string;
  role: string;
  participantId: string;
  bankId: string | null;
  bankName: string | null;
}
