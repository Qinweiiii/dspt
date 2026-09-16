import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: Number(__ENV.VUS || 200),
  iterations: Number(__ENV.ITERATIONS || 1000),
};

const token = __ENV.TOKEN;
const voucherId = __ENV.VOUCHER_ID || '8';
const baseUrl = __ENV.BASE_URL || 'http://127.0.0.1:8081';

export default function () {
  const res = http.post(`${baseUrl}/voucher-order/seckill/${voucherId}`, null, {
    headers: { authorization: token },
  });

  check(res, {
    'http 200': (r) => r.status === 200,
    'json response': (r) => r.json('success') !== undefined,
  });
}
