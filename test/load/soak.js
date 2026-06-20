// Soak test: moderate sustained load over 10 minutes to detect
// memory leaks, connection pool exhaustion, or slowdowns.
import { check, sleep, group } from 'k6';
import http from 'k6/http';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:8080';

const BICS = ['BANKDEFF', 'BANKFRPP', 'BANKGB2L'];
const CURRENCIES = ['USD', 'EUR', 'GBP'];

export const options = {
  stages: [
    { duration: '2m', target: 30 },
    { duration: '8m', target: 30 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<3000', 'avg<1000'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('submit payment', () => {
    const id = `soak-${Date.now()}-${randomString(6)}`;
    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: BICS[randomIntBetween(0, BICS.length - 1)],
      dest_account: `ACCT-${randomString(10)}`,
      amount: randomIntBetween(1000, 50000),
      currency: CURRENCIES[randomIntBetween(0, CURRENCIES.length - 1)],
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `soak-${Date.now()}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);
    check(resp, {
      'accepted (201) or non-server error': (r) => r.status < 500,
    });
  });

  sleep(randomIntBetween(0.5, 1.5));
}
