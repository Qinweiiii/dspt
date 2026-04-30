package utils

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/qinweiiii/dspt/models"
	"github.com/redis/go-redis/v9"
)

// ShopGeoLoader 负责把 DB 里的商铺坐标导入 Redis GEO
//
// Redis Key：shop:geo:{typeId}
// Member：shopId（字符串）
// Score：经纬度（Redis GEO 内部用 geohash 存储）
//
// 调用时机：服务启动时检查，Key 不存在则导入，存在则跳过。
// 对应 Java 项目里用单元测试手动导入的逻辑。
type ShopGeoLoader struct {
	rdb *redis.Client
}

func NewShopGeoLoader(rdb *redis.Client) *ShopGeoLoader {
	return &ShopGeoLoader{rdb: rdb}
}

// ShopProvider 供 GeoLoader 查询所有商铺（避免循环依赖，用接口隔离）
type ShopProvider interface {
	FindAllWithGeo(ctx context.Context) ([]*models.Shop, error)
}

// Load 按 typeId 分组，批量写入 Redis GEO
// 幂等：已存在的 Key 跳过，不重复写入
func (l *ShopGeoLoader) Load(ctx context.Context, provider ShopProvider) error {
	shops, err := provider.FindAllWithGeo(ctx)
	if err != nil {
		return fmt.Errorf("GeoLoader.Load query: %w", err)
	}
	if len(shops) == 0 {
		log.Println("[GeoLoader] 没有需要导入的商铺坐标")
		return nil
	}

	// 按 typeId 分组
	byType := make(map[int64][]*models.Shop)
	for _, s := range shops {
		byType[s.TypeID] = append(byType[s.TypeID], s)
	}

	for typeID, group := range byType {
		key := "shop:geo:" + strconv.FormatInt(typeID, 10)

		// 已存在则跳过（幂等）
		exists, err := l.rdb.Exists(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("GeoLoader check key %s: %w", key, err)
		}
		if exists > 0 {
			log.Printf("[GeoLoader] key=%s 已存在，跳过", key)
			continue
		}

		// 构造 GeoLocation 批量写入
		locations := make([]*redis.GeoLocation, 0, len(group))
		for _, s := range group {
			locations = append(locations, &redis.GeoLocation{
				Name:      strconv.FormatInt(s.ID, 10),
				Longitude: s.X,
				Latitude:  s.Y,
			})
		}

		added, err := l.rdb.GeoAdd(ctx, key, locations...).Result()
		if err != nil {
			return fmt.Errorf("GeoLoader GeoAdd key=%s: %w", key, err)
		}
		log.Printf("[GeoLoader] key=%s 写入 %d 个坐标", key, added)
	}

	return nil
}
