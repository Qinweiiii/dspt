# 有票 · 线下活动抢票平台

> 对标大麦网 + 小红书形态的线下活动抢票平台，覆盖活动浏览、社区图文、关注 Feed 流、限量票秒杀全链路。

---

## 目录

- [项目简介](#项目简介)
- [技术栈](#技术栈)
- [系统架构](#系统架构)
- [核心功能](#核心功能)
- [关键设计决策](#关键设计决策)
- [快速启动](#快速启动)
- [压测结果](#压测结果)
- [目录结构](#目录结构)

---

## 项目简介

**有票** 是一个基于 Go 独立实现的线下活动抢票平台后端，提供 40+ RESTful 接口。业务模型参考大麦网的票务逻辑与小红书的社区图文形态：

- **场馆（Venue）** 作为物理地点，承办多场不同类型的**活动（Shop）**
- 每场活动可以售卖普通票（**Voucher**）和限时特价秒杀票（**SeckillVoucher**）
- 用户可以发布活动**图文帖（Blog）**、关注其他用户、浏览**Feed 流**

系统核心挑战在于**限量秒杀场景下的高并发处理**，采用四层漏斗拦截架构，将 90%+ 的无效请求拦在 Redis Lua 之前。

---

## 技术栈

| 层次 | 技术选型 | 说明 |
|---|---|---|
| Web 框架 | Go 1.21 + Gin | 路由、中间件、参数绑定 |
| 数据库 | MySQL 8.0 | 主存储，乐观锁扣减库存 |
| 缓存 | Redis 7 | String / ZSet / GEO / BitMap / BF |
| 消息队列 | Redis Stream | 秒杀订单异步落库 |
| 分布式锁 | Redis SETNX + Lua | 原子释放，UUID 防误删 |
| 全局 ID | 时间戳 + Redis 自增 | 雪花变体，按天分 key |
| 限流 | golang.org/x/time/rate | 令牌桶，按 voucherID 独立桶 |
| 本地缓存 | sync.Map | 售罄标记 + 时间窗口，进程内拦截 |
| 布隆过滤器 | Redis Stack BF 模块 | 过滤无资格用户 |

> **为什么选 Redis Stream 而不是 Kafka？**
> Redis Stream 在此场景下满足全部需求：顺序消费、消费者组、pending list 补偿均已覆盖。Kafka 的核心优势是持久化的超大规模吞吐（百万 QPS）和多消费者组日志回放，在单体服务量级引入是过度设计。若后续多个下游系统需要同时消费同一条订单消息，再迁移 Kafka。

---

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Client（前端 / 小程序）                  │
└───────────────────────────┬─────────────────────────────────┘
                            │ HTTP
┌───────────────────────────▼─────────────────────────────────┐
│                    Gin Router + Middleware                   │
│    RefreshToken  ·  LoginRequired  ·  SeckillRateLimit      │
└──────┬──────────┬──────────┬──────────┬──────────┬──────────┘
       │          │          │          │          │
  UserHandler ShopHandler BlogHandler FollowHandler VoucherHandler
       │          │          │          │          │
┌──────▼──────────▼──────────▼──────────▼──────────▼──────────┐
│                        Service Layer                        │
│      UserSvc · ShopSvc · BlogSvc · FollowSvc · VoucherSvc  │
└──────┬──────────────────────────────────────┬───────────────┘
       │                                      │
┌──────▼──────────┐                 ┌─────────▼───────────────┐
│  Repository     │                 │        Redis            │
│  Layer (MySQL)  │                 │  String/ZSet/GEO        │
│                 │                 │  Stream/BitMap/BF       │
└─────────────────┘                 └─────────────────────────┘
                                             │ Stream 消费
                               ┌─────────────▼───────────────┐
                               │  VoucherOrderConsumer       │
                               │  重试计数 + 死信表兜底       │
                               └──────────────┬──────────────┘
                                              │ 写订单
                               ┌──────────────▼──────────────┐
                               │          MySQL              │
                               └─────────────────────────────┘
```

---

## 核心功能

### 秒杀抢票链路

**四层漏斗拦截**，把无效请求尽可能拦在代价最低的层次：

```
请求进入
    │
    ▼ 层 0：布隆过滤器（Redis BF.EXISTS）
    │  过滤无购票资格用户（黑名单 / 非会员专属票）
    │  → 无资格：0 DB / 0 Redis 写操作，直接返回
    │
    ▼ 层 1：本地售罄缓存（进程内 sync.Map）
    │  Lua 首次返回库存不足时写入，后续请求进程内拦截
    │  → 已售罄：0 网络开销，直接返回
    │
    ▼ 层 2：本地时间窗口缓存（进程内 sync.Map）
    │  启动时预热每张券的 begin_time / end_time
    │  → 不在活动期：0 网络开销，直接返回
    │
    ▼ 层 3：Redis Lua 原子脚本（1 次网络 RTT）
    │  原子执行：检查库存 → 一人一单 → 写 Stream
    │  → 超卖 / 重复下单：对应错误码返回
    │
    ▼ 层 4：Redis Stream 异步落库
       消费者重试计数（XPENDING delivery-count）
       超过 maxRetry=3 → 写死信表，ACK 不阻塞队列
       → MySQL 写订单，乐观锁扣减库存
```

**Lua 脚本核心逻辑：**

```lua
-- seckill.lua（原子执行，防超卖）
local stock = tonumber(redis.call('get', stockKey))
if (not stock) or (stock <= 0) then return 1 end      -- 库存不足

if tonumber(redis.call('sismember', orderKey, userId)) == 1 then
    return 2                                           -- 重复下单
end

redis.call('incrby', stockKey, -1)                    -- 扣库存
redis.call('sadd', orderKey, userId)                  -- 记录用户
redis.call('xadd', 'stream.orders', '*', ...)         -- 写消息队列
return 0
```

---

### Feed 流与社区

**推模式 + 游标翻页：**

```
发帖时：ZADD feed:{fanId} {timestamp} {blogId}   -- 推送到所有粉丝收件箱

翻页时：ZREVRANGEBYSCORE feed:{userId} {lastId} 0
        WITHSCORES LIMIT {offset} {pageSize}
```

`offset` 字段专门处理同一毫秒内多条帖子的边界情况，避免分页重复或跳过。

**点赞排行榜：**

```
ZADD blog:liked:{blogId} {timestamp} {userId}
ZRANGE blog:liked:{blogId} 0 4   -- Top5 最早点赞
```

score 用时间戳而非计数，支持按"最早点赞"排序。

**关注共同好友：**

```
SINTER follow:{userId1} follow:{userId2}   -- Redis 集合交集
```

Redis 命中时直接返回，miss 时降级为 DB INNER JOIN。

---

### GEO 附近活动

活动数据包含经纬度（从场馆冗余复制），服务启动时按类型分组写入 Redis GEO：

```
GEOADD shop:geo:{typeId} {lng} {lat} {shopId}
GEORADIUS shop:geo:{typeId} {x} {y} 2000000 m
          ASC WITHCOORD WITHDIST COUNT {end}
```

查询结果按距离升序，手动截取当前页并回填 `distance` 字段返回前端。

---

## 关键设计决策

### 秒杀库存启动预热（幂等）

历史秒杀券在服务启动时批量同步到 Redis，同时预热本地时间窗口缓存：

```go
// 幂等：EXISTS 检查，key 存在则跳过，防止覆盖运行时已扣减的值
stockLoader := utils.NewSeckillStockLoader(rdb)
stockLoader.Load(ctx, voucherRepo)
```

若直接 `SET` 覆盖，Redis 重启后会用 DB 旧值覆盖实时扣减后的值，导致超卖。

### 分布式锁原子释放

```lua
-- unlock.lua
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('del', KEYS[1])
end
return 0
```

UUID 作为锁持有者标识，Lua 原子校验后删除，防止进程崩溃重启后误删他人持有的锁。

### Shop 缓存逻辑过期（防击穿）

Shop 详情用逻辑过期而非 TTL，避免热点数据缓存失效瞬间的大量穿透：

```go
// 已过期：返回旧数据（不阻塞请求）+ 异步 goroutine 加锁重建
if isExpired(redisData.ExpireTime) {
    s.rebuildShopCache(id, key)   // SETNX 防重复重建
    return models.OKData(redisData.Data), nil
}
```

### 用户签到 BitMap

```
SETBIT sign:{yyyy:MM:}{userId} {dayOfMonth-1} 1
BITFIELD sign:{yyyy:MM:}{userId} GET u{dayOfMonth} 0
```

按月份一个 key，31 天 = 31 bit ≈ 4 字节，内存极省。从最低位往前数连续 1 的个数即为连续签到天数。

---

## 快速启动

**依赖环境：**
- Go 1.21+
- MySQL 8.0
- Redis Stack（含 BF 模块）

```bash
# 启动 Redis Stack（推荐 Docker）
docker run -d -p 6379:6379 redis/redis-stack-server
```

**1. 初始化数据库**

```bash
mysql -u root -p < qppt_v2_0.sql
```

**2. 配置文件**

```yaml
# config.yaml
server:
  port: 8080
mysql:
  dsn: "root:password@tcp(127.0.0.1:3306)/qppt?parseTime=true&loc=Local"
redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
upload:
  dir: "resources/uploads"
```

**3. 启动**

```bash
go mod tidy
go run main/main.go
```

正常启动日志：

```
✅ MySQL 连接成功
✅ Redis 连接成功
✅ Stream 消费组 g1 创建成功
[GeoLoader] key=shop:geo:1 写入 3 个坐标
[SeckillStockLoader] Redis key=seckill:stock:6 写入 stock=200
[SeckillStockLoader] 本地缓存 voucherID=6 window=[2026-06-10 12:00, 2026-06-30 15:59]
✅ VoucherOrderConsumer 启动，监听 stream.orders
🚀 服务启动，端口: 8080
```

**4. 验证秒杀链路**

```bash
# 发验证码
curl -X POST "http://localhost:8080/user/code?phone=13686869696"

# 登录
curl -X POST http://localhost:8080/user/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13686869696","code":"YOUR_CODE"}'

# 抢票（voucherId=6，库存 200）
curl -X POST http://localhost:8080/voucher-order/seckill/6 \
  -H "authorization: YOUR_TOKEN"

# 验证 Redis 库存扣减
redis-cli GET seckill:stock:6

# 验证消费者 pending 情况
redis-cli XPENDING stream.orders g1 - + 10
```

---

## 压测结果

```bash
# 使用 hey 并发压测
hey -n 1000 -c 200 \
  -m POST \
  -H "authorization: TOKEN" \
  http://localhost:8080/voucher-order/seckill/8
```

> 📌 压测数据待补充（建议用真实数据替换）

**验证超卖防护：**

```sql
-- 压测后：订单数应 ≤ 原始库存 500
SELECT COUNT(*) FROM tb_voucher_order WHERE voucher_id = 8;

-- 验证无重复下单
SELECT user_id, COUNT(*) cnt
FROM tb_voucher_order WHERE voucher_id = 8
GROUP BY user_id HAVING cnt > 1;
```

---

## 目录结构

```
.
├── main/
│   └── main.go                    # 启动入口，依赖注入
├── handlers/
│   ├── middleware.go               # Token 刷新、登录校验
│   ├── rate_limit.go               # 令牌桶限流（按 voucherID 独立桶）
│   ├── user_handler.go
│   ├── shop_handler.go
│   ├── blog_handler.go
│   ├── follow_handler.go
│   └── voucher_handler.go
├── services/
│   ├── user_service.go             # 登录、签到、用户信息
│   ├── shop_service.go             # 商铺缓存（逻辑过期）+ GEO
│   ├── blog_service.go             # 图文帖、点赞 ZSet、Feed 流
│   ├── follow_service.go           # 关注、共同关注（SINTER）
│   ├── voucher_service.go          # 秒杀核心（四层拦截）
│   └── seckill.lua                 # 原子 Lua 脚本
├── repositories/
│   ├── user_repo.go
│   ├── shop_repo.go
│   ├── blog_repo.go
│   ├── follow_repo.go
│   └── voucher_repo.go
├── models/                         # 数据实体 + DTO + 统一响应体
├── mq/
│   └── voucher_order_consumer.go   # Stream 消费者（重试 + 死信）
├── utils/
│   ├── id_worker.go                # 全局唯一 ID
│   ├── redis_lock.go               # 分布式锁（SETNX + Lua 释放）
│   ├── local_cache.go              # 本地缓存（售罄标记 + 时间窗口）
│   ├── bloom_filter.go             # 布隆过滤器（Redis BF 模块）
│   ├── geo_loader.go               # 启动时 GEO 数据预热
│   └── seckill_stock_loader.go     # 启动时秒杀库存预热（幂等）
└── qppt_v2_0.sql                   # 完整数据库初始化脚本
```
