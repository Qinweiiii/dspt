# 水平扩容与海量秒杀设计

本文档用于回答面试问题：“如果用户量从本地测试增长到几十万甚至百万级并发，你怎么扩？”

## 当前单体边界

当前实现是 Go + Gin 单体服务，外部依赖为 MySQL 和 Redis。秒杀路径已经把最关键的状态放到 Redis：

- 登录态：`login:token:{token}`
- 秒杀库存：`seckill:stock:{voucherId}`
- 一人一单预检：`seckill:order:{voucherId}`
- 异步订单队列：`stream.orders`
- Feed、点赞、GEO、签到：Redis ZSet / Set / GEO / BitMap

因此 Go 应用层本身基本无状态，可以横向扩容。

## 应用层扩容

多个 Go 实例挂在负载均衡后：

```text
Client
  -> Nginx / SLB
    -> dspt-api-1
    -> dspt-api-2
    -> dspt-api-3
         |
         +-> Redis Cluster
         +-> MySQL
```

要求：

- 不把登录态、库存、订单状态存本机内存。
- 本地缓存只做“可丢失优化”，比如售罄标记、秒杀时间窗口。
- 本地缓存不作为最终一致性来源，最终仍由 Redis Lua 和 MySQL 乐观锁兜底。

## Redis 的职责

Redis 承担秒杀入口的高频状态：

- Lua 原子扣减库存，保证 Redis 层不超卖。
- Set 记录已下单用户，拦截重复请求。
- Stream 削峰，把同步落库变成异步消费。
- Rate limit / Bloom / whitelist 把无效请求挡在更前面。

单 Redis 实例到瓶颈后：

- 使用 Redis Cluster 分片。
- 按 `voucherId` 做 key tag，保证同一券的库存和订单集合尽量落到同一 slot，例如 `{seckill:8}:stock`。
- 热点券可以独立部署 Redis 实例或独立 cluster，和普通业务缓存隔离。

## 热点券隔离

最容易打爆系统的是少量超热门券。处理方式：

- 按券维度限流：当前代码用 `golang.org/x/time/rate` 为每个 voucherID 维护令牌桶。
- 热点券独立库存 key 和独立 Stream。
- 热点券预约/白名单预筛，未预约用户不进入 Lua。
- 大促前预热库存、时间窗口、GEO 等数据。

## 库存分片

当单个 Redis key 成为热点，可以把库存拆成多个桶：

```text
seckill:stock:{voucherId}:0
seckill:stock:{voucherId}:1
...
seckill:stock:{voucherId}:N
```

请求根据 userId hash 到某个桶，Lua 只扣对应桶；桶空时可以尝试少量备用桶。注意库存分片会增加复杂度：

- 需要准确初始化各桶库存总和。
- 需要处理桶间倾斜。
- 仍要保留 userId 去重集合，避免用户换桶重复下单。

面试时可以说明：当前项目没有实现库存分片，因为单体展示项目优先保证正确性和可读性；分片是大促场景的下一步演进。

## MySQL 的职责

MySQL 是最终一致性兜底：

- 订单表记录最终成功订单。
- `UPDATE tb_seckill_voucher SET stock = stock - 1 WHERE voucher_id = ? AND stock > 0` 做 DB 层乐观锁。
- 消费者落库时再次检查一人一单，防止 Redis 故障或消息重复造成重复订单。

规模继续上升时：

- 订单表按时间或 voucherId 分库分表。
- 历史订单归档。
- 读多写少接口使用缓存或只读副本。
- 死信表和消费监控独立告警。

## Stream 消费扩容

Redis Stream 支持消费者组：

- 多个消费者属于同一个 group，可以并行消费不同消息。
- pending list 记录已投递但未 ACK 的消息。
- 消费者恢复后补偿 pending 消息。
- 超过最大重试次数写死信表，避免队列被坏消息卡死。

扩容时可以：

- 多实例内启动多个 consumerName。
- 按 voucherId 分 stream，例如 `stream.orders.{voucherId}` 或 `stream.orders.hot`。
- 将高峰订单消息迁移到 Kafka / Pulsar，Redis Stream 保留中小规模场景。

## 性能压测 vs 正确性压测

本地不一定真实模拟几十万连接，但可以验证系统关键不变量：

- 成功订单数不能超过库存。
- 同一用户不能出现多笔同券订单。
- Redis 库存不能扣成负数。
- Stream pending 数量在消费者恢复后能下降。
- 死信消息不会阻塞后续消息。

真正的性能压测需要：

- 多台压测机，避免压测端先成为瓶颈。
- 多个 Go 实例和独立负载均衡。
- Redis/MySQL 监控：CPU、网络、慢查询、连接数、命令耗时。
- 分开测试读接口、秒杀入口、消费者落库能力。

## 面试回答摘要

这个项目的核心思路是“请求入口快速判定，Redis 原子削峰，MySQL 最终兜底”：

- Go 层无状态，方便水平扩容。
- Redis 承担高频、强原子、低延迟判断。
- Stream 把写 MySQL 从用户请求路径中移出去。
- DB 层乐观锁和唯一性校验保底。
- 对热点券使用限流、资格过滤、库存分片和资源隔离。
