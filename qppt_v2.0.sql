/*
 ============================================================
 项目改造：黑马点评 → 「有票」线下活动抢票平台
 版本：v3 — 新增 tb_venue（场馆表），tb_shop 改为活动表
 ============================================================

 数据模型：
   tb_venue     场馆（物理地点，14条，从原 tb_shop 迁移）
   tb_shop      活动（每场演出/展览/脱口秀，venue_id关联场馆）
   tb_voucher   票种（shop_id 关联活动）
   tb_blog      图文帖（shop_id 关联活动）

 对现有代码的影响：
   ✅ tb_shop 保留全部原有字段（id/name/type_id/images/area/address/
      x/y/avg_price/sold/comments/score/open_hours/create_time/update_time）
   ✅ x/y 从对应 venue 冗余复制，ShopGeoLoader 零改动
   ✅ 新增 venue_id 字段，现有代码不读此字段，不影响任何查询
   ✅ tb_venue 是全新表，现有代码完全感知不到
   ✅ tb_voucher.shop_id 指向活动id（tb_shop.id），逻辑不变
   ✅ tb_blog.shop_id 指向活动id（tb_shop.id），逻辑不变
   → 结论：现有全部 Go 代码零改动

 图片路径（完全沿用你现有文件中已有的路径）：
   tb_shop_type.icon  → /types/concert.png 等（已有）
   tb_user.icon       → /imgs/icons/icon_*.jpg（已有）
   tb_blog.images     → /imgs/blogs/xxx/xxx.jpg（已有）
   tb_shop.images     → 外链URL（已有，活动海报沿用场馆图）
   tb_venue.images    → 同上外链URL（从原 tb_shop 迁移）
 ============================================================
*/

SET NAMES utf8mb4;
/*!40014 SET FOREIGN_KEY_CHECKS=0 */;

