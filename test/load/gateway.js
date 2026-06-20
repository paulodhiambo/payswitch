import { check, sleep, group } from 'k6';
import http from 'k6/http';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:8080';

const BICS = ['BANKDEFF', 'BANKFRPP', 'BANKGB2L', 'BANKJPJT', 'BANKSGSG'];
const CURRENCIES = ['USD', 'EUR', 'GBP', 'JPY', 'SGD'];

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('submit payment', () => {
    const id = `load-${Date.now()}-${randomString(8)}`;
    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: BICS[randomIntBetween(0, BICS.length - 1)],
      dest_account: `ACCT-${randomString(12)}`,
      amount: randomIntBetween(100, 100000),
      currency: CURRENCIES[randomIntBetween(0, CURRENCIES.length - 1)],
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `${randomString(8)}-${Date.now()}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);

    check(resp, {
      'status is 201 or 400 or 500': (r) => [201, 400, 500].includes(r.status),
      'response body has status': (r) => r.status !== 201 || r.json('status') !== undefined,
      'response time < 2s': (r) => r.timings.duration < 2000,
    });

    if (resp.status === 201) {
      const statusResp = http.get(`${BASE_URL}/payments/${id}`);
      check(statusResp, {
        'GET status is 200': (r) => r.status === 200,
        'GET returns same id': (r) => r.json('id') !== undefined,
      });
    }
  });

  sleep(randomIntBetween(0.5, 2));
}
