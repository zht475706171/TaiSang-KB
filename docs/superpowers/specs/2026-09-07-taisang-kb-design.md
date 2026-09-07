# TaiSang-KB 设计文档

- 日期：2026-09-07
- 作者：泰哥 + Claude
- 远程仓库：`git@github.com:zht475706171/TaiSang-KB.git`
- 参考实现：`D:\Project\WeKnora`（腾讯开源 RAG/Agent/Wiki 框架）

## 1. 项目定位

**极简个人 / 小团队知识库**：上传文档 → 解析分块 → 向量化 → RAG 问答。

明确不做：
- Agent / ReAct 推理 / 工具调用 / 沙箱
- Wiki 自动生成 / 知识图谱
- IM 集成（企业微信 / 飞书 / Slack 等）
- 多空间 RBAC / 多用户
- MCP Server / Chrome 插件 / 小程序
- 多向量库后端（Milvus / Qdrant 等，只用 pgvector）

## 2. 技术栈

| 层 | 选型 |
|----|------|
| 后端 | Go 1.22+ / Gin / GORM |
| 数据库 | PostgreSQL 14+ + pgvector 扩展 |
| 向量索引 | pgvector HNSW（余弦） |
| 文档解析 | Go 原生 + unioffice（Office）+ ledongthuc/pdf（PDF） |
| LLM / Embedding | OpenAI 兼容 API（DeepSeek / Qwen / OpenAI 均可） |
| 前端 | Vue 3 + TDesign Vue Next + Pinia + Vue Router + Vite + Less |
| Markdown 渲染 | marked + highlight.js + KaTeX（同 WeKnora） |
| SSE 客户端 | @microsoft/fetch-event-source |
| 部署 | docker compose（单机） |
| 鉴权 | 无（单用户本地，绑 127.0.0.1） |

选型理由：与 WeKnora 同栈，便于借鉴其 RAG 链路代码（`internal/searchutil`、`internal/application` 等）；TDesign 是腾讯自家组件库，视觉风格直接对齐 WeKnora。

## 3. 整体架构

```
┌──────────────────────────────────────────────────────┐
│  浏览器 (Vue 3 SPA)                                  │
│  ┌──────────┬─────────────┬──────────┐               │
│  │ 聊天页   │ KB 管理页   │ 设置页   │               │
│  └──────────┴─────────────┴──────────┘               │
└────────────────────┬─────────────────────────────────┘
                     │ HTTP / SSE (流式回答)
┌────────────────────▼─────────────────────────────────┐
│  Go 后端 (单进程, Gin)                                │
│  router ─ middleware ─ handler                       │
│    │                                                 │
│    ▼                                                 │
│  service 层:                                          │
│   ┌────────┬──────────┬──────────┬────────┐          │
│   │ kb     │ document │ chunk    │ chat   │          │
│   └────────┴──────────┴──────────┴────────┘          │
│    │                                                 │
│    ▼                                                 │
│  infrastructure:                                      │
│   postgres(pgvector) │ llmclient │ parser │ storage │
└────────────────────┬─────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
   Postgres     OpenAI兼容    本地文件
  (+pgvector)    API          存储桶
```

### 后端模块（internal/）

| 目录 | 职责 |
|------|------|
| `router` | 路由注册 |
| `handler` | HTTP/SSE 处理器（kb / document / chat / setting 四组） |
| `service` | 业务逻辑（kb / document / chunk / chat / embedding / rerank） |
| `infrastructure` | postgres 仓储 / llmclient / parser / storage |
| `models` | GORM 实体 |
| `config` | 配置加载（env → struct） |

### 关键依赖

- 后端：gin、gorm、pgvector-go、unioffice、ledongthuc/pdf、zap（或 logrus）
- 前端：vue 3、tdesign-vue-next、pinia、vue-router、vite、less、marked、highlight.js、katex、@microsoft/fetch-event-source、axios

## 4. 数据模型

PG 一库到底，`pgvector` 扩展存向量。GORM 实体 5 张表 + 1 张配置表。

### 4.1 knowledge_base

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | PK |
| name | string | KB 名称 |
| description | string | 描述 |
| embedding_model | string | 该 KB 用的 embedding 模型名 |
| created_at | timestamptz | |
| updated_at | timestamptz | |

### 4.2 document

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | PK |
| kb_id | uuid | FK → knowledge_base |
| title | string | 文档标题（默认文件名） |
| source | string | `upload` / `url`（第 1 期只做 upload） |
| file_path | string | 存储桶内路径（UUID 重命名） |
| mime_type | string | |
| status | string | `pending` / `parsing` / `done` / `failed` |
| error_msg | string | 失败原因（status=failed 时） |
| meta | jsonb | 原始文件名、大小、解析耗时等 |
| created_at | timestamptz | |

