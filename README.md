# 公考助手（gongkao）

微信小程序 + Go(Gin) + PostgreSQL + Redis。

**文档：** [项目文档](docs/项目文档.md)（总览）· [功能模块 doc/](doc/README.md) · [需求文档](docs/需求文档.md) · [开发方案](docs/开发方案.md) · [API](docs/API.md)

## 快速启动

### 1. 基础设施

推荐（Docker Desktop 需已启动）：

```bash
cd deploy
docker compose up -d postgres redis
```

将 `server/.env` 中 `DATABASE_URL` 改为：

`postgres://gongkao:gongkao@127.0.0.1:5432/gongkao?sslmode=disable`

本地无 Docker 时可直接用 SQLite（`.env.example` 默认）+ 本机 Redis：

```bash
# Windows 若已安装 Redis
# "C:\Program Files\Redis\redis-server.exe"
```

### 2. 后端 API

```bash
cd server
copy .env.example .env   # Windows
# 默认 WX_MOCK=true，无需真实 AppID
go mod tidy
go run ./cmd/api
```

Worker（采集 + 代办提醒）：

```bash
go run ./cmd/worker
```

一键启动（检测 Redis → 拉起 API + Worker + Web 预览）：

```bash
powershell -ExecutionPolicy Bypass -File scripts/start-dev.ps1
powershell -ExecutionPolicy Bypass -File scripts/smoke.ps1
```

健康检查：`GET http://127.0.0.1:8080/health`  
Web 预览：`http://127.0.0.1:5173/`

### 3. 小程序

用微信开发者工具打开 `miniapp/` 目录，关闭域名校验，确保 `app.js` 中 `baseURL` 指向本机 API。

## 模块概览

| 模块 | 说明 |
|---|---|
| 登录 | `POST /api/v1/auth/wechat`（MOCK 可用任意 code）；401 自动 refresh |
| 公告 | 定时采集：中国人事考试网 + 中国政府网政策（招考关键词）；支持筛选 |
| 我的考试 / 代办 | 自动代办；倒计时与 24h 高亮；订阅设置页；未授权不推送 |
| 刷题 | 顺序/随机、作答看解析、交卷、错题再练、统计 |
| 时政 | 定时采集：新华网/人民网 RSS；标签筛选、收藏；可选每日推送 |
| 管理 | `POST /api/v1/admin/login` 默认 `admin/admin123`；题目 CRUD / CSV 导入 |

完整说明见 [`docs/项目文档.md`](docs/项目文档.md)。
