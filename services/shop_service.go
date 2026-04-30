package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

const (
	CacheShopKey = "cache:shop:"
	CacheTypeKey = "cache:type"
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