### 4.3 chunk

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | PK |
| doc_id | uuid | FK → document |
| content | text | 分块文本（子块） |
| parent_content | text | 所属父块文本（回答时喂 LLM） |
| chunk_index | int | 文档内序号 |
| token_count | int | |
| embedding | vector(1536) | 可 null（异步填充） |
| embedding_retry | bool | embedding 失败待补跑标记 |
| created_at | timestamptz | |

索引：
- `(doc_id)` btree
- `embedding` HNSW（`vector_cosine_ops`）
- `to_tsvector('chinese', content)` GIN（中文全文检索，依赖 pg_jieba / zhparser 扩展，见 §7）

### 4.4 chat_session / chat_message

chat_session：id / title / kb_id (nullable) / created_at / updated_at

chat_message：id / session_id (fk) / role (`user` / `assistant`) / content / citations (jsonb) / created_at

citations 结构：`[{chunk_id, doc_id, doc_title, content_snippet, score}]`

### 4.5 setting（单行）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 固定 = 1 |
| api_base_url | string | OpenAI 兼容 endpoint |
| api_key | string | AES-256-GCM 加密存储 |
| chat_model | string | |
| embedding_model | string | |
| rerank_enabled | bool | |
| top_k | int | 喂 LLM 的检索条数，默认 4 |

### 4.6 设计要点

1. **向量维度固定 1536**：OpenAI 兼容接口主流维度，换模型重建索引即可，不做可变维度（YAGNI）。
2. **document.status 状态机**：`pending → parsing → done / failed`，解析异步，单独 goroutine worker，不引入 MQ。
3. **chunk.embedding 允许 null**：解析分块后先入库，再批量调 embedding API 填充；失败可重试。
4. **citations jsonb**：RAG 引用溯源，前端渲染成可点击浮层（对齐 WeKnora）。
5. **setting 单行**：无鉴权场景下，配置走 .env 启动时入库一次，之后前端设置页可改。API Key 用 AES-256-GCM 加密（密钥从 .env 来）。
6. **不引入 user 表**：无鉴权，chat_session 不绑用户。

## 5. 组件清单

| 组件 | 职责 | 关键依赖 |
|------|------|---------|
| `kbService` | KB CRUD | gorm |
| `documentService` | 上传、状态流转、重解析 | parser, storage, chunkService |
| `parser` | MD/TXT/HTML/PDF/DOCX/XLSX/PPTX → 纯文本 | Go 库 |
| `chunker` | 文本 → 父子分块 + 滑窗 | — |
| `embeddingClient` | 调 OpenAI 兼容 embedding API | net/http |
| `chatService` | RAG 主链路：检索 → 重排 → 拼 prompt → LLM 流式 | embedding, rerank, llm |
| `reranker`（可选） | 调 rerank API 重排 top-K | net/http |
| `llmClient` | OpenAI 兼容 chat completions（stream） | net/http |
| `chatRepo` | 会话/消息持久化 | gorm |
| `storage` | 上传文件落本地桶 | os |

依赖方向单一：handler → service → infrastructure，无环。

## 6. 核心数据流

### 6.1 上传文档 → 解析 → 分块 → 向量化

```
浏览器                handler          documentService        parser/chunker
  │  POST /api/kb/{id}/documents (multipart)                    │
  │ ─────────────────► │                                       │
  │                     │ 落盘到 storage                        │
  │                     │ INSERT document (status=pending)      │
  │                     │ ─────────────► │                      │
  │ ◄─── 202 doc_id ───│                                       │
  │                                                            │
  │              后台 worker (goroutine, 单进程内)              │
  │                     │ UPDATE status=parsing                │
  │                     │ parser.Parse(file) → text            │
  │                     │ chunker.Split(text) → []chunk        │
  │                     │ INSERT chunks (embedding=null)       │
  │                     │ embeddingClient.EmbedBatch(chunks)   │
  │                     │ UPDATE chunks.embedding              │
  │                     │ UPDATE document.status=done          │
  │                     │ (任一步失败 → status=failed, 记录错误)│
  │                                                            │
  │  GET /api/documents/{id} (轮询状态)                         │
  │ ◄─── {status, chunk_count} ──                              │
```

要点：
1. 同步返 202，前端拿 `document_id` 轮询状态；不引入 SSE 推送进度（YAGNI）。
2. worker 用单 goroutine + channel 队列，进程内即可，不引入 Redis/MQ。重启时扫 `status in (pending, parsing)` 的文档重新排队，parsing 状态的先清理已写入 chunk 再重新解析。
3. **分块策略**：父子分块（参考 WeKnora searchutil）—— 先按段落/标题切大块（父，~800 token），再在父块内滑窗切小块（子，~256 token，overlap 50）。检索用子块命中，回答时把父块作为上下文喂 LLM。
4. **Embedding 批量**：一次 64 条 chunk，失败重试 3 次（指数退避 1s/2s/4s）；部分失败时成功的先落库，失败的标 `embedding_retry=true` 留待补跑。
5. **Office 解析**：DOCX/XLSX/PPTX 用 `unioffice`；PDF 用 `ledongthuc/pdf` 提取文本层；扫描件 PDF 暂不支持。

