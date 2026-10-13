# API 说明（节选）

Base: `/api/v1`  
响应：`{ "code": 0, "message": "ok", "data": ... }`

## Auth / Config

- `POST /auth/wechat` body: `{ "code": "...", "nickname?": "", "avatar_url?": "" }`
- `POST /auth/refresh` body: `{ "refresh_token": "..." }`
- `GET /me` Header: `Authorization: Bearer <access>`
- `PATCH /me` body: `{ "nickname?", "avatar_url?" }`
- `GET /config/client` → `{ "todo_template_id", "affairs_template_id", "wx_mock" }`
- `GET /me/subscribe-prefs` → `{ "todo": "accept|reject|", "affairs": "..." }`
- `POST /me/subscribe-prefs` body: `{ "todo?", "affairs?" }`

## Announcements

- `GET /announcements?type=civil|institution&region=&keyword=&published_from=YYYY-MM-DD&published_to=YYYY-MM-DD&page=&page_size=`
- `GET /announcements/:id`

## Me exams / todos

- `GET /me/exams`
- `POST /me/exams` `{ "announcement_id": 1 }`
- `DELETE /me/exams/:announcementId`
- `GET /me/todos`（按公告分组）
- `PATCH /me/todos/:id` `{ "status": "done|cancelled|pending" }`

## Practice

- `GET /questions/categories`
- `POST /practice/sessions` `{ "category_id?", "mode": "sequential|random|wrong", "count?", "show_analysis?" }`
- `POST /practice/check` `{ "session_id", "question_id", "user_key" }` → 对错 + 解析
- `POST /practice/submit` `{ "session_id", "answers": [{ "question_id", "user_key" }] }`
- `GET /me/wrong-questions`
- `DELETE /me/wrong-questions/:questionId`
- `GET /me/practice/stats`

## Affairs

- `GET /affairs?tag=&keyword=&page=`
- `GET /affairs/:id`
- `GET /me/affairs/favorites`
- `POST /me/affairs/favorites` `{ "article_id" }`
- `DELETE /me/affairs/favorites/:id`

## AI 分析（需登录）

未配置 `AI_API_KEY` 时全部走规则引擎（`provider="rule"`）；大模型调用失败或超出 `AI_DAILY_LIMIT` 时自动回退规则引擎，并在 `notice` 中说明。

- `GET /ai/status` → `{ "llm_enabled", "model", "daily_limit", "used_today" }`
- `GET /me/ai/profile` / `POST /me/ai/profile` `{ "education?", "major?", "graduation_year?", "fresh_graduate?", "political_status?", "work_years?", "grassroots_years?", "certificates?", "target_regions?", "target_types?": "civil|institution", "exam_date?": "YYYY-MM-DD", "daily_hours?", "resume_text?" }`
- `DELETE /me/ai/profile`：删除简历、资料与全部 AI 记录
- `POST /ai/resume/analyze` `{ "resume_text?", "profile?": {同上} }` → `{ report_id, provider, notice?, profile, summary, strengths[], weaknesses[], directions[{name,reason}], recommendations[{announcement_id,title,type,region,register_end,exam_at,score,reasons[],cautions[]}], next_steps[] }`
- `POST /ai/study-plan` `{ "exam_date?", "daily_hours?", "focus?" }` → `{ report_id, provider, exam_date, exam_date_note?, days_left, daily_hours, summary, diagnosis[], phases[], daily_template[{module,minutes,task}], weekly[], tips[] }`
- `POST /ai/ask` `{ "question" }` → `{ report_id, provider, notice?, answer }`
- `GET /me/ai/reports?kind=resume|plan|ask&page=` / `GET /me/ai/reports/:id`（含 `result`）

## Admin

- `POST /admin/login` `{ "username", "password" }` → admin JWT
- `GET/POST /admin/announcements`，`PATCH /admin/announcements/:id`（改期会同步未提醒代办）
- `GET/POST /admin/categories`
- `GET/POST /admin/questions`，`PATCH/DELETE /admin/questions/:id`
- `POST /admin/questions/import` JSON `{ "items": [...] }` 或 `Content-Type: text/csv`（表头 `category,stem,answer_key,analysis,difficulty,tags,A,B,C,D`）
- `GET/POST /admin/affairs`，`PATCH /admin/affairs/:id`
- `GET /admin/crawl-failures`

## Crawler（Worker）

Worker 约每 30 分钟执行一次（`CRAWLER_ENABLED=true`）。内置源：

| 源 | 类型 | 地址 |
|---|---|---|
| `cpta` | 公告+时政 | 中国人事考试网首页链接 |
| `gov-zhengce` | 公告（招考关键词过滤） | `gov.cn/.../ZUIXINZHENGCE.json` |
| `xinhua-politics` / `people-politics` / `xinhua-legal` | 时政 RSS | 新华网 / 人民网 |

本地可手动：`cd server && go run ./cmd/crawlonce`。无外网数据时，`CRAWLER_SEED_FALLBACK=true` 会写入演示种子。