-- ============================================================
-- tb_shop_type  活动类型（数据完全沿用你现有的）
-- ============================================================
DROP TABLE IF EXISTS `tb_shop_type`;
CREATE TABLE `tb_shop_type` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`        varchar(32)  CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '活动类型名称',
  `icon`        varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '类型图标',
  `sort`        int(3) UNSIGNED NULL DEFAULT NULL COMMENT '排序权重',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 11
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- 完全沿用你现有数据，图标路径不变
INSERT INTO `tb_shop_type` VALUES ( 1, '演唱会',      '/types/concert.png',    1,  '2021-12-22 12:17:47', '2026-05-02 06:04:23');
INSERT INTO `tb_shop_type` VALUES ( 2, '展览',         '/types/exhibition.png', 2,  '2021-12-22 12:18:27', '2026-05-02 06:04:30');
INSERT INTO `tb_shop_type` VALUES ( 3, '脱口秀',       '/types/talkshow.png',   3,  '2021-12-22 12:18:48', '2026-05-02 06:04:37');
INSERT INTO `tb_shop_type` VALUES ( 4, '体育赛事',     '/types/sports.png',     4,  '2021-12-22 12:19:04', '2026-05-02 06:04:46');
INSERT INTO `tb_shop_type` VALUES ( 5, '话剧·戏剧',  '/types/opera.png',      5,  '2021-12-22 12:19:27', '2026-05-02 06:07:14');
INSERT INTO `tb_shop_type` VALUES ( 6, '音乐节',       '/types/live.png',       6,  '2021-12-22 12:19:35', '2026-05-02 06:07:44');
INSERT INTO `tb_shop_type` VALUES ( 7, '亲子活动',     '/types/family.png',     7,  '2021-12-22 12:19:53', '2026-05-02 06:29:42');
INSERT INTO `tb_shop_type` VALUES ( 8, '集市·夜市',  '/types/cock.png',       8,  '2021-12-22 12:20:02', '2026-05-02 06:29:28');
INSERT INTO `tb_shop_type` VALUES ( 9, '见面会·签售','/types/vinyl.png',      9,  '2021-12-22 12:20:08', '2026-05-02 06:11:27');
INSERT INTO `tb_shop_type` VALUES (10, '工作坊·课程','/types/workshop.png',   10, '2021-12-22 12:21:46', '2026-05-02 06:07:38');


-- ============================================================
-- tb_venue  场馆表（新增）
-- 从原 tb_shop 的 14 条数据迁移而来
-- 现有 Go 代码不读此表，纯数据层存在，不影响任何逻辑
-- ============================================================
DROP TABLE IF EXISTS `tb_venue`;
CREATE TABLE `tb_venue` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name`        varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '场馆名称',
  `type_id`     bigint(20) UNSIGNED NOT NULL COMMENT '场馆主营活动类型',
  `images`      varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '场馆图片',
  `area`        varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '所在区域',
  `address`     varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '详细地址',
  `x`           double UNSIGNED NOT NULL COMMENT '经度',
  `y`           double UNSIGNED NOT NULL COMMENT '纬度',
  `capacity`    int(10) UNSIGNED NULL DEFAULT NULL COMMENT '场馆最大容量（人）',
  `open_hours`  varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '开放时间',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 15
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- 从原 tb_shop 14条数据原样迁移，id 保持一致方便 venue_id 对照
INSERT INTO `tb_venue` VALUES ( 1, '上海梅赛德斯-奔驰文化中心', 1, 'https://i.redd.it/8h9saw6lqi2b1.jpg',                                                                                                                              '浦东新区', '浦东新区博成路1200号',               121.506377, 31.239237, 18000, '根据演出安排',       '2021-12-22 10:10:39', '2026-05-02 03:32:50');
INSERT INTO `tb_venue` VALUES ( 2, '北京工人体育场',             1, 'https://img.jzda001.com/image/591675939895314.jpg',                                                                                                               '朝阳区',   '朝阳区工人体育场北路2号',             116.453745, 39.930294, 66000, '根据演出安排',       '2021-12-22 11:00:13', '2026-05-02 03:36:51');
INSERT INTO `tb_venue` VALUES ( 3, '广州天河体育中心',           1, 'https://img.dahepiao.com/uploads/200220/200220/148061-200220215543U4.jpg',                                                                                       '天河区',   '天河区天河路299号',                   113.330405, 23.133487, 58000, '根据演出安排',       '2021-12-22 11:10:05', '2026-05-02 03:39:23');
INSERT INTO `tb_venue` VALUES ( 4, '成都凤凰山体育公园',         1, 'https://pic.cyol.com/img/20231030/img_9630d9b9ed67631c1eeb79bb63346d3f9f_c.jpg',                                                                                '金牛区',   '金牛区凤凰山体育路1号',               104.054108, 30.714592, 60000, '根据演出安排',       '2021-12-22 11:17:15', '2026-05-02 03:40:15');
INSERT INTO `tb_venue` VALUES ( 5, '上海当代艺术博物馆',         2, 'https://icity-static.icitycdn.com/images/uploads/ap/imsm/museum/pic_head/hmo59bc/8a67bd7e479331d1hmo59bc.jpg',                                                '黄浦区',   '黄浦区花园港路200号',                 121.480491, 31.236136, 3000,  '11:00-19:00（周二休馆）', '2021-12-22 11:20:58', '2026-05-02 03:40:55');
INSERT INTO `tb_venue` VALUES ( 6, '北京798艺术区',              2, 'https://thumbs.dreamstime.com/b/798-72865898.jpg',                                                                                                                '朝阳区',   '朝阳区酒仙桥路4号',                   116.497473, 39.984556, 5000,  '10:00-18:00',        '2021-12-22 11:24:53', '2026-05-02 03:42:37');
INSERT INTO `tb_venue` VALUES ( 7, '笑果文化·上海演艺新空间',   3, 'https://n.sinaimg.cn/spider20230420/74/w500h374/20230420/aad3-b9459b66d4319d890f9c9afaa7c41d14.png',                                                           '黄浦区',   '黄浦区西藏南路666号',                 121.487252, 31.225316, 300,   '19:30场/20:00场',    '2021-12-22 11:40:52', '2026-05-02 03:43:41');
INSERT INTO `tb_venue` VALUES ( 8, '单立人喜剧·北京剧场',       3, 'https://pic.cyol.com/img/20211207/img_9601bb7559eee03f7ace66e7132e926680e2.jpeg',                                                                               '朝阳区',   '朝阳区朝外大街甲6号万通中心C座',     116.449921, 39.921509, 200,   '19:30场',            '2021-12-22 11:51:06', '2026-05-02 03:44:56');
INSERT INTO `tb_venue` VALUES ( 9, '上海浦东足球场',             4, 'https://wenhui.whb.cn/u/cms/www/201804/28124854ys23.jpg',                                                                                                        '浦东新区', '浦东新区杨高南路3999号',              121.570648, 31.178022, 35000, '根据赛事安排',       '2021-12-22 11:53:59', '2026-05-02 03:46:04');
INSERT INTO `tb_venue` VALUES (10, '草莓音乐节·上海主会场',     6, 'https://img.jiemian.com/101/original/20190428/155644067780805400.jpg',                                                                                           '嘉定区',   '嘉定区安亭镇墨玉南路777号',           121.197842, 31.289651, 30000, '10:00-22:00（节日期间）', '2021-12-22 12:25:16', '2026-05-02 03:47:12');
INSERT INTO `tb_venue` VALUES (11, '国家大剧院',                 5, 'https://thumbs.dreamstime.com/b/%E5%9B%BD%E5%AE%B6%E5%A4%A7%E5%89%A7%E9%99%A2%EF%BC%8C%E5%8C%97%E4%BA%AC-%E4%B8%AD%E5%9B%BD-73713497.jpg',                  '西城区',   '西城区西长安街2号',                   116.389550, 39.903816, 5452,  '根据演出安排',       '2021-12-22 12:29:02', '2026-05-02 03:48:55');
INSERT INTO `tb_venue` VALUES (12, '上海国际会议中心',           9, 'https://qimgs.qunarzz.com/mp_ctrip_gateway_0001/0101m120009d0uu5n7B90_W_10000_1200.jpg',                                                                        '浦东新区', '浦东新区陆家嘴环路2727号',            121.512914, 31.235256, 2000,  '根据活动安排',       '2021-12-22 12:34:34', '2026-05-02 03:49:31');
INSERT INTO `tb_venue` VALUES (13, '上海迪士尼小镇剧场',         7, 'https://static.shanghaidisneyresort.com/tridion/prod/zh-cn/system/images/shdr-ent-mickey-storybook-adventure-video-cover_tcm1874-242099.jpg',                 '浦东新区', '浦东新区川沙新镇南六公路310号',       121.666884, 31.148858, 800,   '10:00-20:00',        '2021-12-22 12:38:54', '2026-05-02 03:50:36');
INSERT INTO `tb_venue` VALUES (14, '造作艺术空间·北京',         10, 'https://images.adsttc.com/media/images/63bb/faf8/f7e2/c001/7079/3ca3/newsletter/not-ready-flop-art-space-achipoetry-studio_1.jpg',                            '朝阳区',   '朝阳区三里屯太古里南区B1-02',         116.454033, 39.937563, 80,    '10:00-21:00',        '2021-12-22 12:48:54', '2026-05-02 03:51:15');


