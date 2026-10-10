# 公考助手（gongkao）

微信小程序 + Go(Gin) + PostgreSQL + Redis。文档见 `docs/`。

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
| 登录 | `POST /api/v1/auth/wechat`（MOCK 可用任意 code） |
| 公告 | 公务员 / 事业单位列表与详情，种子采集 |
| 我的考试 / 代办 | 加入后自动生成报名/确认/考试代办，到期前 10 分钟提醒 |
| 刷题 | 分类练习、交卷、错题本、统计 |
| 时政 | 列表、详情、收藏 |
| 管理 | `POST /api/v1/admin/login` 默认 `admin/admin123` |

更细的需求与方案：`docs/需求文档.md`、`docs/开发方案.md`。
