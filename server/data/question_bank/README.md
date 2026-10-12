# 刷题题库

种子数据：`questions.json`（约 98 题）；增量自有包：`open_pack.json`。可用 `python gen_bank.py` / `python gen_open_pack.py` 重新生成。

分类：行测-言语理解 / 数量关系 / 判断推理 / 资料分析；公基-政治理论 / 法律基础 / 人文常识 / 科技自然；时政常识。

合规：只接入自有/授权/公开练习题，不爬受版权保护的商业整库或真题整包。

## 采集（crawl → JSON → DB）

```bash
cd server
# 合并：本地题包 + 时政文章转题 + QUESTION_BANK_FEED_URLS
go run ./cmd/crawlbank
```

产出：`data/question_bank/crawled.json`，并 `ImportQuestionsSkipDup` 入库。Worker 在 `CRAWLER_QUESTION_BANK_ENABLED=true` 时每 30 分钟同步一次。

可选环境变量：`QUESTION_BANK_FEED_URLS`（授权 JSON URL）、`QUESTION_BANK_EXTRA_PACKS`（额外本地包）。

## 格式

JSON 数组，字段：

```json
{
  "category": "行测-言语理解",
  "stem": "题干",
  "answer_key": "A",
  "analysis": "解析",
  "difficulty": 1,
  "tags": "逻辑填空",
  "options": [
    {"key": "A", "content": "..."},
    {"key": "B", "content": "..."},
    {"key": "C", "content": "..."},
    {"key": "D", "content": "..."}
  ]
}
```

也可使用 CSV（表头）：

`category,stem,answer_key,analysis,difficulty,tags,A,B,C,D`

## 导入

```bash
cd server
go run ./cmd/seedbank -file data/question_bank/questions.json
# 或清空后重建（仅开发环境）
go run ./cmd/seedbank -file data/question_bank/questions.json -reset
```

管理 API：`POST /api/v1/admin/questions/import`（需 admin JWT）。
