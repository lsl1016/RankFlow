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
# 监听 :8080，默认读取 backend/config.yaml，可用环境变量覆盖
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

## DTO 分层与参数绑定

传输层与领域层解耦：

- `internal/dto`：HTTP 请求 / 响应对象，承载 gin 绑定规则（`binding` 标签）、字段校验、Swagger 注解与中文注释，统一响应信封为 `{code, message, data}`。
- `internal/service`：领域层输入 / 输出，不感知 HTTP 与绑定。
- `handler`：负责 `dto` ↔ `service` 的显式转换（`ToServiceInput` / `From*`）。

每个请求 / 响应字段均带中文注释；必填与枚举通过 `binding` 标签声明（如 `required`、`oneof`、`dive`）。

## Swagger 文档

启动后端后访问交互式文档：<http://localhost:8080/swagger/index.html>

修改注解后重新生成（需先 `go install github.com/swaggo/swag/cmd/swag@latest`）：

```bash
cd backend
swag init -g cmd/api/main.go --parseInternal --parseDependency -o docs
```

生成产物位于 `backend/docs/`（`swagger.json` / `swagger.yaml` / `docs.go`），已随仓库提交。

## 测试

```bash
cd backend && go test ./...
```

纯逻辑单元测试覆盖维度拼接（`dimension`）和排序分编码（`score`）。

## CI/CD

仓库新增两条 GitHub Actions 流水线：

- `CI`：在 `pull_request` 和 `push main` 时执行后端 `go test ./...`，以及前端 `npm ci && npm run build`
- `CD`：在 `CI` 成功且分支为 `main` 时，构建并推送 `backend` / `frontend` 镜像到 GHCR，然后通过 SSH 登录服务器执行 `docker compose up -d --pull always`

默认镜像名：

- `ghcr.io/<owner>/rankflow-backend:<tag>`
- `ghcr.io/<owner>/rankflow-frontend:<tag>`

生产部署使用 `docker-compose.prod.yml`，只编排：

- `frontend`：对外提供 HTTP 访问
- `backend`：只在 Compose 内部暴露 `:8080`

MySQL 和 Redis 走外部服务，通过 GitHub Secrets 注入以下环境变量：

- `RANKFLOW_MYSQL_DSN`
- `RANKFLOW_REDIS_ADDR`
- `RANKFLOW_REDIS_PASSWORD`
- `RANKFLOW_REDIS_DB`
- `RANKFLOW_PERSIST_WORKERS`

部署前需要在 GitHub 仓库中配置这些 Secrets：

- `DEPLOY_HOST`
- `DEPLOY_PORT`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_KNOWN_HOSTS`
- `DEPLOY_PATH`
- `GHCR_USERNAME`
- `GHCR_PULL_TOKEN`
- `FRONTEND_PORT`
- 上述 `RANKFLOW_*` 配置项（其中 `RANKFLOW_MYSQL_DSN`、`RANKFLOW_REDIS_ADDR` 为必填）

服务器前置要求：

- 已安装 Docker Engine 和 Docker Compose Plugin
- 部署目录 `DEPLOY_PATH` 已存在且可写
- 服务器可访问 `ghcr.io`
- SSH 用户有执行 `docker compose` 的权限
- 手动触发 `CD` 时也需要从 `main` 分支发起，工作流会拒绝其他分支
