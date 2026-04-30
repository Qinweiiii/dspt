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
	maxPageSize  = 5           // 对应 Java SystemConstants.MAX_PAGE_SIZE
	geoRadius    = 5000.0      // 5km，单位米
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

	// 1. 查 Redis
	cacheData, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		if cacheData == "" { // 缓存空值（防穿透）
			return models.Fail("商铺不存在"), nil
		}
		var shop models.Shop
		json.Unmarshal([]byte(cacheData), &shop)
		return models.OKData(shop), nil
	} else if err != redis.Nil {
		return models.Fail("系统异常"), err
	}

	// 2. Redis 未命中，查 DB
	shop, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return models.Fail("系统异常"), err
	}

	// 3. DB 不存在，缓存空值（防穿透）
	if shop == nil {
		s.rdb.Set(ctx, key, "", 2*time.Minute)
		return models.Fail("商铺不存在"), nil
	}

	// 4. DB 存在，写入缓存
	shopBytes, _ := json.Marshal(shop)
	s.rdb.Set(ctx, key, string(shopBytes), 30*time.Minute)

	return models.OKData(shop), nil
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
	// Redis GEOSEARCH 不直接支持 offset，需取 [0, end) 再手动截取
	// 对应 Java: opsForGeo().search(...).limit(end)
	end := current * maxPageSize
	key := ShopGeoKey + strconv.Itoa(typeID)

	results, err := s.rdb.GeoSearchLocation(ctx, key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  *x,
			Latitude:   *y,
			Radius:     geoRadius,
			RadiusUnit: "m",
			Sort:       "ASC",
			Count:      end, // 取到当前页末尾，再手动 skip from
		},
		WithDist: true,
	}).Result()
	if err != nil {
		return models.Fail("系统异常"), fmt.Errorf("GeoSearchLocation: %w", err)
	}
	if len(results) == 0 {
		return models.OKData([]*models.Shop{}), nil
	}

	// 没有下一页
	if int64(len(results)) <= int64(offset) {
		return models.OKData([]*models.Shop{}), nil
	}

	// 截取当前页，收集 id 和距离
	// 对应 Java: content.stream().skip(from).forEach(...)
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
