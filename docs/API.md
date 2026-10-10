# API 说明（节选）

Base: `/api/v1`  
响应：`{ "code": 0, "message": "ok", "data": ... }`

## Auth

- `POST /auth/wechat` body: `{ "code": "...", "nickname?": "", "avatar_url?": "" }`
- `POST /auth/refresh` body: `{ "refresh_token": "..." }`
- `GET /me` Header: `Authorization: Bearer <access>`
- `PATCH /me` body: `{ "nickname?", "avatar_url?" }`

## Announcements

- `GET /announcements?type=civil|institution&region=&keyword=&page=&page_size=`
- `GET /announcements/:id`

## Me exams / todos

- `GET /me/exams`
- `POST /me/exams` `{ "announcement_id": 1 }`
- `DELETE /me/exams/:announcementId`
- `GET /me/todos`（按公告分组）
- `PATCH /me/todos/:id` `{ "status": "done|cancelled|pending" }`

## Practice

- `GET /questions/categories`
- `POST /practice/sessions` `{ "category_id", "mode": "sequential|random", "count" }`
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

## Admin

- `POST /admin/login` `{ "username", "password" }` → admin JWT
- `GET/POST /admin/announcements`，`PATCH /admin/announcements/:id`
- `POST /admin/questions/import` `{ "items": [...] }`
- `POST /admin/affairs`，`PATCH /admin/affairs/:id`
- `GET /admin/crawl-failures`
