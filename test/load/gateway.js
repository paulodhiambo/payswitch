// Gateway ramp-up load test: escalates from 10 → 50 → 100 VUs to find
// throughput limits and latency degradation under increasing concurrency.
//
// Usage:
//   k6 run test/load/gateway.js

import { check, sleep, group } from 'k6';
import http from 'k6/http';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Trend, Counter } from 'k6/metrics';

const BASE_URL = __ENV.GATEWAY_URL || 'https://localhost:8443';

// Only use BICs and currencies that exist in the routing table.
// Source is always BANKUS33 (bank-a client cert).
const DEST_BICS = ['BANKDEFF', 'BANKGB2L'];
const DEST_ACCOUNTS = ['ACC-B', 'ACC-C'];
const CURRENCIES = ['USD', 'EUR', 'GBP'];

// Custom metrics
const paymentDuration = new Trend('payment_duration', true);
const settledCount = new Counter('payments_settled');
const failedCount = new Counter('payments_failed');

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 0 },
  ],
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('../../client-bank-a-cert.pem'),
      key: open('../../client-bank-a-key.pem'),
    },
  ],
  thresholds: {
    http_req_duration: ['p(95)<60000'],
    http_req_failed: ['rate<0.05'],
    payments_settled: ['count>0'],
  },
};

export default function () {
  group('submit payment', () => {
    const destIdx = randomIntBetween(0, DEST_BICS.length - 1);
    const id = `load-${Date.now()}-${randomString(8)}`;

    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: DEST_BICS[destIdx],
      dest_account: DEST_ACCOUNTS[destIdx],
      amount: randomIntBetween(100, 100000),
      currency: CURRENCIES[randomIntBetween(0, CURRENCIES.length - 1)],
      debtor_name: `LoadTest VU${__VU}`,
      creditor_name: `Recipient ${randomString(4)}`,
      remittance_info: `Load test iteration ${__ITER}`,
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `load-${randomString(12)}-${Date.now()}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);
    paymentDuration.add(resp.timings.duration);

    const isCreated = resp.status === 201;
    check(resp, {
      'status is 201': (r) => r.status === 201,
    });

    if (isCreated) {
      const status = resp.json('status');
      if (status === 'SETTLED' || status === 'COMMITTED') {
        settledCount.add(1);
      } else {
        failedCount.add(1);
      }
    } else {
      failedCount.add(1);
    }

    // For a subset of successful payments, verify GET retrieval
    if (isCreated && __ITER % 5 === 0) {
      const statusResp = http.get(`${BASE_URL}/payments/${id}`);
      check(statusResp, {
        'GET status is 200': (r) => r.status === 200,
      });
    }
  });

  sleep(randomIntBetween(0.5, 2));
}