### 6.2 RAG 问答（流式）

```
浏览器(SSE)           handler          chatService
  │  POST /api/chat (kb_id, question, session_id?)  │
  │ ─────────────────► │                          │
  │                     │ 1. embeddingClient.Embed(question) → q_vec
  │                     │ 2. chatRepo 拉历史消息（多轮，最近 6 条）
  │                     │ 3. 检索（两路并行）：
  │                     │    a) Dense: pgvector 余弦 top-K(=20)
  │                     │    b) Sparse: PG ts_vector 全文检索 top-K(=20)
  │                     │ 4. 合并去重 → reranker.Rerank → top_k(=4)
  │                     │    (rerank 关闭则直接取 dense 分数 top_k)
  │                     │ 5. 拼 prompt: system + retrieved父块 + 历史消息 + question
  │                     │ 6. llmClient.StreamChat → token 流
  │                     │    每个 token: SSE event: data: {delta}
  │                     │ 7. 流结束: INSERT chat_message(user+assistant)
  │                     │           SSE event: {citations: [...]}
  │                     │
  │ ◄─── SSE stream ────│                          │
  │   data:{delta}...                              │
  │   data:{citations}                             │
  │   data:[DONE]                                  │
```

要点：
1. **混合检索**：dense + sparse 都在 PG 内完成。sparse 用 `to_tsvector('chinese', content)` + `ts_rank`。中文分词需 `pg_jieba` 或 `zhparser` 扩展（见 §7）。无该扩展时降级为纯 dense。
2. **多轮上下文**：取最近 6 条消息（3 轮）拼进 prompt，超过则截断。
3. **流式协议**：SSE，`Content-Type: text/event-stream`，事件格式参考 OpenAI `data: {json}\n\n`，末尾 `data: [DONE]`。前端用 `@microsoft/fetch-event-source`。
4. **引用溯源**：检索命中的 4 个父块带 `doc_title` + `chunk_id` + `score`，随最后一帧发给前端。
5. **错误处理**：embedding 失败 → 503；检索为空 → 走"未找到相关内容"兜底回复（不调 LLM，省钱）；LLM 流式失败 → SSE 发 error 事件，已落库部分消息保留。

## 7. 错误处理与边界

### 7.1 错误分类

| 错误类型 | 来源 | 处理 | 用户感知 |
|---------|------|------|---------|
| 配置缺失 | 启动时 .env 校验 | 拒绝启动，日志明确 | 启动日志 |
| DB 连接失败 | GORM Open | 启动失败退出；运行时断连 GORM 自动重连 | 5xx |
| 文件超限 | multipart | 单文件 50MB，超限 413 | Toast |
| 不支持类型 | parser | 400 + 支持列表 | 上传时前端预筛 |
| 解析失败 | parser | status=failed，error_msg 存 meta | 列表失败标记 + 可重试 |
| Embedding 失败 | llmclient | 批量重试 3 次（1s/2s/4s 指数退避）；仍失败标 `embedding_retry=true`，后台每 10 分钟补跑 | 不阻塞文档状态 |
| LLM 调用失败 | chatService | 非 200 → 502；超时 60s → 504；流式中断 → SSE error | "回答中断，请重试" |
| 检索为空 | chatService | 不调 LLM，返回兜底文案 | 正常消息展示 |
| KB 不存在 | kbService | 404 | Toast |
| 参数非法 | handler | Gin binding 失败 → 400 | 表单内联错误 |

### 7.2 边界

1. **并发**：解析 worker 单 goroutine；聊天请求无并发限制（个人用），后续可加信号量限到 10。
2. **资源限额**：单文件 50MB；单 KB 文档数无硬上限；chunk 数无硬上限（HNSW 检索 O(log N)）；聊天历史每 session 取最近 6 条。
3. **幂等**：上传不去重（同文件名建新 document）；worker 重启扫 pending/parsing 重排队，parsing 的先清旧 chunk；重解析先删旧 chunk 再走流程。
4. **安全**：无鉴权，绑 `127.0.0.1:8080`；API Key 加密存储 + 响应脱敏；全参数化查询；文件名 sanitize + UUID 重命名；URL 导入第 1 期不做，避免 SSRF。
5. **可观测性**：zap 结构化日志 JSON 格式写 stdout；关键埋点（上传/解析/失败/embedding 重试/LLM 耗时 token）；不引入 Langfuse。
6. **数据迁移**：GORM AutoMigrate；pgvector 索引单独 `CREATE INDEX IF NOT EXISTS`。

