# RankFlow · 通用榜单服务 (MVP)

可配置、可复用的榜单基础服务。基于设计文档 [`docs/通用榜单服务.md`](docs/通用榜单服务.md) 的 MVP 范围实现。

技术栈：**Go (Gin + GORM) · Redis ZSet / Streams · MySQL · Vue3 + Ant Design Vue**。

## 能力范围（MVP）

- 榜单配置：创建 / 编辑 / 上下线，维度配置（全站 / 自定义维度 / 时间维度日榜·月榜等）
- 分数更新：`addScore` / `setScore` / `batchAddScore`
- 原子写入：单次 Redis Lua 同时完成分数、排名索引、revision、幂等结果和 Stream 持久化消息
- 排名存储：Redis ZSet 实现 TopN、我的排名、周边排名；主分写 ZSet score，同分顺序编码在 member 中，避免大分数 float64 精度丢失
- 排序：先到优先 / 后到优先 / 自定义二级分，支持升/降序
- 持久化：Redis Streams Consumer Group 异步落库 MySQL，revision 防止多 Worker 乱序覆盖；`XAUTOCLAIM` 接管失联 Consumer 的 stale pending，安全 `XTRIM MINID` 回收已完成前缀
- 恢复：支持旧 ZSet 惰性迁移及 MySQL → Redis 排名重建
- 写分热路径：子榜状态使用 Redis 缓存，已有子榜不再在每次写分时 Upsert MySQL 元数据
- API 权限：公开查询、Writer 写分、Admin 配置管理三层隔离
- 管理后台：榜单列表、新建/编辑、详情页（实时排名 + 概览 + 测试加分）
- 可观测：结构化访问日志、QPS / 缓存命中率基础指标

> 暂缓（二期）：复杂规则引擎、慢队列聚合、热点自动探测、结榜审核、自动归档。

## 目录结构

```text
RankFlow/
├── backend/              # Go 服务
│   ├── cmd/api           # HTTP 入口 + 异步落库 worker
│   ├── internal/
│   │   ├── config/       # YAML / 环境变量配置
│   │   ├── model/        # GORM 模型
│   │   ├── store/        # mysql / redis 仓储
│   │   ├── dimension/    # type_id 维度计算
│   │   ├── score/        # 同分顺序编码
│   │   ├── service/      # 配置 / 写入 / 查询服务
│   │   ├── queue/        # Redis Streams 异步落库 worker
│   │   ├── api/          # handler / middleware / router
│   │   └── observability/# 日志 + 指标
│   └── deployments/init.sql
├── web-admin/            # Vue3 + Ant Design Vue 管理后台
└── docker-compose.yml    # 本地 MySQL + Redis
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
# 默认读取 backend/config.yaml，可用 RANKFLOW_* 环境变量覆盖
go run ./cmd/api
# 默认监听 :8080
```

本地 `backend/config.yaml` 默认关闭 API 鉴权，便于开发。需要本地验证鉴权时可设置：

```bash
export RANKFLOW_AUTH_ENABLED=true
export RANKFLOW_ADMIN_TOKEN='replace-with-admin-secret'
export RANKFLOW_WRITER_TOKEN='replace-with-writer-secret'
go run ./cmd/api
```

启用鉴权时 Admin / Writer token 都必须非空且互不相同，否则服务拒绝启动。

### 3. 启动管理后台

```bash
cd web-admin
npm install
npm run dev
# 打开 http://localhost:5173 ，/api 已代理到 :8080
```

生产鉴权开启后，管理后台右上角可设置 Admin Token。Token 只保存在当前浏览器会话的 `sessionStorage` 中，并通过 `Authorization: Bearer <token>` 发送。

## 核心概念

- 榜单实例 = `rank_id + type_id`；`type_id` 由「时间桶 + 横向维度」生成。
- 成员 = `item_id`。
- Redis ZSet 的 score 只保存业务主分；同分顺序由固定宽度 member 前缀表达，因此不再依赖 `final_score` 小数精度。
- `final_score` 保留用于兼容展示 / MySQL 落库，不再作为 Redis 排名依据。
- 查询接口不会创建 `rank_sub_board`。子榜只会由显式 `POST /subboards` 或写分路径物化。
- 写分时优先读取 Redis 子榜状态缓存；只有 cache miss 才查 MySQL，只有确认子榜不存在时才创建，因此正常热路径不会反复更新 `rank_sub_board.updated_at`。

## API 权限

生产环境使用两个独立 Bearer Token：

- **Public**：无需 Token，仅可查询排名。
- **Writer**：`RANKFLOW_WRITER_TOKEN`，可执行写分接口。
- **Admin**：`RANKFLOW_ADMIN_TOKEN`，可执行所有管理接口，并自动拥有 Writer 权限。

权限边界：

| 权限 | 接口 |
|---|---|
| Public | `GET /api/ranks/{id}/top`、`GET /members/{itemId}/rank`、`GET /members/{itemId}/around`、`GET /stats` |
| Writer | `POST /api/ranks/{id}/score/add`、`score/set`、`score/batch` |
| Admin | 榜单创建 / 查询配置 / 编辑 / 上下线 / 子榜管理，以及 `/swagger/*` |

管理接口携带：

```text
Authorization: Bearer <RANKFLOW_ADMIN_TOKEN>
```

写分调用方可使用 Writer Token；Admin Token 同样可调用写分接口。Token 不支持通过 query string 传递。

