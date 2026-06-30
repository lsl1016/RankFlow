-- RankFlow schema bootstrap. GORM AutoMigrate also creates these tables,
-- but keeping the canonical DDL here documents the data model and seeds data.

CREATE DATABASE IF NOT EXISTS rankflow DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE rankflow;

CREATE TABLE IF NOT EXISTS rank_config (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    rank_id BIGINT NOT NULL UNIQUE COMMENT '榜单ID',
    rank_name VARCHAR(128) NOT NULL COMMENT '榜单名称',
    biz_code VARCHAR(64) NOT NULL COMMENT '业务线编码',
    target_type VARCHAR(32) NOT NULL COMMENT '上榜对象类型:user/content/room/product/org',
    status TINYINT NOT NULL DEFAULT 0 COMMENT '状态:0草稿,1上线,2下线,3归档',
    sort_type VARCHAR(32) NOT NULL COMMENT '排序类型:score_desc/score_asc',
    same_score_policy VARCHAR(32) NOT NULL COMMENT '同分排序策略',
    score_integer_digits INT NOT NULL DEFAULT 12 COMMENT '真实分数整数位数',
    max_rank_size INT NOT NULL DEFAULT 10000 COMMENT '榜单最大长度',
    redis_cluster VARCHAR(64) DEFAULT '' COMMENT 'Redis集群标识',
    mysql_cluster VARCHAR(64) DEFAULT '' COMMENT 'MySQL集群标识',
    cache_ttl_seconds INT NOT NULL DEFAULT 3600 COMMENT '缓存TTL',
    start_time DATETIME DEFAULT NULL COMMENT '积分开始时间',
    end_time DATETIME DEFAULT NULL COMMENT '积分结束时间',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
) COMMENT='榜单基础配置表';

CREATE TABLE IF NOT EXISTS rank_dimension_config (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    rank_id BIGINT NOT NULL COMMENT '榜单ID',
    dimension_name VARCHAR(64) NOT NULL COMMENT '维度名称',
    dimension_field VARCHAR(64) NOT NULL COMMENT '维度字段',
    dimension_order INT NOT NULL COMMENT '拼接顺序',
    required TINYINT NOT NULL DEFAULT 1 COMMENT '是否必填',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    KEY idx_rank_id (rank_id)
) COMMENT='榜单维度配置表';

CREATE TABLE IF NOT EXISTS rank_time_config (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    rank_id BIGINT NOT NULL COMMENT '榜单ID',
    time_type VARCHAR(32) NOT NULL COMMENT '时间粒度:none/hour/day/week/month/season/custom',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai' COMMENT '时区',
    anchor_type VARCHAR(32) NOT NULL COMMENT '时间锚点:event_time/request_time',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE KEY uk_rank_id (rank_id)
) COMMENT='榜单时间维度配置表';

CREATE TABLE IF NOT EXISTS rank_member_score (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    rank_id BIGINT NOT NULL COMMENT '榜单ID',
    type_id VARCHAR(256) NOT NULL COMMENT '子榜维度ID',
    item_id VARCHAR(128) NOT NULL COMMENT '上榜对象ID',
    score BIGINT NOT NULL DEFAULT 0 COMMENT '业务真实分数',
    sub_score BIGINT NOT NULL DEFAULT 0 COMMENT '二级排序分',
    final_score DECIMAL(32, 8) NOT NULL COMMENT '最终排序分',
    rank_no INT DEFAULT NULL COMMENT '当前排名，可异步刷新',
    last_event_time DATETIME DEFAULT NULL COMMENT '最后一次加分事件时间',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE KEY uk_rank_type_item (rank_id, type_id, item_id),
    KEY idx_rank_type_score (rank_id, type_id, score)
) COMMENT='榜单成员分数表';
