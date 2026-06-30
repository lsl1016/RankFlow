# RankFlow · 通用榜单服务 (MVP)

可配置、可复用的榜单基础服务。基于设计文档 [`docs/通用榜单服务.md`](docs/通用榜单服务.md) 的 MVP 范围实现。

技术栈：**Go (Gin + GORM) · Redis ZSet · MySQL · Vue3 + Ant Design Vue**。

## 能力范围（MVP）

- 榜单配置：创建 / 编辑 / 上下线，维度配置（全站 / 自定义维度 / 时间维度日榜·月榜等）
- 分数更新：`addScore` / `setScore` / `batchAddScore`，基于 `requestId` 的幂等
- 排名存储：Redis ZSet 实现 TopN、我的排名、周边排名
- 排序：业务分 + 二级排序（先到优先 / 后到优先 / 自定义），支持升/降序
- 持久化：Redis 实时更新 + 异步队列落库 MySQL
- 管理后台：榜单列表、新建/编辑、详情页（实时排名 + 概览 + 测试加分）
- 可观测：结构化访问日志、QPS / 缓存命中率基础指标

> 暂缓（二期）：复杂规则引擎、慢队列聚合、热点自动探测、结榜审核、自动归档。

## 目录结构

```
RankFlow/
├── backend/              # Go 服务
│   ├── cmd/api           # HTTP 入口 + 异步落库 worker
│   ├── internal/
│   │   ├── config/       # 环境变量配置
│   │   ├── model/        # GORM 模型
│   │   ├── store/        # mysql / redis 仓储
│   │   ├── dimension/    # type_id 维度计算
│   │   ├── score/        # final_score 排序分计算
│   │   ├── service/      # 配置 / 写入 / 查询服务
│   │   ├── queue/        # Redis 异步落库 worker
│   │   ├── api/          # handler / middleware / router
│   │   └── observability/# 日志 + 指标
│   └── deployments/init.sql
├── web-admin/            # Vue3 + Ant Design Vue 管理后台
└── docker-compose.yml    # MySQL + Redis
```

## 快速开始

### 1. 启动依赖（MySQL + Redis）

```bash
docker compose up -d
```

`init.sql` 会自动建库建表；后端启动时也会执行 GORM AutoMigrate 兜底。

### 2. 启动后端

```bash
cd backend
go run ./cmd/api
# 监听 :8080，默认连接本机 MySQL/Redis（见 backend/.env.example）
```

### 3. 启动管理后台

```bash
cd web-admin
npm install
npm run dev
# 打开 http://localhost:5173 ，/api 已代理到 :8080
```

## 核心概念

- 榜单实例 = `rank_id + type_id`；`type_id` 由「时间桶 + 横向维度」拼接生成。
- 成员 = `item_id`；排序使用 `final_score`（整数位=业务分，小数位=二级排序）。
- Redis Key 约定见设计文档第 9 章；幂等键 `rank:idem:{rank_id}:{request_id}`。

## API 速览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/ranks` | 创建榜单 |
| GET | `/api/ranks` | 列表（name/bizCode/status/page/size） |
| GET | `/api/ranks/{id}` | 详情（含维度/时间配置） |
| PUT | `/api/ranks/{id}` | 编辑 |
| POST | `/api/ranks/{id}/status` | 上下线 `{status}` |
| POST | `/api/ranks/{id}/score/add` | 加分（幂等） |
| POST | `/api/ranks/{id}/score/set` | 设置分数 |
| POST | `/api/ranks/{id}/score/batch` | 批量加分 `{items:[...]}` |
| GET | `/api/ranks/{id}/top` | TopN（`offset/limit/timestamp/dim_*`） |
| GET | `/api/ranks/{id}/members/{itemId}/rank` | 我的排名 |
| GET | `/api/ranks/{id}/members/{itemId}/around` | 周边排名（`before/after`） |
| GET | `/api/ranks/{id}/stats` | 实时概览 |

查询子榜维度通过 `dim_` 前缀传参，例如 `?dim_business_id=community&dim_category_id=tech`。

## 测试

```bash
cd backend && go test ./...
```

纯逻辑单元测试覆盖维度拼接（`dimension`）和排序分编码（`score`）。
