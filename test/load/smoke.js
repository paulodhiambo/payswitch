import { check, sleep, group } from 'k6';
import http from 'k6/http';
import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:8080';

const BICS = ['BANKDEFF'];
const CURRENCIES = ['USD'];

export const options = {
  vus: 1,
  iterations: 2,
  thresholds: {
    http_req_duration: ['p(95)<3000'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('submit basic payment', () => {
    const id = `smoke-${Date.now()}-${randomString(4)}`;
    const payload = JSON.stringify({
      end_to_end_id: id,
      destination_bic: 'BANKDEFF',
      dest_account: 'DE89370400440532013000',
      amount: 50000,
      currency: 'USD',
    });

    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `smoke-${Date.now()}`,
      },
    };

    const resp = http.post(`${BASE_URL}/payments`, payload, params);
    check(resp, {
      'submitted successfully': (r) => r.status === 201,
    });

    if (resp.status === 201) {
      const getResp = http.get(`${BASE_URL}/payments/${id}`);
      check(getResp, {
        'retrieved successfully': (r) => r.status === 200,
      });
    }
  });
}
