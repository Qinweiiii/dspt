package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

const (
	CacheShopKey = "cache:shop:"
	CacheTypeKey = "cache:type"
	ShopGeoKey   = "shop:geo:" // shop:geo:{typeId}
	LockShopKey  = "lock:shop:"

	CacheShopTTL        = 30 * time.Minute
	CacheNullTTL        = 2 * time.Minute
	CacheShopLogicalTTL = 30 * time.Minute
	LockShopTTL         = 10 * time.Second

	maxPageSize = 10        // 对应 Java SystemConstants.MAX_PAGE_SIZE
	geoRadius   = 2000000.0 // 2000km，单位米
)

type ShopService struct {
	repo *repositories.ShopRepository
	rdb  *redis.Client
}

func NewShopService(repo *repositories.ShopRepository, rdb *redis.Client) *ShopService {
	return &ShopService{repo: repo, rdb: rdb}
}

// ----------------------------------------
// 商铺相关业务
// ----------------------------------------

func (s *ShopService) QueryByID(ctx context.Context, id int64) (models.Result, error) {
	key := fmt.Sprintf("%s%d", CacheShopKey, id)

	cacheData, err := s.rdb.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return models.Fail("系统异常"), err
	}
	if err == redis.Nil {
		return s.queryShopFromDB(ctx, key, id)
	}
	if cacheData == "" {
		return models.Fail("商铺不存在"), nil
	}

	var redisData models.RedisData
	if err := json.Unmarshal([]byte(cacheData), &redisData); err != nil {
		var shop models.Shop
		if err := json.Unmarshal([]byte(cacheData), &shop); err != nil {
			return models.Fail("系统异常"), err
		}
		return models.OKData(shop), nil
	}

	if !isExpired(redisData.ExpireTime) {
		return models.OKData(redisData.Data), nil
	}

	s.rebuildShopCache(id, key)
	return models.OKData(redisData.Data), nil
}

func (s *ShopService) Save(ctx context.Context, shop *models.Shop) (models.Result, error) {
	id, err := s.repo.Save(ctx, shop)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	shop.ID = id
	return models.OKData(id), nil
}

func (s *ShopService) Update(ctx context.Context, shop *models.Shop) (models.Result, error) {
	if shop.ID == 0 {
		return models.Fail("商铺ID不能为空"), nil
	}

	// 1. 更新数据库
	if err := s.repo.Update(ctx, shop); err != nil {
		return models.Fail("系统异常"), err
	}

	// 2. 删除缓存
	s.rdb.Del(ctx, fmt.Sprintf("%s%d", CacheShopKey, shop.ID))

	return models.OK(), nil
}

// ----------------------------------------
// 商铺类型业务
// ----------------------------------------

func (s *ShopService) GetTypeList(ctx context.Context) (models.Result, error) {
	// 1. 查 Redis List
	listStrs, err := s.rdb.LRange(ctx, CacheTypeKey, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return models.Fail("系统异常"), err
	}
	if len(listStrs) > 0 {
		var types []models.ShopType
		for _, str := range listStrs {
			var t models.ShopType
			json.Unmarshal([]byte(str), &t)
			types = append(types, t)
		}
		return models.OKData(types), nil
	}

	// 2. 查 DB
	types, err := s.repo.FindAllShopTypes(ctx)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if len(types) == 0 {
		return models.Fail("没有分类数据"), nil
	}

	// 3. 写入 Redis List
	var jsonStrs []interface{}
	for _, t := range types {
		bytes, _ := json.Marshal(t)
		jsonStrs = append(jsonStrs, string(bytes))
	}
	s.rdb.RPush(ctx, CacheTypeKey, jsonStrs...)

	return models.OKData(types), nil
}

