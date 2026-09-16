# 压测与正确性验证

本文档用于面试展示：说明如何准备数据、发起压测、收集指标，以及如何证明“没有超卖、没有重复下单”。

## 目标

- 性能压测：观察 QPS、平均延迟、P95/P99、错误率、Redis/Go/MySQL 消耗。
- 正确性压测：验证库存不会扣成负数、同一用户不会重复下单、Stream 堆积最终能被消费。
- 可靠性压测：验证消费者失败后的 pending list 补偿和死信降级路径。

本地压测不等价于真实几十万在线用户。面试时重点讲清楚：本地主要验证链路正确性和单机容量拐点，海量用户需要在多实例、Redis Cluster、压测机集群和真实网络条件下验证。

## 准备环境

```bash
go test ./...
go test -tags=integration ./services -run TestSeckillLuaIntegration -count=1
go run main/main.go
```

默认服务地址：

```text
Backend: http://127.0.0.1:8081
MySQL:   127.0.0.1:3308 / qppt / root / 123456
Redis:   127.0.0.1:6378
```

## 准备 token

项目验证码固定为 `000000`，可以用下面流程拿 token：

```bash
PHONE=13686869696

curl -s -X POST "http://127.0.0.1:8081/user/code?phone=${PHONE}"

TOKEN=$(curl -s -X POST "http://127.0.0.1:8081/user/login" \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"${PHONE}\",\"code\":\"000000\"}" \
  | jq -r '.data')

echo "$TOKEN"
```

如果要做“一人一单”正确性压测，需要准备多个手机号登录得到多个 token；如果用同一个 token 并发请求，预期结果是只有 1 单成功，其余返回“禁止重复下单”。

## 准备秒杀券

示例数据里 `voucherId=8` 的初始库存为 500。压测前建议重置库存和订单，保证结果可解释：

```sql
UPDATE tb_seckill_voucher SET stock = 500 WHERE voucher_id = 8;
DELETE FROM tb_voucher_order WHERE voucher_id = 8;
```

Redis 侧也要同步重置：

```bash
redis-cli DEL seckill:stock:8 seckill:order:8
redis-cli SET seckill:stock:8 500
redis-cli DEL stream.orders
redis-cli XGROUP CREATE stream.orders g1 $ MKSTREAM
```

如果消费组已存在，`XGROUP CREATE` 会报 `BUSYGROUP`，可以忽略。

## 使用 hey

```bash
hey -n 1000 -c 200 \
  -m POST \
  -H "authorization: ${TOKEN}" \
  http://127.0.0.1:8081/voucher-order/seckill/8
```

同一个 token 的压测用于验证“一人一单”；多个 token 的压测用于验证库存扣减和吞吐。

## 使用 k6

`test/k6-seckill.js` 可以作为脚本模板：

```javascript
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 200,
  iterations: 1000,
};

const token = __ENV.TOKEN;
const voucherId = __ENV.VOUCHER_ID || '8';

export default function () {
  const res = http.post(`http://127.0.0.1:8081/voucher-order/seckill/${voucherId}`, null, {
    headers: { authorization: token },
  });
  check(res, {
    'http 200': (r) => r.status === 200,
    'json response': (r) => r.json('success') !== undefined,
  });
}
```

运行：

```bash
TOKEN="$TOKEN" VOUCHER_ID=8 k6 run test/k6-seckill.js
```

## 结果记录模板

| 指标 | 数值 |
|---|---:|
| 环境 | MacBook / Go 1.26 / MySQL 8 Podman / Redis 7 |
| 工具 | hey 或 k6 |
| 请求数 | 1000 |
| 并发数 | 200 |
| voucherId | 8 |
| 初始库存 | 500 |
| QPS | 待填写 |
| 平均延迟 | 待填写 |
| P95 | 待填写 |
| P99 | 待填写 |
| HTTP 非 200 | 待填写 |
| 成功下单数 | 待填写 |
| 业务失败数 | 待填写 |
| Redis Stream pending | 待填写 |
| 是否超卖 | 否 |
| 是否重复下单 | 否 |

## 正确性 SQL

```sql
-- 成功订单数不能超过初始库存
SELECT COUNT(*) AS order_count
FROM tb_voucher_order
WHERE voucher_id = 8;

-- 库存不能小于 0
SELECT voucher_id, stock
FROM tb_seckill_voucher
WHERE voucher_id = 8;

-- 同一用户不能重复下单
SELECT user_id, COUNT(*) AS cnt
FROM tb_voucher_order
WHERE voucher_id = 8
GROUP BY user_id
HAVING cnt > 1;
```

## Redis 队列观察

```bash
redis-cli XLEN stream.orders
redis-cli XPENDING stream.orders g1
redis-cli XPENDING stream.orders g1 - + 10
```

面试讲法：

- Lua 成功后请求立即返回 orderId，MySQL 落库由消费者异步完成。
- 如果消费者短暂失败，消息留在 pending list，恢复后继续消费。
- 如果超过最大重试次数，写入死信表并 ACK，避免单条坏消息阻塞队列。