-- ============================================================
-- tb_shop  活动表
--
-- ⚠️  字段说明（对比原 tb_shop，唯一变化是新增 venue_id）：
--   name       → 活动全称
--   type_id    → 活动类型（与 tb_shop_type 关联，不变）
--   images     → 活动宣传海报（沿用 venue 的图片URL，可后续单独替换）
--   area       → 举办城市/区域（冗余自 venue，方便列表展示）
--   address    → 举办地点全称（"场馆名 + 地址"，冗余自 venue）
--   x / y      → 举办地经纬度（冗余自 venue，GeoLoader 直接读，零改动）
--   avg_price  → 最低票价（元，列表页"低至xxx元"）
--   sold       → 累计售票量
--   comments   → 相关帖子/评价数
--   score      → 活动评分（观演后评分）
--   open_hours → 演出时间（如"2024-06-15 19:30"）
--   venue_id   → 🆕 关联 tb_venue.id（现有代码不读，不影响任何逻辑）
--
-- 活动分布：
--   梅奔(venue=1)   → 3场演唱会（Taylor Swift / 五月天 / 薛之谦）
--   工体(venue=2)   → 2场演唱会（周杰伦北京站 / Coldplay）
--   天河(venue=3)   → 1场演唱会（林俊杰）
--   凤凰山(venue=4) → 2场（周杰伦成都站 / 草莓音乐节成都）
--   PSA(venue=5)    → 2场展览（teamLab / 村上隆）
--   798(venue=6)    → 1场展览（KAWS）
--   笑果(venue=7)   → 3场脱口秀（Norah专场 / 王勉专场 / 开放麦）
--   单立人(venue=8) → 1场脱口秀（Rock专场）
--   浦东足球场(venue=9) → 1场体育赛事（申花主场）
--   草莓上海(venue=10)  → 1场音乐节
--   国家大剧院(venue=11)→ 2场话剧（悲惨世界 / 猫）
--   国际会议中心(venue=12) → 1场见面会
--   迪士尼剧场(venue=13)   → 1场亲子活动
--   造作空间(venue=14)     → 1场工作坊
-- ============================================================
DROP TABLE IF EXISTS `tb_shop`;
CREATE TABLE `tb_shop` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name`        varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '活动全称',
  `type_id`     bigint(20) UNSIGNED NOT NULL COMMENT '活动类型id',
  `images`      varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '活动宣传图',
  `area`        varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '举办城市/区域',
  `address`     varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '举办地点（场馆名+地址）',
  `x`           double UNSIGNED NOT NULL COMMENT '经度（冗余自venue，GeoLoader用）',
  `y`           double UNSIGNED NOT NULL COMMENT '纬度（冗余自venue，GeoLoader用）',
  `avg_price`   bigint(10) UNSIGNED NULL DEFAULT NULL COMMENT '最低票价（元）',
  `sold`        int(10) UNSIGNED ZEROFILL NOT NULL COMMENT '累计售票量',
  `comments`    int(10) UNSIGNED ZEROFILL NOT NULL COMMENT '评价/帖子数',
  `score`       int(2) UNSIGNED ZEROFILL NOT NULL COMMENT '评分（1~5分乘10）',
  `open_hours`  varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '演出时间',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `venue_id`    bigint(20) UNSIGNED NOT NULL COMMENT '关联场馆id（tb_venue.id）',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_type_id`(`type_id`) USING BTREE,
  INDEX `idx_venue_id`(`venue_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 25
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- ----------------------------------------------------------
-- venue_id=1  上海梅赛德斯-奔驰文化中心  3场演唱会
-- x/y 直接从 venue 1 冗余：121.506377, 31.239237
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  1, 'Taylor Swift | The Eras Tour·上海站', 1,
  'https://i.redd.it/8h9saw6lqi2b1.jpg',
  '浦东新区', '上海梅赛德斯-奔驰文化中心 · 浦东新区博成路1200号',
  121.506377, 31.239237,
  580, 0000128000, 0000045200, 49,
  '2024-05-18 19:30 / 2024-05-19 19:30',
  '2021-12-22 10:10:39', '2026-05-02 03:32:50', 1
);

INSERT INTO `tb_shop` VALUES (
  2, '五月天「诺亚方舟」世界巡回演唱会·上海站', 1,
  'https://i.redd.it/8h9saw6lqi2b1.jpg',
  '浦东新区', '上海梅赛德斯-奔驰文化中心 · 浦东新区博成路1200号',
  121.506377, 31.239237,
  480, 0000096000, 0000038400, 48,
  '2024-07-20 19:30',
  '2021-12-22 10:10:39', '2026-05-02 03:32:50', 1
);

INSERT INTO `tb_shop` VALUES (
  3, '薛之谦「天外来物」巡回演唱会·上海站', 1,
  'https://i.redd.it/8h9saw6lqi2b1.jpg',
  '浦东新区', '上海梅赛德斯-奔驰文化中心 · 浦东新区博成路1200号',
  121.506377, 31.239237,
  380, 0000071000, 0000029100, 47,
  '2024-08-10 19:30',
  '2021-12-22 10:10:39', '2026-05-02 03:32:50', 1
);

-- ----------------------------------------------------------
-- venue_id=2  北京工人体育场  2场演唱会
-- x/y: 116.453745, 39.930294
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  4, '周杰伦「嘉年华」世界巡回演唱会·北京站', 1,
  'https://img.jzda001.com/image/591675939895314.jpg',
  '朝阳区', '北京工人体育场 · 朝阳区工人体育场北路2号',
  116.453745, 39.930294,
  420, 0000095000, 0000031000, 49,
  '2024-09-14 19:30 / 2024-09-15 19:30',
  '2021-12-22 11:00:13', '2026-05-02 03:36:51', 2
);

INSERT INTO `tb_shop` VALUES (
  5, 'Coldplay Music Of The Spheres World Tour·北京站', 1,
  'https://img.jzda001.com/image/591675939895314.jpg',
  '朝阳区', '北京工人体育场 · 朝阳区工人体育场北路2号',
  116.453745, 39.930294,
  650, 0000108000, 0000052000, 49,
  '2024-10-05 19:00',
  '2021-12-22 11:00:13', '2026-05-02 03:36:51', 2
);

-- ----------------------------------------------------------
-- venue_id=3  广州天河体育中心  1场演唱会
-- x/y: 113.330405, 23.133487
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  6, '林俊杰「圣所」世界巡回演唱会·广州站', 1,
  'https://img.dahepiao.com/uploads/200220/200220/148061-200220215543U4.jpg',
  '天河区', '广州天河体育中心 · 天河区天河路299号',
  113.330405, 23.133487,
  380, 0000087000, 0000028500, 47,
  '2024-11-02 19:30',
  '2021-12-22 11:10:05', '2026-05-02 03:39:23', 3
);

-- ----------------------------------------------------------
-- venue_id=4  成都凤凰山体育公园  2场
-- x/y: 104.054108, 30.714592
-- blog_id=4 对应此活动，shop_id=4 → 活动id=4（保持一致）
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  -- ⚠️ 此条 id=4 与 tb_blog (id=4, shop_id=4) 对应，保持 id=4
  -- （原 blog 里写的就是在凤凰山看周杰伦）
  -- 但 AUTO_INCREMENT 已从1开始，此处用具体值插入保证对齐
  -- 实际 id 由前面插入顺序决定，此为第7条，id=7
  -- 见下方专门说明块
  7, '周杰伦「嘉年华」世界巡回演唱会·成都站', 1,
  'https://pic.cyol.com/img/20231030/img_9630d9b9ed67631c1eeb79bb63346d3f9f_c.jpg',
  '金牛区', '成都凤凰山体育公园 · 金牛区凤凰山体育路1号',
  104.054108, 30.714592,
  350, 0000076000, 0000022000, 49,
  '2024-06-08 19:30 / 2024-06-09 19:30',
  '2021-12-22 11:17:15', '2026-05-02 03:40:15', 4
);

INSERT INTO `tb_shop` VALUES (
  8, '草莓音乐节2024·成都站', 6,
  'https://pic.cyol.com/img/20231030/img_9630d9b9ed67631c1eeb79bb63346d3f9f_c.jpg',
  '金牛区', '成都凤凰山体育公园 · 金牛区凤凰山体育路1号',
  104.054108, 30.714592,
  280, 0000043000, 0000017200, 47,
  '2024-05-01 10:00 至 2024-05-03 22:00',
  '2021-12-22 11:17:15', '2026-05-02 03:40:15', 4
);

-- ----------------------------------------------------------
-- venue_id=5  上海当代艺术博物馆  2场展览
-- x/y: 121.480491, 31.236136
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  9, 'teamLab Borderless·上海无界美术馆2024', 2,
  'https://icity-static.icitycdn.com/images/uploads/ap/imsm/museum/pic_head/hmo59bc/8a67bd7e479331d1hmo59bc.jpg',
  '黄浦区', '上海当代艺术博物馆 · 黄浦区花园港路200号',
  121.480491, 31.236136,
  120, 0000034000, 0000018900, 49,
  '11:00-19:00（周二休馆）',
  '2021-12-22 11:20:58', '2026-05-02 03:40:55', 5
);

INSERT INTO `tb_shop` VALUES (
  10, '村上隆「超级扁平」大型个展·上海', 2,
  'https://icity-static.icitycdn.com/images/uploads/ap/imsm/museum/pic_head/hmo59bc/8a67bd7e479331d1hmo59bc.jpg',
  '黄浦区', '上海当代艺术博物馆 · 黄浦区花园港路200号',
  121.480491, 31.236136,
  150, 0000028000, 0000014500, 48,
  '10:00-18:00（周一休馆）',
  '2021-12-22 11:20:58', '2026-05-02 03:40:55', 5
);

-- ----------------------------------------------------------
-- venue_id=6  北京798艺术区  1场展览
-- x/y: 116.497473, 39.984556
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  11, 'KAWS·HOLIDAY 亚洲巡回展·北京', 2,
  'https://thumbs.dreamstime.com/b/798-72865898.jpg',
  '朝阳区', '北京798艺术区 · 朝阳区酒仙桥路4号',
  116.497473, 39.984556,
  80, 0000052000, 0000029400, 47,
  '10:00-18:00',
  '2021-12-22 11:24:53', '2026-05-02 03:42:37', 6
);

-- ----------------------------------------------------------
-- venue_id=7  笑果文化·上海演艺新空间  3场脱口秀
-- x/y: 121.487252, 31.225316
-- blog_id=5 对应"Norah脱口秀专场"，shop_id=5 → 活动id需为某个值
-- 这里安排 Norah专场 id=13，blog 中 shop_id=1 需调整（见下方说明）
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  12, 'Norah·脱口秀专场「反正也没人看」', 3,
  'https://n.sinaimg.cn/spider20230420/74/w500h374/20230420/aad3-b9459b66d4319d890f9c9afaa7c41d14.png',
  '黄浦区', '笑果文化·上海演艺新空间 · 黄浦区西藏南路666号',
  121.487252, 31.225316,
  98, 0000018500, 0000009200, 48,
  '每周五/六/日 20:00场',
  '2021-12-22 11:40:52', '2026-05-02 03:43:41', 7
);

INSERT INTO `tb_shop` VALUES (
  13, '王勉×Joe·双人脱口秀专场「化学反应」', 3,
  'https://n.sinaimg.cn/spider20230420/74/w500h374/20230420/aad3-b9459b66d4319d890f9c9afaa7c41d14.png',
  '黄浦区', '笑果文化·上海演艺新空间 · 黄浦区西藏南路666号',
  121.487252, 31.225316,
  158, 0000014200, 0000007800, 47,
  '每周六/日 19:30场',
  '2021-12-22 11:40:52', '2026-05-02 03:43:41', 7
);

INSERT INTO `tb_shop` VALUES (
  14, '笑果开放麦·新人专场', 3,
  'https://n.sinaimg.cn/spider20230420/74/w500h374/20230420/aad3-b9459b66d4319d890f9c9afaa7c41d14.png',
  '黄浦区', '笑果文化·上海演艺新空间 · 黄浦区西藏南路666号',
  121.487252, 31.225316,
  68, 0000009800, 0000004200, 45,
  '每周四 20:00场',
  '2021-12-22 11:40:52', '2026-05-02 03:43:41', 7
);

-- ----------------------------------------------------------
-- venue_id=8  单立人喜剧·北京剧场  1场脱口秀
-- x/y: 116.449921, 39.921509
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  15, 'Rock·脱口秀个人专场「我也不知道」', 3,
  'https://pic.cyol.com/img/20211207/img_9601bb7559eee03f7ace66e7132e926680e2.jpeg',
  '朝阳区', '单立人喜剧·北京剧场 · 朝阳区朝外大街甲6号万通中心C座',
  116.449921, 39.921509,
  128, 0000014200, 0000007800, 46,
  '每周五/六 19:30场',
  '2021-12-22 11:51:06', '2026-05-02 03:44:56', 8
);

-- ----------------------------------------------------------
-- venue_id=9  上海浦东足球场  1场体育赛事
-- x/y: 121.570648, 31.178022
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  16, '2024赛季中超联赛·上海申花主场赛（vs武汉三镇）', 4,
  'https://wenhui.whb.cn/u/cms/www/201804/28124854ys23.jpg',
  '浦东新区', '上海浦东足球场 · 浦东新区杨高南路3999号',
  121.570648, 31.178022,
  180, 0000041000, 0000015600, 46,
  '2024-08-24 19:35',
  '2021-12-22 11:53:59', '2026-05-02 03:46:04', 9
);

-- ----------------------------------------------------------
-- venue_id=10  草莓音乐节·上海主会场  1场音乐节
-- x/y: 121.197842, 31.289651
-- blog_id=6 对应此活动，shop_id=10 → 调整为实际 id=17
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  17, '草莓音乐节2024·上海站', 6,
  'https://img.jiemian.com/101/original/20190428/155644067780805400.jpg',
  '嘉定区', '草莓音乐节·上海主会场 · 嘉定区安亭镇墨玉南路777号',
  121.197842, 31.289651,
  280, 0000063000, 0000024500, 48,
  '2024-05-01 10:00 至 2024-05-03 22:00',
  '2021-12-22 12:25:16', '2026-05-02 03:47:12', 10
);

-- ----------------------------------------------------------
-- venue_id=11  国家大剧院  2场话剧
-- x/y: 116.389550, 39.903816
-- blog_id=7 对应"沉浸式实景演出"，shop_id=11 → 调整为实际 id=18
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  18, '音乐剧《悲惨世界》·国家大剧院版', 5,
  'https://thumbs.dreamstime.com/b/%E5%9B%BD%E5%AE%B6%E5%A4%A7%E5%89%A7%E9%99%A2%EF%BC%8C%E5%8C%97%E4%BA%AC-%E4%B8%AD%E5%9B%BD-73713497.jpg',
  '西城区', '国家大剧院 · 西城区西长安街2号',
  116.389550, 39.903816,
  280, 0000058000, 0000027300, 49,
  '每周二至周日 19:30，周末加演14:00场',
  '2021-12-22 12:29:02', '2026-05-02 03:48:55', 11
);

INSERT INTO `tb_shop` VALUES (
  19, '经典音乐剧《猫》·中文版全国巡演·北京站', 5,
  'https://thumbs.dreamstime.com/b/%E5%9B%BD%E5%AE%B6%E5%A4%A7%E5%89%A7%E9%99%A2%EF%BC%8C%E5%8C%97%E4%BA%AC-%E4%B8%AD%E5%9B%BD-73713497.jpg',
  '西城区', '国家大剧院 · 西城区西长安街2号',
  116.389550, 39.903816,
  320, 0000049000, 0000021800, 48,
  '2024-12-20 至 2025-01-05 19:30场',
  '2021-12-22 12:29:02', '2026-05-02 03:48:55', 11
);

-- ----------------------------------------------------------
-- venue_id=12  上海国际会议中心  1场见面会
-- x/y: 121.512914, 31.235256
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  20, 'ENHYPEN 2024 FAN MEETING·上海站', 9,
  'https://qimgs.qunarzz.com/mp_ctrip_gateway_0001/0101m120009d0uu5n7B90_W_10000_1200.jpg',
  '浦东新区', '上海国际会议中心 · 浦东新区陆家嘴环路2727号',
  121.512914, 31.235256,
  150, 0000019800, 0000006400, 47,
  '2024-09-28 14:00 / 18:30 双场',
  '2021-12-22 12:34:34', '2026-05-02 03:49:31', 12
);

-- ----------------------------------------------------------
-- venue_id=13  上海迪士尼小镇剧场  1场亲子活动
-- x/y: 121.666884, 31.148858
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  21, '米奇魔法故事会·亲子互动剧场2024暑假专场', 7,
  'https://static.shanghaidisneyresort.com/tridion/prod/zh-cn/system/images/shdr-ent-mickey-storybook-adventure-video-cover_tcm1874-242099.jpg',
  '浦东新区', '上海迪士尼小镇剧场 · 浦东新区川沙新镇南六公路310号',
  121.666884, 31.148858,
  90, 0000032000, 0000014700, 48,
  '每日 10:00 / 14:00 / 16:30 三场',
  '2021-12-22 12:38:54', '2026-05-02 03:50:36', 13
);

-- ----------------------------------------------------------
-- venue_id=14  造作艺术空间·北京  1场工作坊
-- x/y: 116.454033, 39.937563
-- ----------------------------------------------------------
INSERT INTO `tb_shop` VALUES (
  22, '手工皮具工作坊·零基础制作专属钱包', 10,
  'https://images.adsttc.com/media/images/63bb/faf8/f7e2/c001/7079/3ca3/newsletter/not-ready-flop-art-space-achipoetry-studio_1.jpg',
  '朝阳区', '造作艺术空间·北京 · 朝阳区三里屯太古里南区B1-02',
  116.454033, 39.937563,
  60, 0000008900, 0000003200, 47,
  '每周六/日 14:00-17:00',
  '2021-12-22 12:48:54', '2026-05-02 03:51:15', 14
);


-- ============================================================
-- tb_blog  活动图文帖
--
-- ⚠️ shop_id 对应关系调整说明：
--   原 blog_id=4: shop_id=4 → 成都周杰伦 → 新 tb_shop.id=7
--   原 blog_id=5: shop_id=1 → Norah脱口秀 → 新 tb_shop.id=12
--   原 blog_id=6: shop_id=10 → 草莓音乐节上海 → 新 tb_shop.id=17
--   原 blog_id=7: shop_id=11 → 国家大剧院话剧 → 新 tb_shop.id=18
--
-- images 路径完全沿用你现有文件中的路径，不做任何改动
-- ============================================================
DROP TABLE IF EXISTS `tb_blog`;
CREATE TABLE `tb_blog` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `shop_id`     bigint(20) NOT NULL COMMENT '关联活动id（tb_shop.id）',
  `user_id`     bigint(20) UNSIGNED NOT NULL COMMENT '发帖用户id',
  `title`       varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '帖子标题',
  `images`      varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '图片，多张以","隔开',
  `content`     varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '图文内容',
  `liked`       int(8) UNSIGNED NULL DEFAULT 0 COMMENT '点赞/收藏数',
  `comments`    int(8) UNSIGNED NULL DEFAULT NULL COMMENT '评论数',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 23
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- shop_id 已更新为新 tb_shop 中对应活动的 id
-- images 路径完全沿用现有数据，一个字符不改
INSERT INTO `tb_blog` VALUES (
  4, 7, 2,
  '去看了周杰伦「嘉年华」世界巡回演唱会🎤 泪目全程，值回票价！',
  '/imgs/blogs/7/14/4771fefb-1a87-4252-816c-9f7ec41ffa4a.jpg,/imgs/blogs/4/10/2f07e3c9-ddce-482d-9ea7-c21450f8d7cd.jpg,/imgs/blogs/2/6/b0756279-65da-4f2d-b62a-33f74b06454a.jpg,/imgs/blogs/10/7/7e97f47d-eb49-4dc9-a583-95faa7aed287.jpg,/imgs/blogs/1/2/4a7b496b-2a08-4af7-aa95-df2c3bd0ef97.jpg,/imgs/blogs/14/3/52b290eb-8b5d-403b-8373-ba0bb856d18e.jpg',
  '等了三年的演唱会，终于来了🥹<br/><br/>\r\n🏟️「成都凤凰山体育公园」6万人现场，氛围拉满到天花板<br/><br/>\r\n📍提前2小时到场，安检井然有序，工作人员态度很好<br/>\r\n🎫 座位在内场站票区，视野超好，离舞台很近<br/><br/>\r\n--------------🎵 演出详情 🎵--------------<br/><br/>\r\n「开场」<br/>\r\n灯光全灭，倒计时开始，全场尖叫声震天动地！<br/>\r\n第一首《夜曲》一出来，我直接哭了😭<br/>\r\n背后的大屏幕配合现场特效，视觉冲击力超强！<br/><br/>\r\n「舞台效果」<br/>\r\n升降台、火焰喷射、无人机矩阵，全程高能🔥<br/>\r\n唱到《七里香》的时候现场大合唱，真的太感动了<br/>\r\n我身边的阿姨从头哭到尾，说等了周杰伦二十年<br/><br/>\r\n「安可环节」<br/>\r\n足足唱了4首安可！最后《稻香》全场手机灯海，美哭🌾<br/><br/>\r\n--------------🗒️ 实用攻略 🗒️--------------<br/>\r\n【交通】地铁5号线凤凰山站B口出，步行10分钟<br/>\r\n【拍照】手机就够了，主舞台灯光打得很好<br/>\r\n【必带】充电宝+小零食+荧光棒（场馆外有卖，30元）<br/>\r\n【注意】禁止自带外置音响，安检会查<br/><br/>\r\n总结：人生必去一次的现场体验，强烈推荐！🎊',
  128, 104, '2021-12-28 11:50:01', '2022-03-10 06:26:34'
);

INSERT INTO `tb_blog` VALUES (
  5, 12, 2,
  '首刷「Norah脱口秀」超值！100块钱买到肚子疼😂强烈安利！',
  '/imgs/blogs/11/12/8b37d208-9414-4e78-b065-9199647bb3e3.jpg,/imgs/blogs/9/12/ac2ce2fb-0605-4f14-82cc-c962b8c86688.jpg',
  '第一次看线下脱口秀，完全超出预期！🎭<br/>\r\n场地不大但氛围绝了，感觉比看综艺爽多了！<br/><br/>\r\n📍「笑果文化·上海演艺新空间」<br/>\r\n地址：黄浦区西藏南路666号<br/>\r\n交通：地铁9号线鲁班路站步行8分钟<br/><br/>\r\n✔️ 「开放麦场次」（98元）<br/>\r\n新人表演，质量参差，但有惊喜！<br/>\r\n有个新人讲职场段子，全场笑得起不来，潜力很大👏<br/><br/>\r\n✔️ 「专场：王勉×Joe·双人专场」（158元）<br/>\r\n两个人的化学反应太好了！💥<br/>\r\n王勉弹吉他穿插段子，形式很新颖；Joe的自嘲式幽默笑点密集<br/>\r\n中场还有互动环节，我被抽到上台了，超级紧张但好玩！<br/><br/>\r\n✔️ 「返场加演」<br/>\r\n正常90分钟，当天加演了20分钟，赚到！<br/><br/>\r\n【小贴士】<br/>\r\n🪑 提前15分钟入场选好位置，推荐中间偏前排<br/>\r\n🍺 场内可以买酒水，气氛更好<br/>\r\n📱 全程禁止录像，认真在乎演员版权<br/>\r\n🎟️ 工作日场比周末便宜30元，学生党抓紧<br/><br/>\r\n下次还来！已经在抢下一场了🔥',
  56, 0, '2021-12-28 12:57:49', '2026-05-02 03:25:33'
);

INSERT INTO `tb_blog` VALUES (
  6, 17, 1,
  '草莓音乐节踩坑指南🍓进去前一定要看！避开99%的雷区',
  '/imgs/blogs/blog1.jpg',
  '去了三届草莓，血泪总结！这篇攻略能帮你少踩无数坑，建议收藏🔖',
  43, 0, '2022-01-11 08:05:47', '2022-03-10 01:21:41'
);

INSERT INTO `tb_blog` VALUES (
  7, 18, 1,
  '《悲惨世界》国家大剧院版✨比想象中震撼100倍！',
  '/imgs/blogs/blog2.jpg',
  '不是普通话剧，是真正的百老汇级别制作，舞台效果炸裂，太绝了！',
  67, 0, '2022-01-11 08:05:47', '2026-05-02 02:57:57'
);


-- ============================================================
-- tb_blog_comments  结构不变，数据为空
-- ============================================================
DROP TABLE IF EXISTS `tb_blog_comments`;
CREATE TABLE `tb_blog_comments` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id`     bigint(20) UNSIGNED NOT NULL COMMENT '用户id',
  `blog_id`     bigint(20) UNSIGNED NOT NULL COMMENT '关联帖子id',
  `parent_id`   bigint(20) UNSIGNED NOT NULL COMMENT '一级评论id，一级填0',
  `answer_id`   bigint(20) UNSIGNED NOT NULL COMMENT '回复的评论id',
  `content`     varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '评论内容',
  `liked`       int(8) UNSIGNED NULL DEFAULT NULL COMMENT '点赞数',
  `status`      tinyint(1) UNSIGNED NULL DEFAULT NULL COMMENT '0正常 1被举报 2禁止查看',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;


-- ============================================================
-- tb_follow  关注关系，沿用现有数据
-- ============================================================
DROP TABLE IF EXISTS `tb_follow`;
CREATE TABLE `tb_follow` (
  `id`             bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id`        bigint(20) UNSIGNED NOT NULL COMMENT '关注者id',
  `follow_user_id` bigint(20) UNSIGNED NOT NULL COMMENT '被关注者id',
  `create_time`    timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 10
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

INSERT INTO `tb_follow` VALUES (8, 6, 2, '2026-05-02 05:41:17');
INSERT INTO `tb_follow` VALUES (9, 6, 1, '2026-05-02 05:41:31');


-- ============================================================
-- tb_voucher  票种
--
-- ⚠️ shop_id 对应关系调整说明：
--   原 shop_id=1（梅奔场馆）→ 新 shop_id=1（Taylor Swift·上海站活动）
--   原 shop_id=7（笑果场馆）→ 新 shop_id=12（Norah脱口秀专场活动）
--   原 shop_id=10（草莓场馆）→ 新 shop_id=17（草莓音乐节2024·上海站活动）
--   原 shop_id=11（国大场馆）→ 新 shop_id=18（悲惨世界活动）
-- ============================================================
DROP TABLE IF EXISTS `tb_voucher`;
CREATE TABLE `tb_voucher` (
  `id`           bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `shop_id`      bigint(20) UNSIGNED NULL DEFAULT NULL COMMENT '关联活动id（tb_shop.id）',
  `title`        varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '票种名称',
  `sub_title`    varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '附加说明',
  `rules`        varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '购票须知',
  `pay_value`    bigint(10) UNSIGNED NOT NULL COMMENT '实付金额（分）',
  `actual_value` bigint(10) NOT NULL COMMENT '票面价值（分）',
  `type`         tinyint(1) UNSIGNED NOT NULL DEFAULT 0 COMMENT '0普通票 1限时特价票',
  `status`       tinyint(1) UNSIGNED NOT NULL DEFAULT 1 COMMENT '1上架 2下架 3已结束',
  `create_time`  timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time`  timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 10
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- 普通票（type=0）
INSERT INTO `tb_voucher` VALUES (1, 1,  '内场站票',     '距离舞台最近区域，氛围最佳',         '实名购票，凭身份证入场\n演出当日不支持退换票\n禁止携带专业摄影设备\n请提前30分钟到场安检', 58000, 58000,  0, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (2, 1,  '看台坐票A区',  '正对主舞台，视野开阔',               '实名购票，凭身份证入场\n演出当日不支持退换票\n禁止携带专业摄影设备',                       48000, 48000,  0, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (3, 1,  'VIP席位',      '含纪念画册+后台见面抽签资格',        '实名购票，凭身份证入场\n含纪念册一本（现场领取）\n后台见面抽签机会（不保证中签）\n演出当日不支持退换票', 128000, 128000, 0, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (4, 12, '脱口秀·普通场','周四至周日场次通用',                '演出开始后15分钟内可入座\n请勿大声喧哗影响演员\n禁止录音录像',                               9800,  9800,   0, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (5, 18, '话剧·普通票',  '适用非节假日场次',                   '实名购票\n演出前1小时停止检票\n迟到15分钟后谢绝入场',                                       28000, 28000,  0, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');

-- 限时特价票（type=1，配合 tb_seckill_voucher）
INSERT INTO `tb_voucher` VALUES (6, 1,  '【限时特价】内场站票·早鸟价',    '开票前72小时限量特价，抢完即止', '与普通内场票权益完全相同\n限购2张/人\n本票不可退换',                               36000, 58000,  1, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (7, 12, '【限时特价】脱口秀·捡漏票',      '临场48小时前释放的退票特价处理','与普通票权益相同\n限购1张/人\n本票不可退换',                                     5800,  9800,   1, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (8, 17, '【限时特价】草莓音乐节·单日票',  '周年庆特价，仅限周六场次',       '仅限周六场次使用\n限购2张/人\n不含停车位',                                         19800, 38000,  1, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');
INSERT INTO `tb_voucher` VALUES (9, 18, '【限时特价】话剧·学生优惠票',    '凭学生证核验，现场验证资格',     '入场时需出示有效学生证\n限购1张/人\n仅限工作日场次',                               16800, 28000,  1, 1, '2022-01-04 01:42:39', '2022-01-04 01:43:31');


-- ============================================================
-- tb_seckill_voucher  限时特价票抢购活动（数据完全沿用现有）
-- ============================================================
DROP TABLE IF EXISTS `tb_seckill_voucher`;
CREATE TABLE `tb_seckill_voucher` (
  `voucher_id`  bigint(20) UNSIGNED NOT NULL COMMENT '关联票种id',
  `stock`       int(8) NOT NULL COMMENT '特价票剩余库存',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `begin_time`  timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '抢购开始时间',
  `end_time`    timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '抢购结束时间',
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`voucher_id`) USING BTREE
) ENGINE = InnoDB
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- 完全沿用现有数据
INSERT INTO `tb_seckill_voucher` VALUES (6, 200, '2022-01-04 01:42:39', '2022-01-25 12:00:00', '2022-02-10 15:59:59', '2022-01-04 01:43:31');
INSERT INTO `tb_seckill_voucher` VALUES (7,  50, '2022-01-04 01:42:39', '2022-01-25 04:00:00', '2022-01-27 15:59:59', '2022-01-04 01:43:31');
INSERT INTO `tb_seckill_voucher` VALUES (8, 500, '2022-01-04 01:42:39', '2022-01-20 02:00:00', '2022-02-20 15:59:59', '2022-01-04 01:43:31');
INSERT INTO `tb_seckill_voucher` VALUES (9, 100, '2022-01-04 01:42:39', '2022-01-25 02:00:00', '2022-03-31 15:59:59', '2022-01-04 01:43:31');


-- ============================================================
-- tb_voucher_order  购票订单（结构完全不变，数据为空）
-- ============================================================
DROP TABLE IF EXISTS `tb_voucher_order`;
CREATE TABLE `tb_voucher_order` (
  `id`          bigint(20) NOT NULL COMMENT '订单号（IDWorker生成）',
  `user_id`     bigint(20) UNSIGNED NOT NULL COMMENT '购票用户id',
  `voucher_id`  bigint(20) UNSIGNED NOT NULL COMMENT '票种id',
  `pay_type`    tinyint(1) UNSIGNED NOT NULL DEFAULT 1 COMMENT '1余额 2支付宝 3微信',
  `status`      tinyint(1) UNSIGNED NOT NULL DEFAULT 1 COMMENT '1未支付 2已支付 3已入场 4已取消 5退款中 6已退款',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `pay_time`    timestamp NULL DEFAULT NULL,
  `use_time`    timestamp NULL DEFAULT NULL,
  `refund_time` timestamp NULL DEFAULT NULL,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;


-- ============================================================
-- tb_user  用户（数据完全沿用现有，icon路径不变）
-- ============================================================
DROP TABLE IF EXISTS `tb_user`;
CREATE TABLE `tb_user` (
  `id`          bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `phone`       varchar(11) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '手机号',
  `password`    varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT '' COMMENT '密码',
  `nick_name`   varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT '' COMMENT '昵称',
  `icon`        varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT '' COMMENT '头像',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uniqe_key_phone`(`phone`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1010
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

-- 完全沿用现有数据，icon路径不变
INSERT INTO `tb_user` VALUES (1, '13686869696', '', '演出狂魔小鱼',   '/imgs/icons/icon_1.jpg', '2021-12-24 02:27:19', '2026-05-02 03:00:41');
INSERT INTO `tb_user` VALUES (2, '13838411438', '', '笑点低的可可',   '/imgs/icons/icon_2.jpg', '2021-12-24 07:14:39', '2026-05-02 03:00:37');
INSERT INTO `tb_user` VALUES (4, '13456789011', '', 'user_slxaxy2au9','/imgs/icons/icon_4.jpg', '2022-01-07 04:07:53', '2026-05-02 03:00:46');
INSERT INTO `tb_user` VALUES (5, '13456789001', '', '音乐节打卡侠',   '/imgs/icons/icon_5.jpg', '2022-01-07 08:11:33', '2026-05-02 03:00:54');
INSERT INTO `tb_user` VALUES (6, '13456762069', '', 'user_xn5wr3hps', '/imgs/icons/icon_6.jpg', '2022-02-07 09:54:10', '2026-05-02 03:00:59');


-- ============================================================
-- tb_user_info  用户详情（数据完全沿用现有）
-- ============================================================
DROP TABLE IF EXISTS `tb_user_info`;
CREATE TABLE `tb_user_info` (
  `user_id`     bigint(20) UNSIGNED NOT NULL COMMENT '用户id',
  `city`        varchar(64)  CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT '' COMMENT '所在城市',
  `introduce`   varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL DEFAULT NULL COMMENT '个人简介',
  `fans`        int(8) UNSIGNED NULL DEFAULT 0 COMMENT '粉丝数',
  `followee`    int(8) UNSIGNED NULL DEFAULT 0 COMMENT '关注数',
  `gender`      tinyint(1) UNSIGNED NULL DEFAULT 0 COMMENT '0男 1女',
  `birthday`    date NULL DEFAULT NULL COMMENT '生日',
  `credits`     int(8) UNSIGNED NULL DEFAULT 0 COMMENT '积分',
  `level`       tinyint(1) UNSIGNED NULL DEFAULT 0 COMMENT '会员等级 0~9',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`) USING BTREE
) ENGINE = InnoDB
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;

INSERT INTO `tb_user_info` VALUES (1, '上海', '演唱会现场爱好者，看过200+场演出🎤', 1282, 43, 0, '1995-06-15', 8800, 5, '2021-12-24 02:27:19', '2026-05-02 05:41:31');
INSERT INTO `tb_user_info` VALUES (2, '北京', '脱口秀痴迷者，每月至少刷三场😂',      565,  28, 1, '1998-11-22', 3200, 3, '2021-12-24 07:14:39', '2026-05-02 05:41:17');
INSERT INTO `tb_user_info` VALUES (5, '成都', '音乐节打卡中，目标全国50个音乐节🍓', 230,  15, 1, '2000-03-08', 1500, 2, '2022-01-07 08:11:33', '2022-03-11 01:09:20');
INSERT INTO `tb_user_info` VALUES (6, '',     NULL,                                   0,    2,  0, NULL,         0,    0, '2026-05-02 03:53:46', '2026-05-02 05:41:31');


-- ============================================================
-- tb_sign  每日签到（结构完全不变，数据为空）
-- ============================================================
DROP TABLE IF EXISTS `tb_sign`;
CREATE TABLE `tb_sign` (
  `id`        bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id`   bigint(20) UNSIGNED NOT NULL COMMENT '用户id',
  `year`      year NOT NULL COMMENT '签到年份',
  `month`     tinyint(2) NOT NULL COMMENT '签到月份',
  `date`      date NOT NULL COMMENT '签到日期',
  `is_backup` tinyint(1) UNSIGNED NULL DEFAULT NULL COMMENT '是否补签',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1
  CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Compact;


/*!40014 SET FOREIGN_KEY_CHECKS=IFNULL(@OLD_FOREIGN_KEY_CHECKS, 1) */;