// QueryShopByType 按商铺类型查询，支持 GEO 附近排序
//
// x/y 为 nil → 纯 DB 分页（按默认排序）
// x/y 有值  → Redis GEO 按距离排序 + 分页，并回填距离字段
//
// 对应 Java: ShopServiceImpl.queryShopByType()
func (s *ShopService) QueryShopByType(ctx context.Context, typeID, current int, x, y *float64) (models.Result, error) {
	offset := (current - 1) * maxPageSize

	// ── 无坐标：普通分页 ──────────────────────────────────────────
	if x == nil || y == nil {
		shops, err := s.repo.FindByTypeID(ctx, typeID, offset, maxPageSize)
		if err != nil {
			return models.Fail("系统异常"), err
		}
		return models.OKData(shops), nil
	}

	// ── 有坐标：GEO 距离排序 ──────────────────────────────────────
	// 用 GeoRadius（Redis 3.2+ 支持），兼容 Redis 5.x
	// GEOSEARCH 是 Redis 6.2 才引入的，这里不能用
	// GeoRadius 同样不支持 offset，取 [0, end) 后手动截取
	end := current * maxPageSize
	key := ShopGeoKey + strconv.Itoa(typeID)

	exists, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if exists == 0 {
		shops, err := s.repo.FindByTypeID(ctx, typeID, offset, maxPageSize)
		if err != nil {
			return models.Fail("系统异常"), err
		}
		return models.OKData(shops), nil
	}

	results, err := s.rdb.GeoRadius(ctx, key, *x, *y, &redis.GeoRadiusQuery{
		Radius:   geoRadius,
		Unit:     "m",
		WithDist: true,
		Sort:     "ASC",
		Count:    end,
	}).Result()
	if err != nil {
		return models.Fail("系统异常"), fmt.Errorf("GeoRadius: %w", err)
	}
	if len(results) == 0 {
		return models.OKData([]*models.Shop{}), nil
	}

	// 没有下一页（修复：offset >= 实际结果数才是真正没数据）
	if offset >= len(results) {
		return models.OKData([]*models.Shop{}), nil
	}

	// 截取当前页，收集 id 和距离
	page := results[offset:]
	ids := make([]int64, 0, len(page))
	distMap := make(map[int64]float64, len(page))

	for _, r := range page {
		id, err := strconv.ParseInt(r.Name, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		distMap[id] = r.Dist // 单位：米
	}

	// 按 GEO 返回的顺序查 DB（FIELD 保序）
	shops, err := s.repo.FindByIDsOrdered(ctx, ids)
	if err != nil {
		return models.Fail("系统异常"), err
	}

	// 回填距离字段
	for _, shop := range shops {
		shop.Distance = distMap[shop.ID]
	}

	return models.OKData(shops), nil
}

// QueryShopByName 按名称关键字模糊查询分页
// 对应 Java: ShopController.queryShopByName()
func (s *ShopService) QueryShopByName(ctx context.Context, name string, current int) (models.Result, error) {
	offset := (current - 1) * maxPageSize
	shops, err := s.repo.FindByName(ctx, name, offset, maxPageSize)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(shops), nil
}

func (s *ShopService) queryShopFromDB(ctx context.Context, key string, id int64) (models.Result, error) {
	shop, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if shop == nil {
		s.rdb.Set(ctx, key, "", CacheNullTTL)
		return models.Fail("商铺不存在"), nil
	}

	if err := s.setShopCache(ctx, key, shop); err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(shop), nil
}

func (s *ShopService) setShopCache(ctx context.Context, key string, shop *models.Shop) error {
	data := models.RedisData{
		ExpireTime: time.Now().Add(CacheShopLogicalTTL).Format(time.RFC3339Nano),
		Data:       *shop,
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, bytes, 0).Err()
}

func (s *ShopService) rebuildShopCache(id int64, key string) {
	lockKey := LockShopKey + strconv.FormatInt(id, 10)
	if !s.tryLock(context.Background(), lockKey) {
		return
	}

	go func() {
		defer s.unlock(context.Background(), lockKey)

		shop, err := s.repo.FindByID(context.Background(), id)
		if err != nil || shop == nil {
			return
		}
		_ = s.setShopCache(context.Background(), key, shop)
	}()
}

func (s *ShopService) tryLock(ctx context.Context, key string) bool {
	ok, err := s.rdb.SetNX(ctx, key, "1", LockShopTTL).Result()
	return err == nil && ok
}

func (s *ShopService) unlock(ctx context.Context, key string) {
	s.rdb.Del(ctx, key)
}

func isExpired(expireAt string) bool {
	if expireAt == "" {
		return true
	}
	expireTime, err := time.Parse(time.RFC3339Nano, expireAt)
	if err != nil {
		return true
	}
	return expireTime.Before(time.Now())
}
