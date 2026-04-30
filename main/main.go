package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"go.yaml.in/yaml/v3"

	"github.com/qinweiiii/dspt/handlers"
	"github.com/qinweiiii/dspt/mq"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/qinweiiii/dspt/services"
	"github.com/qinweiiii/dspt/utils"
)

// ========================================
// 配置结构体
// ========================================

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	MySQL struct {
		DSN string `yaml:"dsn"`
	} `yaml:"mysql"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
}

func main() {
	// 1. 加载配置
	cfg := loadConfig("config.yaml")

	// 2. 初始化 MySQL
	db := initMySQL(cfg)
	defer db.Close()

	// 3. 初始化 Redis
	rdb := initRedis(cfg)
	defer rdb.Close()

	// 4. 初始化 Redis Stream 消费组
	initStreamGroup(rdb)

	// ========================================
	// 依赖注入
	// Repository → Service → Handler 逐层注入
	// ========================================

	// 用户模块
	userRepo := repositories.NewUserRepository(db)
	userSvc := services.NewUserService(userRepo, rdb)
	userHandler := handlers.NewUserHandler(userSvc)

	// 商铺模块
	shopRepo := repositories.NewShopRepository(db)
	shopSvc := services.NewShopService(shopRepo, rdb)
	shopHandler := handlers.NewShopHandler(shopSvc)

	// 博客模块
	// followRepo 同时被 blogSvc （推 Feed）和 followSvc 使用，共享同一个实例
	blogRepo := repositories.NewBlogRepository(db)
	followRepo := repositories.NewFollowRepository(db)
	blogSvc := services.NewBlogService(blogRepo, userRepo, followRepo, rdb)
	blogHandler := handlers.NewBlogHandler(blogSvc)

	// 关注模块
	followSvc := services.NewFollowService(followRepo, userRepo, rdb)
	followHandler := handlers.NewFollowHandler(followSvc)

	// 共享基础设施
	idWorker := utils.NewIDWorker(rdb)
	locker := utils.NewRedisLocker(rdb)

	// 秒杀模块
	voucherRepo := repositories.NewVoucherRepository(db)
	voucherSvc := services.NewVoucherService(voucherRepo, rdb, idWorker, locker)
	voucherHandler := handlers.NewVoucherHandler(voucherSvc)
	voucherOrderHandler := handlers.NewVoucherOrderHandler(voucherSvc)

	uploadHandler := handlers.NewUploadHandler()

	geoLoader := utils.NewShopGeoLoader(rdb)
	if err := geoLoader.Load(context.Background(), shopRepo); err != nil {
		log.Printf("⚠️  GEO 数据导入失败（不影响启动）: %v", err)
	}

	// 5. 创建 Gin 路由
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := setupRouter(rdb, userHandler, shopHandler,
		blogHandler, followHandler, voucherHandler, voucherOrderHandler, uploadHandler)

	// 6. 启动秒杀订单异步消费者（Phase 5 实现后取消注释）
	consumer := mq.NewVoucherOrderConsumer(rdb, voucherSvc, locker)
	go consumer.Start(ctx) // ctx 取消时消费者自动退出

	// 7. 启动 HTTP 服务器（支持优雅关机）
	startServerWithCtx(r, cfg.Server.Port, cancel)
}

// ========================================
// 路由注册（统一管理所有模块路由）
// ========================================

func setupRouter(
	rdb *redis.Client,
	userH *handlers.UserHandler,
	shopH *handlers.ShopHandler,
	blogH *handlers.BlogHandler,
	followH *handlers.FollowHandler,
	voucherH *handlers.VoucherHandler,
	voucherOrderH *handlers.VoucherOrderHandler,
	uploadH *handlers.UploadHandler,
) *gin.Engine {

	r := gin.Default()

	// 全局中间件：Token 刷新（order=0，所有路由都经过）
	r.Use(handlers.RefreshTokenMiddleware(rdb))

	// 健康检查
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong", "time": time.Now().Format(time.RFC3339)})
	})

	// 登录校验中间件（order=1，受保护路由使用）
	authRequired := handlers.LoginRequiredMiddleware()

	// 各模块路由注册
	userH.RegisterRoutes(r, authRequired)
	shopH.RegisterRoutes(r, authRequired)
	blogH.RegisterRoutes(r, authRequired)
	followH.RegisterRoutes(r, authRequired)
	voucherH.RegisterRoutes(r, authRequired)
	voucherOrderH.RegisterRoutes(r, authRequired)
	uploadH.RegisterRoutes(r)

	return r
}

// ========================================
// 基础设施初始化（与 Phase 1 相同）
// ========================================

func loadConfig(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("打开配置文件失败：%v", err)
	}
	defer f.Close()

	var c Config
	if err = yaml.NewDecoder(f).Decode(&c); err != nil {
		log.Fatalf("解析配置文件失败：%v", err)
	}
	log.Printf("✅ 配置加载成功 | 端口: %d | Redis: %s", c.Server.Port, c.Redis.Addr)
	return &c
}

func initMySQL(c *Config) *sql.DB {
	database, err := sql.Open("mysql", c.MySQL.DSN)
	if err != nil {
		log.Fatalf("MySQL 连接失败：%v", err)
	}
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(10)
	database.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = database.PingContext(ctx); err != nil {
		log.Fatalf("MySQL Ping 失败：%v", err)
	}
	log.Printf("✅ MySQL 连接成功 | DSN: %s", maskDSN(c.MySQL.DSN))
	return database
}

func initRedis(c *Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           c.Redis.DB,
		PoolSize:     10,
		MinIdleConns: 1,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis 连接失败：%v", err)
	}
	log.Printf("✅ Redis 连接成功 | Addr: %s", c.Redis.Addr)
	return client
}

func initStreamGroup(client *redis.Client) {
	ctx := context.Background()
	// stream.orders 是 传order的stream
	// g1 是 消费组
	// $ 是 从最新消息开始消费（如果是 0 则从头开始消费）
	// XGroupCreateMkStream 等同于先 XGROUP CREATE 再 XGROUP SETID $，如果 Stream 不存在会自动创建
	err := client.XGroupCreateMkStream(ctx, "stream.orders", "g1", "$").Err()
	if err != nil {
		if strings.Contains(err.Error(), "BUSYGROUP") {
			log.Println("✅ Stream 消费组 g1 已存在，跳过创建")
			return
		}
		log.Fatalf("创建 Stream 消费组失败: %v", err)
	}
	log.Println("✅ Redis Stream 消费组 g1 创建成功")
}

func startServerWithCtx(r *gin.Engine, port int, cancel context.CancelFunc) {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: r}
	go func() {
		log.Printf("🚀 服务启动，端口: %d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel() // 先通知所有 goroutine 退出

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)
	log.Println("✅ 服务已安全退出")
}

func maskDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at == -1 {
		return dsn
	}
	colon := strings.Index(dsn[:at], ":")
	if colon == -1 {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}
