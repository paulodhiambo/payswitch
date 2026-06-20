// Helper to generate UUIDs
export function generateUUID(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

export interface Bank {
  id: string;
  bic: string;
  name: string;
  country: string;
  status: 'APPLICATION' | 'SANDBOX' | 'CERTIFICATION' | 'PRODUCTION_ACTIVE' | 'SUSPENDED' | 'DECOMMISSIONED';
  settlementAccount: string;
  onboardedAt: string | null;
  createdAt: string;
  notes?: string;
  lookupEndpoint: string;
  paymentEndpoint: string;
  statusEndpoint: string;
  certDuration: number;
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
  event: 'RECEIVED' | 'VALIDATED' | 'QUOTED' | 'SCREENED' | 'RESERVED' | 'COMMITTED' | 'SETTLED' | 'ABORTED';
  occurredAt: string;
  detail?: Record<string, any> | null;
}

export interface Transaction {
  paymentId: string;
  endToEndId: string;
  sourceBank: { bic: string; name: string };
  destinationBank: { bic: string; name: string };
  amount: number; // in cents/minor units
  currency: string;
  status: 'RECEIVED' | 'VALIDATED' | 'QUOTED' | 'SCREENED' | 'RESERVED' | 'COMMITTED' | 'SETTLED' | 'ABORTED';
  abortReason: string | null;
  createdAt: string;
  updatedAt: string;
  uetr: string;
  instructionId: string;
  chargeBearer: 'DEBT' | 'CRED' | 'SHAR' | 'SLEV';
  settlementDate: string;
  debtorName: string;
  creditorName: string;
  purposeCode: string;
  remittanceInfo: string;
  timeline: TransactionEvent[];
}

export interface SettlementWindow {
  id: string;
  openedAt: string;
  closedAt: string | null;
  status: 'OPEN' | 'CLOSED' | 'SETTLED' | 'DISPUTED';
  settlementDate: string;
  currency: string;
  netPositionMinorUnits?: number | null;
  positions?: Array<{
    bic: string;
    bankName: string;
    sentMinorUnits: number;
    receivedMinorUnits: number;
    netMinorUnits: number;
    transactionCount: number;
  }>;
}

export interface User {
  id: string;
  email: string;
  role: 'SWITCH_ADMIN' | 'SWITCH_OPS' | 'BANK_ADMIN' | 'BANK_OPERATOR' | 'BANK_VIEWER';
  participantId: string | null;
  status: 'ACTIVE' | 'SUSPENDED';
  createdAt: string;
}

export interface AuditLogEntry {
  id: string;
  actor: {
    id: string;
    email: string;
    role: string;
  };
  action: 'BANK_STATUS_CHANGED' | 'CERTIFICATE_ISSUED' | 'CERTIFICATE_REVOKED' | 'USER_CREATED' | 'USER_ROLE_CHANGED' | 'FIELD_REVEALED';
  targetType: 'BANK' | 'CERTIFICATE' | 'USER' | 'TRANSACTION';
  targetId: string;
  diff: Record<string, any> | null;
  occurredAt: string;
}

// ---------------------------------------------------------
// Seed Data
// ---------------------------------------------------------

export const mockBanks: Bank[] = [
  {
    id: 'eqbl-ke-uuid',
    bic: 'EQBLKENA',
    name: 'Equity Bank Kenya',
    country: 'KE',
    status: 'PRODUCTION_ACTIVE',
    settlementAccount: 'KEACB0000001234',
    onboardedAt: '2026-01-15T08:30:00Z',
    createdAt: '2025-12-01T10:00:00Z',
    notes: 'Primary retail banking participant.',
    lookupEndpoint: 'https://api.eqblkena.bank/lookup/v1',
    paymentEndpoint: 'https://api.eqblkena.bank/payment/v1',
    statusEndpoint: 'https://api.eqblkena.bank/status/v1',
    certDuration: 365
  },
  {
    id: 'kcbl-ke-uuid',
    bic: 'KCBLKENX',
    name: 'KCB Bank Kenya',
    country: 'KE',
    status: 'PRODUCTION_ACTIVE',
    settlementAccount: 'KEACB0000005678',
    onboardedAt: '2026-02-10T09:15:00Z',
    createdAt: '2025-12-05T11:00:00Z',
    notes: 'Largest commercial bank participant.',
    lookupEndpoint: 'https://api.kcblkenx.bank/lookup/v1',
    paymentEndpoint: 'https://api.kcblkenx.bank/payment/v1',
    statusEndpoint: 'https://api.kcblkenx.bank/status/v1',
    certDuration: 365
  },
  {
    id: 'scbl-ke-uuid',
    bic: 'SCBLKENA',
    name: 'Standard Chartered Kenya',
    country: 'KE',
    status: 'CERTIFICATION',
    settlementAccount: 'KEACB0000009999',
    onboardedAt: null,
    createdAt: '2026-03-01T14:00:00Z',
    notes: 'Currently undergoing compliance review on certificates.',
    lookupEndpoint: 'https://api.scblkena.bank/lookup/v1',
    paymentEndpoint: 'https://api.scblkena.bank/payment/v1',
    statusEndpoint: 'https://api.scblkena.bank/status/v1',
    certDuration: 365
  },
  {
    id: 'absa-ke-uuid',
    bic: 'ABSAKENA',
    name: 'Absa Bank Kenya',
    country: 'KE',
    status: 'SANDBOX',
    settlementAccount: 'KEACB0000004444',
    onboardedAt: null,
    createdAt: '2026-05-20T08:00:00Z',
    notes: 'Testing ISO 20022 gateway integrations.',
    lookupEndpoint: 'https://api.absakena.bank/lookup/v1',
    paymentEndpoint: 'https://api.absakena.bank/payment/v1',
    statusEndpoint: 'https://api.absakena.bank/status/v1',
    certDuration: 365
  },
  {
    id: 'coop-ke-uuid',
    bic: 'COOPKENA',
    name: 'Co-operative Bank of Kenya',
    country: 'KE',
    status: 'APPLICATION',
    settlementAccount: 'KEACB0000007777',
    onboardedAt: null,
    createdAt: '2026-06-18T16:30:00Z',
    notes: 'Pending initial board review.',
    lookupEndpoint: 'https://api.coopkena.bank/lookup/v1',
    paymentEndpoint: 'https://api.coopkena.bank/payment/v1',
    statusEndpoint: 'https://api.coopkena.bank/status/v1',
    certDuration: 365
  }
];

export const mockCertificates: Certificate[] = [
  {
    id: generateUUID(),
    bankId: 'eqbl-ke-uuid',
    subject: 'CN=equity-bank,O=Equity Bank,C=KE',
    fingerprint: '3F:9A:8B:4C:E6:D3:B1:0E:5F:A7:28:C2:E9:D8:1A:30:B2:C5:D1:E4:F3:A2:9B:C8:D7:E6:4A:5F:6B:7C:8D',
    notBefore: '2026-01-10T00:00:00Z',
    notAfter: '2027-01-10T23:59:59Z',
    status: 'ACTIVE',
    issuedAt: '2026-01-10T08:45:00Z',
    revokedAt: null
  },
  {
    id: generateUUID(),
    bankId: 'kcbl-ke-uuid',
    subject: 'CN=kcb-bank,O=KCB Bank Group,C=KE',
    fingerprint: 'A1:B2:C3:D4:E5:F6:77:88:99:00:AA:BB:CC:DD:EE:FF:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00',
    notBefore: '2026-02-08T00:00:00Z',
    notAfter: '2027-02-08T23:59:59Z',
    status: 'ACTIVE',
    issuedAt: '2026-02-08T10:00:00Z',
    revokedAt: null
  },
  {
    id: generateUUID(),
    bankId: 'scbl-ke-uuid',
    subject: 'CN=stanchart-kenya,O=Standard Chartered,C=KE',
    fingerprint: 'CD:4A:8B:E9:32:FF:10:98:88:AB:C3:72:0D:1A:EF:66:5E:4C:3B:2A:1B:0C:9F:8E:7D:6C:5B:4A:3F:2E:1D',
    notBefore: '2026-03-05T00:00:00Z',
    notAfter: '2027-03-05T23:59:59Z',
    status: 'ACTIVE',
    issuedAt: '2026-03-05T14:22:00Z',
    revokedAt: null
  }
];

export const mockUsers: User[] = [
  {
    id: 'usr-admin-uuid',
    email: 'admin@payment-switch.example.com',
    role: 'SWITCH_ADMIN',
    participantId: null,
    status: 'ACTIVE',
    createdAt: '2025-11-20T10:00:00Z'
  },
  {
    id: 'usr-ops-uuid',
    email: 'monitoring@payment-switch.example.com',
    role: 'SWITCH_OPS',
    participantId: null,
    status: 'ACTIVE',
    createdAt: '2025-11-22T08:00:00Z'
  },
  {
    id: 'usr-eq-admin-uuid',
    email: 'alice@equity.ke',
    role: 'BANK_ADMIN',
    participantId: 'eqbl-ke-uuid',
    status: 'ACTIVE',
    createdAt: '2026-01-16T09:00:00Z'
  },
  {
    id: 'usr-eq-ops-uuid',
    email: 'bob@equity.ke',
    role: 'BANK_OPERATOR',
    participantId: 'eqbl-ke-uuid',
    status: 'ACTIVE',
    createdAt: '2026-01-18T10:15:00Z'
  },
  {
    id: 'usr-kcb-admin-uuid',
    email: 'charles@kcb.co.ke',
    role: 'BANK_ADMIN',
    participantId: 'kcbl-ke-uuid',
    status: 'ACTIVE',
    createdAt: '2026-02-12T11:00:00Z'
  },
  {
    id: 'usr-kcb-viewer-uuid',
    email: 'david@kcb.co.ke',
    role: 'BANK_VIEWER',
    participantId: 'kcbl-ke-uuid',
    status: 'ACTIVE',
    createdAt: '2026-02-15T15:30:00Z'
  }
];

export const mockAuditLogs: AuditLogEntry[] = [
  {
    id: generateUUID(),
    actor: { id: 'usr-admin-uuid', email: 'admin@payment-switch.example.com', role: 'SWITCH_ADMIN' },
    action: 'USER_CREATED',
    targetType: 'USER',
    targetId: 'usr-eq-admin-uuid',
    diff: { email: 'alice@equity.ke', role: 'BANK_ADMIN', participantId: 'eqbl-ke-uuid' },
    occurredAt: '2026-01-16T09:00:00Z'
  },
  {
    id: generateUUID(),
    actor: { id: 'usr-admin-uuid', email: 'admin@payment-switch.example.com', role: 'SWITCH_ADMIN' },
    action: 'BANK_STATUS_CHANGED',
    targetType: 'BANK',
    targetId: 'eqbl-ke-uuid',
    diff: { before: 'CERTIFICATION', after: 'PRODUCTION_ACTIVE' },
    occurredAt: '2026-01-15T08:30:00Z'
  },
  {
    id: generateUUID(),
    actor: { id: 'usr-eq-admin-uuid', email: 'alice@equity.ke', role: 'BANK_ADMIN' },
    action: 'CERTIFICATE_ISSUED',
    targetType: 'CERTIFICATE',
    targetId: 'cert-1-uuid',
    diff: { subject: 'CN=equity-bank,O=Equity Bank,C=KE', status: 'ACTIVE' },
    occurredAt: '2026-01-10T08:45:00Z'
  },
  {
    id: generateUUID(),
    actor: { id: 'usr-kcb-admin-uuid', email: 'charles@kcb.co.ke', role: 'BANK_ADMIN' },
    action: 'USER_CREATED',
    targetType: 'USER',
    targetId: 'usr-kcb-viewer-uuid',
    diff: { email: 'david@kcb.co.ke', role: 'BANK_VIEWER', participantId: 'kcbl-ke-uuid' },
    occurredAt: '2026-02-15T15:30:00Z'
  },
  {
    id: generateUUID(),
    actor: { id: 'usr-admin-uuid', email: 'admin@payment-switch.example.com', role: 'SWITCH_ADMIN' },
    action: 'BANK_STATUS_CHANGED',
    targetType: 'BANK',
    targetId: 'scbl-ke-uuid',
    diff: { before: 'SANDBOX', after: 'CERTIFICATION' },
    occurredAt: '2026-03-01T14:00:00Z'
  }
];

// Helper to generate transaction events
function createTimeline(createdAtStr: string, status: string, abortReason?: string | null): TransactionEvent[] {
  const baseTime = new Date(createdAtStr).getTime();
  const timeline: TransactionEvent[] = [
    { event: 'RECEIVED', occurredAt: new Date(baseTime).toISOString() },
    { event: 'VALIDATED', occurredAt: new Date(baseTime + 100).toISOString() }
  ];

  if (status === 'ABORTED' && abortReason === 'VALIDATION_FAILED') {
    timeline.push({ event: 'ABORTED', occurredAt: new Date(baseTime + 150).toISOString(), detail: { reason: 'VALIDATION_FAILED', error: 'Invalid creditor account IBAN format' } });
    return timeline;
  }

  timeline.push({ event: 'QUOTED', occurredAt: new Date(baseTime + 500).toISOString(), detail: { fee: 150, quoteId: generateUUID().substring(0, 8) } });
  timeline.push({ event: 'SCREENED', occurredAt: new Date(baseTime + 1200).toISOString(), detail: { cleared: true, provider: 'DowJonesCompliance' } });

  if (status === 'ABORTED' && abortReason === 'COMPLIANCE_FAILED') {
    timeline.push({ event: 'ABORTED', occurredAt: new Date(baseTime + 1300).toISOString(), detail: { reason: 'COMPLIANCE_FAILED', match: 'Sanction List Match on Creditor Name' } });
    return timeline;
  }

  // Calculate reservation expiry (e.g. 30 seconds after screening)
  const reservedTime = baseTime + 1500;
  const expiresAt = new Date(reservedTime + 30000).toISOString();
  timeline.push({ event: 'RESERVED', occurredAt: new Date(reservedTime).toISOString(), detail: { expiresAt } });

  if (status === 'RESERVED') {
    return timeline;
  }

  if (status === 'ABORTED' && abortReason === 'RESERVATION_EXPIRED') {
    timeline.push({ event: 'ABORTED', occurredAt: new Date(reservedTime + 30100).toISOString(), detail: { reason: 'RESERVATION_EXPIRED' } });
    return timeline;
  }

  const commitTime = reservedTime + 1000;
  timeline.push({ event: 'COMMITTED', occurredAt: new Date(commitTime).toISOString() });

  if (status === 'COMMITTED') {
    return timeline;
  }

  const settleTime = commitTime + 15000; // e.g. settled later
  timeline.push({ event: 'SETTLED', occurredAt: new Date(settleTime).toISOString(), detail: { clearingWindowId: 'win-9988' } });

  return timeline;
}

// Generate transactions
export const mockTransactions: Transaction[] = [];

const debtorFirstNames = ['Amara', 'Kofi', 'Fatou', 'Moussa', 'Zola', 'Chinedu', 'Tariq', 'Nneka', 'Kwame', 'Simba'];
const debtorLastNames = ['Osei', 'Diallo', 'Keita', 'Mensah', 'Mthembu', 'Okonkwo', 'Kamau', 'Adebayo', 'Toure', 'Ndiaye'];
const creditorNames = ['Safaricom Pay', 'Jumia East Africa', 'MTN Uganda Ltd', 'Airtel Money KE', 'Equity Merchant Service', 'KCB Treasury Client', 'KRA Payment Portal'];
const remittanceInfos = ['School Fees Payment', 'Supplier Invoice #2039', 'Consultancy Fees', 'Family Support', 'Mobile Money Cashin', 'E-commerce Purchase', 'Tax Clearance'];

// Generate transactions back to 7 days ago
const nowTime = new Date().getTime();
for (let i = 0; i < 45; i++) {
  const hoursAgo = i * 4 + Math.floor(Math.random() * 3);
  const createdAt = new Date(nowTime - hoursAgo * 60 * 60 * 1000).toISOString();
  
  // Alternate source and dest banks
  const sourceIndex = i % 2 === 0 ? 0 : 1; // Equity or KCB
  const destIndex = i % 2 === 0 ? 1 : 0;
  const sourceBank = mockBanks[sourceIndex];
  const destinationBank = mockBanks[destIndex];
  
  const amount = Math.floor(Math.random() * 95000) + 5000; // 50.00 to 1000.00
  const currency = 'KES';
  
  // Decide statuses
  let status: Transaction['status'] = 'SETTLED';
  let abortReason: string | null = null;
  
  if (i === 0) {
    status = 'RESERVED'; // One currently active reservation for the timeline countdown
  } else if (i === 1) {
    status = 'COMMITTED'; // Awaiting clearing
  } else if (i % 8 === 0) {
    status = 'ABORTED';
    abortReason = i % 16 === 0 ? 'COMPLIANCE_FAILED' : 'VALIDATION_FAILED';
  }
  
  const paymentId = generateUUID();
  const debtorName = `${debtorFirstNames[i % debtorFirstNames.length]} ${debtorLastNames[(i + 3) % debtorLastNames.length]}`;
  const creditorName = creditorNames[i % creditorNames.length];
  const remittanceInfo = remittanceInfos[i % remittanceInfos.length];
  
  mockTransactions.push({
    paymentId,
    endToEndId: `E2E-20260620-${1000 + i}`,
    sourceBank: { bic: sourceBank.bic, name: sourceBank.name },
    destinationBank: { bic: destinationBank.bic, name: destinationBank.name },
    amount,
    currency,
    status,
    abortReason,
    createdAt,
    updatedAt: new Date(new Date(createdAt).getTime() + 18000).toISOString(),
    uetr: generateUUID(),
    instructionId: `INSTR-${20000 + i}`,
    chargeBearer: 'SHAR',
    settlementDate: createdAt.split('T')[0],
    debtorName,
    creditorName,
    purposeCode: 'GDDS',
    remittanceInfo,
    timeline: createTimeline(createdAt, status, abortReason)
  });
}

// Generate Settlement Windows
export const mockSettlementWindows: SettlementWindow[] = [
  {
    id: generateUUID(),
    openedAt: '2026-06-20T08:00:00Z',
    closedAt: null,
    status: 'OPEN',
    settlementDate: '2026-06-20',
    currency: 'KES',
    positions: [
      { bic: 'EQBLKENA', bankName: 'Equity Bank Kenya', sentMinorUnits: 2500000, receivedMinorUnits: 3400000, netMinorUnits: 900000, transactionCount: 15 },
      { bic: 'KCBLKENX', bankName: 'KCB Bank Kenya', sentMinorUnits: 3400000, receivedMinorUnits: 2500000, netMinorUnits: -900000, transactionCount: 15 }
    ]
  },
  {
    id: generateUUID(),
    openedAt: '2026-06-19T08:00:00Z',
    closedAt: '2026-06-19T17:00:00Z',
    status: 'SETTLED',
    settlementDate: '2026-06-19',
    currency: 'KES',
    positions: [
      { bic: 'EQBLKENA', bankName: 'Equity Bank Kenya', sentMinorUnits: 5800000, receivedMinorUnits: 5200000, netMinorUnits: -600000, transactionCount: 28 },
      { bic: 'KCBLKENX', bankName: 'KCB Bank Kenya', sentMinorUnits: 5200000, receivedMinorUnits: 5800000, netMinorUnits: 600000, transactionCount: 28 }
    ]
  },
  {
    id: generateUUID(),
    openedAt: '2026-06-18T08:00:00Z',
    closedAt: '2026-06-18T17:00:00Z',
    status: 'SETTLED',
    settlementDate: '2026-06-18',
    currency: 'KES',
    positions: [
      { bic: 'EQBLKENA', bankName: 'Equity Bank Kenya', sentMinorUnits: 4300000, receivedMinorUnits: 4900000, netMinorUnits: 600000, transactionCount: 22 },
      { bic: 'KCBLKENX', bankName: 'KCB Bank Kenya', sentMinorUnits: 4900000, receivedMinorUnits: 4300000, netMinorUnits: -600000, transactionCount: 22 }
    ]
  }
];

// Helper to compute dashboard summary buckets for the chart
export function getDashboardSummaryBuckets(bicFilter?: string | null, range: string = '24h') {
  // Aggregate mock transactions based on hours or days
  const now = new Date();
  const buckets: any[] = [];
  const hours = range === '1h' ? 1 : range === '6h' ? 6 : range === '24h' ? 24 : range === '7d' ? 168 : 720;
  const intervalHours = range === '7d' ? 24 : range === '30d' ? 24 : 1; // bucket by day or hour
  
  const numBuckets = Math.ceil(hours / intervalHours);
  
  for (let i = numBuckets - 1; i >= 0; i--) {
    const periodStart = new Date(now.getTime() - i * intervalHours * 60 * 60 * 1000);
    const periodEnd = new Date(periodStart.getTime() + intervalHours * 60 * 60 * 1000);
    
    // Filter transactions in this window
    const txsInBucket = mockTransactions.filter(tx => {
      const txTime = new Date(tx.createdAt);
      if (txTime < periodStart || txTime >= periodEnd) return false;
      if (bicFilter && tx.sourceBank.bic !== bicFilter && tx.destinationBank.bic !== bicFilter) return false;
      return true;
    });
    
    const success = txsInBucket.filter(tx => tx.status === 'SETTLED' || tx.status === 'COMMITTED');
    const aborts = txsInBucket.filter(tx => tx.status === 'ABORTED');
    
    const successCount = success.length;
    const abortCount = aborts.length;
    const totalTransactions = txsInBucket.length;
    const successRate = totalTransactions > 0 ? successCount / totalTransactions : 1.0;
    
    const totalAmount = txsInBucket.reduce((sum, tx) => sum + tx.amount, 0);
    
    // Fake latency aggregation
    const p99LatencyMs = totalTransactions > 0 
      ? Math.floor(Math.random() * 400) + 1200 // 1.2s to 1.6s range
      : null;
      
    buckets.push({
      periodStart: periodStart.toISOString(),
      totalTransactions,
      successCount,
      abortCount,
      successRate,
      p99LatencyMs,
      totalAmountMinorUnits: totalAmount
    });
  }
  
  return buckets;
}

export function getAbortReasonsBreakdown(bicFilter?: string | null) {
  const filtered = mockTransactions.filter(tx => {
    if (tx.status !== 'ABORTED') return false;
    if (bicFilter && tx.sourceBank.bic !== bicFilter && tx.destinationBank.bic !== bicFilter) return false;
    return true;
  });
  
  const countByCategory = {
    compliance: 0,
    timeout: 0,
    bank_error: 0,
    validation: 0,
    unknown: 0
  };
  
  filtered.forEach(tx => {
    if (tx.abortReason === 'COMPLIANCE_FAILED') {
      countByCategory.compliance++;
    } else if (tx.abortReason === 'VALIDATION_FAILED') {
      countByCategory.validation++;
    } else if (tx.abortReason === 'RESERVATION_EXPIRED') {
      countByCategory.timeout++;
    } else {
      countByCategory.unknown++;
    }
  });
  
  const total = filtered.length;
  
  return {
    range: '7d',
    total,
    reasons: Object.entries(countByCategory).map(([category, count]) => ({
      category,
      count,
      percentage: total > 0 ? (count / total) : 0
    }))
  };
}