### 7.3 隐藏依赖（实现阶段需确认）

- **PG 中文分词扩展**：sparse 检索依赖 `pg_jieba` 或 `zhparser`。需确认基础 PG Docker 镜像是否带，不带则：
  - 方案 a：用带扩展的镜像（如 `pgvector/pgvector:pg14` + 手动装 zhparser）
  - 方案 b：降级为纯 dense 检索（v1 先不做 sparse）
  - 推荐：v1 先纯 dense，v2 再加 sparse。**实现阶段需确认是否接受此降级。**

## 8. 测试策略

### 8.1 分层

| 层级 | 范围 | 工具 | 目标覆盖 |
|------|------|------|---------|
| 单元 | service / infra 纯逻辑 | `go test` | 核心 70%+ |
| 集成 | 仓储 + 真实 PG | `go test` + testcontainers-go | 仓储 80%+ |
| API | handler 全链路 | `httptest` + Gin | 核心端点 100% |
| 前端 | 组件 + store | Vitest + Vue Test Utils | 关键组件有覆盖 |
| E2E | 不做 | — | YAGNI |

### 8.2 重点测试点

后端：
1. `chunker.Split` —— 父子分块边界：空文本、超长段落、跨段落滑窗、CJK/英文混合
2. `parser.Parse` —— 各格式 fixture：MD/TXT/HTML/PDF/DOCX/XLSX/PPTX 各一份样例
3. `embeddingClient.EmbedBatch` —— mock HTTP，验证批量分片、重试、部分成功
4. `chatService.Chat` —— mock embedding + llm，验证检索→重排→prompt 拼装→流式→citations 落库
5. `documentService` 状态机 —— pending→parsing→done/failed 流转，worker 重启回归
6. 仓储层 —— CRUD + pgvector 余弦检索（testcontainers 起 PG + pgvector）
7. handler —— 上传、列表、聊天 SSE 流（httptest 录制 SSE 帧断言）

前端：
1. 聊天页 SSE 解析 + Markdown 渲染
2. KB 管理页上传/状态轮询
3. 设置页表单校验

### 8.3 fixture

- `testdata/` 放各格式样例（每格式一份小文件 <100KB）
- 集成测试 testcontainers 起 PG 14 + pgvector，结束销毁
- LLM/Embedding 全 mock，不打真实 API

### 8.4 验证命令

```bash
go test ./... -count=1
go test -race ./internal/service/...
cd frontend && npm run test
cd frontend && npm run type-check
```

## 9. API 端点清单

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/kb` | 创建 KB |
| GET | `/api/kb` | 列出 KB |
| GET | `/api/kb/{id}` | KB 详情 |
| PUT | `/api/kb/{id}` | 更新 KB |
| DELETE | `/api/kb/{id}` | 删除 KB（连带文档分块） |
| POST | `/api/kb/{id}/documents` | 上传文档（multipart） |
| GET | `/api/kb/{id}/documents` | 列出 KB 下文档 |
| GET | `/api/documents/{id}` | 文档详情（含状态） |
| DELETE | `/api/documents/{id}` | 删除文档 |
| POST | `/api/documents/{id}/reparse` | 重新解析 |
| POST | `/api/chat` | 问答（SSE 流） |
| GET | `/api/sessions` | 列出会话 |
| GET | `/api/sessions/{id}/messages` | 会话消息历史 |
| DELETE | `/api/sessions/{id}` | 删除会话 |
| GET | `/api/setting` | 获取设置（API Key 脱敏） |
| PUT | `/api/setting` | 更新设置 |
| GET | `/api/health` | 健康检查 |

## 10. 目录结构（实现阶段参考）

```
TaiSang-KB/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── router/
│   ├── handler/
│   │   ├── kb.go
│   │   ├── document.go
│   │   ├── chat.go
│   │   └── setting.go
│   ├── service/
│   │   ├── kb.go
│   │   ├── document.go
│   │   ├── chunk.go
│   │   ├── chat.go
│   │   ├── embedding.go
│   │   └── rerank.go
│   ├── infrastructure/
│   │   ├── postgres/
│   │   │   ├── repo_kb.go
│   │   │   ├── repo_document.go
│   │   │   ├── repo_chunk.go
│   │   │   └── repo_chat.go
│   │   ├── llmclient/
│   │   ├── parser/
│   │   └── storage/
│   ├── models/
│   └── middleware/
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── views/
│   │   │   ├── Chat.vue
│   │   │   ├── KnowledgeBases.vue
│   │   │   └── Settings.vue
│   │   ├── components/
│   │   ├── stores/
│   │   ├── router/
│   │   └── assets/
│   ├── package.json
│   └── vite.config.ts
├── docker/
│   └── Dockerfile
├── docker-compose.yml
├── .env.example
├── go.mod
└── Makefile
```