## 写分与幂等

`score/add` 是增量操作，**`requestId` 必填**。同一 `rankId + requestId`：

- 完全相同的请求重试：返回第一次写入结果，不重复加分，也不重复产生 Stream 消息。
- 请求内容不同：返回 HTTP `409`，拒绝复用幂等键。
- 分数更新、revision、ZSet、幂等记录、Redis Stream `XADD` 在同一 Lua 脚本中执行。

示例：

```json
{
  "requestId": "order-20260915-0001",
  "itemId": "user_10086",
  "score": 10,
  "subScore": 0,
  "eventTime": 1789400000,
  "dimensions": {}
}
```

`score/set` 是绝对值覆盖；可不传 `requestId`，如果传入则同样启用幂等冲突检测。

## 异步持久化可靠性

Redis Stream 是 MySQL 最终持久化链路，不作为长期审计日志使用：

- Worker 启动时先恢复自己名下的 pending；运行期间周期性使用 `XAUTOCLAIM` 扫描 stale pending，把已经失联或长时间无响应 Consumer 的消息转移到健康 Worker。
- MySQL 仍使用 revision 条件写入，因此消息被重新接管、重复投递时不会用旧 revision 覆盖新状态。
- Worker 定期执行安全 `XTRIM MINID`：如果存在 pending，以最老 pending ID 为边界；如果不存在 pending，以 Consumer Group 的 `last-delivered-id` 为边界。只删除确定已经完成的前缀，不删除 pending 或尚未投递的消息。

这使持久化链路保持 at-least-once 语义，同时避免 Stream 因已 ACK 历史记录长期累积而无限增长。

## API 速览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/ranks` | 创建榜单（Admin） |
| GET | `/api/ranks` | 配置列表（Admin） |
| GET | `/api/ranks/{id}` | 配置详情（Admin） |
| PUT | `/api/ranks/{id}` | 编辑（Admin） |
| POST | `/api/ranks/{id}/status` | 上下线（Admin） |
| GET | `/api/ranks/{id}/subboards` | 子榜列表（Admin） |
| POST | `/api/ranks/{id}/subboards` | 显式创建/解析子榜（Admin） |
| POST | `/api/ranks/{id}/subboards/status` | 子榜上下线（Admin） |
| POST | `/api/ranks/{id}/score/add` | 加分（Writer，`requestId` 必填） |
| POST | `/api/ranks/{id}/score/set` | 设置分数（Writer） |
| POST | `/api/ranks/{id}/score/batch` | 批量加分（Writer，每项 `requestId` 必填） |
| GET | `/api/ranks/{id}/top` | TopN（Public） |
| GET | `/api/ranks/{id}/members/{itemId}/rank` | 我的排名（Public） |
| GET | `/api/ranks/{id}/members/{itemId}/around` | 周边排名（Public） |
| GET | `/api/ranks/{id}/stats` | 实时概览（Public） |

查询子榜维度通过 `dim_` 前缀传参，例如 `?dim_business_id=community&dim_category_id=tech`。

## Swagger 文档

本地鉴权关闭时访问：<http://localhost:8080/swagger/index.html>。

生产鉴权开启后 `/swagger/*` 需要 Admin Bearer Token。浏览器直接打开 Swagger UI 无法方便注入 Header 时，建议通过受控反向代理访问或临时在可信本地环境关闭鉴权进行文档调试。

修改注解后重新生成：

```bash
cd backend
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api/main.go --parseInternal --parseDependency -o docs
```

## 测试

CI 使用真实 Redis + MySQL 服务执行集成测试。由于多个 Go package 共用测试数据库，CI 串行执行 package：

```bash
cd backend
go test -p 1 ./...
```

覆盖重点包括：大分数同分排序、Redis Streams pending / `XAUTOCLAIM` 接管 / 安全裁剪、revision 防回退、MySQL → Redis 恢复、原子写入与幂等、查询不隐式建子榜、写分热路径不重复 Upsert 子榜、Admin / Writer 鉴权。

## CI/CD

- `CI`：Pull Request 与 `push main` 执行后端测试、前端构建、前后端 Docker 镜像构建。
- `CD`：`main` CI 成功后构建并推送 GHCR 镜像，并通过 SSH 执行生产 Compose 部署。

生产部署使用 `docker-compose.prod.yml`。后端生产环境固定启用鉴权，至少需要：

- `RANKFLOW_MYSQL_DSN`
- `RANKFLOW_REDIS_ADDR`
- `RANKFLOW_REDIS_PASSWORD`
- `RANKFLOW_REDIS_DB`
- `RANKFLOW_PERSIST_WORKERS`
- `RANKFLOW_ADMIN_TOKEN`（必填）
- `RANKFLOW_WRITER_TOKEN`（必填，且必须与 Admin Token 不同）

GitHub 仓库还需配置部署 Secrets：

- `DEPLOY_HOST`
- `DEPLOY_PORT`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_KNOWN_HOSTS`
- `DEPLOY_PATH`
- `GHCR_USERNAME`
- `GHCR_PULL_TOKEN`
- `FRONTEND_PORT`
- 上述 `RANKFLOW_*` 配置项

服务器前置要求：Docker Engine + Docker Compose Plugin、部署目录可写、可访问 `ghcr.io`，且 SSH 用户有 Docker Compose 权限。
