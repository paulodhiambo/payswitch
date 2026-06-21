// Smoke test: 1 VU, 2 iterations — quick sanity check that the payment
// flow works end-to-end through Kong with mTLS.
//
// Usage:
//   k6 run test/load/smoke.js
//
// Override base URL:
//   k6 run --env GATEWAY_URL=https://localhost:8443 test/load/smoke.js

import { check, group } from 'k6';
import http from 'k6/http';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.GATEWAY_URL || 'https://localhost:8443';

export const options = {
  vus: 1,
  iterations: 2,
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('../../client-bank-a-cert.pem'),
      key: open('../../client-bank-a-key.pem'),
    },
  ],
  thresholds: {
    http_req_duration: ['p(95)<60000'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('submit basic payment', () => {
    const id = `smoke-${Date.now()}-${randomString(4)}`;
    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: 'BANKDEFF',
      dest_account: 'ACC-B',
      amount: 50000,
      currency: 'USD',
      debtor_name: 'Smoke Test Debtor',
      creditor_name: 'Smoke Test Creditor',
      remittance_info: 'Smoke test payment',
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `smoke-${Date.now()}-${randomString(4)}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);
    check(resp, {
      'status is 201': (r) => r.status === 201,
      'has payment id': (r) => r.status !== 201 || r.json('id') !== undefined,
      'status is SETTLED': (r) => r.status !== 201 || r.json('status') === 'SETTLED',
    });

    if (resp.status === 201) {
      const getResp = http.get(`${BASE_URL}/payments/${id}`);
      check(getResp, {
        'GET status is 200': (r) => r.status === 200,
        'GET returns correct e2e id': (r) =>
          r.status !== 200 || r.json('end_to_end_id') === id,
      });
    }
  });
}
