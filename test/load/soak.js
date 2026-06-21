// Soak test: moderate sustained load over 10 minutes to detect memory leaks,
// connection pool exhaustion, Kafka/Redpanda backpressure, or gradual latency
// degradation in the payment processing saga.
//
// Usage:
//   k6 run test/load/soak.js

import { check, sleep, group } from 'k6';
import http from 'k6/http';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Trend, Rate } from 'k6/metrics';

const BASE_URL = __ENV.GATEWAY_URL || 'https://localhost:8443';

// Only BICs and currencies present in the routing table
const DEST_BICS = ['BANKDEFF', 'BANKGB2L'];
const DEST_ACCOUNTS = ['ACC-B', 'ACC-C'];
const CURRENCIES = ['USD', 'EUR', 'GBP'];

// Custom metrics
const sagaDuration = new Trend('saga_duration', true);
const successRate = new Rate('payment_success_rate');

export const options = {
  stages: [
    { duration: '2m', target: 30 },
    { duration: '8m', target: 30 },
    { duration: '1m', target: 0 },
  ],
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('../../client-bank-a-cert.pem'),
      key: open('../../client-bank-a-key.pem'),
    },
  ],
  thresholds: {
    http_req_duration: ['p(95)<60000', 'avg<30000'],
    http_req_failed: ['rate<0.05'],
    payment_success_rate: ['rate>0.90'],
  },
};

export default function () {
  group('submit payment', () => {
    const destIdx = randomIntBetween(0, DEST_BICS.length - 1);
    const id = `soak-${Date.now()}-${randomString(6)}`;

    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: DEST_BICS[destIdx],
      dest_account: DEST_ACCOUNTS[destIdx],
      amount: randomIntBetween(1000, 50000),
      currency: CURRENCIES[randomIntBetween(0, CURRENCIES.length - 1)],
      debtor_name: `Soak VU${__VU}`,
      creditor_name: `Recipient ${randomString(4)}`,
      remittance_info: `Soak test iter ${__ITER}`,
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `soak-${randomString(10)}-${Date.now()}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);
    sagaDuration.add(resp.timings.duration);

    const created = resp.status === 201;
    successRate.add(created);

    check(resp, {
      'accepted (201)': (r) => r.status === 201,
    });
  });

  sleep(randomIntBetween(0.5, 1.5));
}
