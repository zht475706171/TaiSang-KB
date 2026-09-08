# TaiSang-KB

极简个人 / 小团队 RAG 知识库。上传文档 → 自动解析分块向量化 → 检索增强问答（RAG）。

## 当前版本与进度

**当前版本**：v1.1（切片增强，WeKnora chunker 移植 Option C）

### 已实现 ✅

**Part 1-6 v1 基线（40+ commit，已 push）**：
- Go 1.25 + Gin + GORM + pgvector + zap 后端
- Vue 3.5 + TDesign Vue Next + Vite 7 前端
- PostgreSQL 14 + pgvector HNSW 余弦检索
- 文档解析：Markdown / TXT / HTML / PDF / DOCX / XLSX（不支持 PPTX）
- 父子分块 + 流式 SSE 回答 + 引用溯源
- OpenAI 兼容 API（DeepSeek / Qwen / OpenAI / Ollama）
- AES-256-GCM 加密 API Key
- 单机 Docker 部署（postgres + backend + frontend）
- 三页极简前端：问答 / 知识库 / 设置

**v1.1 切片增强（7 commit，已 push）**：
- 移植 WeKnora chunker 包（`internal/infrastructure/chunker/`，5 策略：auto/heading/heuristic/recursive/legacy）
- 父子分块升级：4096/384/76（原 800/256/50）
- 受保护模式：LaTeX `$$`、fenced code、表格、图片、链接、inline code 不切断
- 语义重叠：段 > 行 > 句 3 级边界 + 4-rune 回扫
- ContextHeader：Markdown 标题面包屑折入 `parent_content` 列（不改 schema）
- 140 个 WeKnora 原生测试 + 7 个适配层测试全 PASS

### 未实现 / 待做 ❌（v1.2+）

**切片相关**（WeKnora 有，本阶段未移植）：
- per-KB 配置（`ChunkingConfig` 存 JSON 列，前端可配 chunk_size/overlap/parent_size/strategy/token_limit/languages）
- `SplitWithDiagnostics` 诊断端点 + 前端分块预览调试器
- `ExtractImageRefs` 图片引用提取
- `NormalizeLineEndings` 行尾归一化

**向量库相关**：
- halfvec 半精度向量（省一半存储）
- partial HNSW 多维度索引（支持多 embedding 模型共存）
- BM25 稀疏检索（ParadeDB + lindera 中文分词，dense + sparse 混合）
- 检索调优：`hnsw.ef_search` / `hnsw.iterative_scan`
- 批量入库 40/5 并发 + 指数退避重试

**功能相关**：
- session 列表 / 消息历史端点是 stub（未完整实现历史对话管理）
- embedding retry loop 启动未在 main.go 显式接线
- 扫描件 PDF / 图片 OCR
- 多用户 + 鉴权

**其他**：
- 前端无分块配置 UI（当前全局硬编码默认值）
- 前端无分块预览 / 调试器

## 特性

- 支持 Markdown / TXT / HTML / PDF / DOCX / XLSX 上传
- **5 策略自适应切片**（auto/heading/heuristic/recursive/legacy，移植自 WeKnora）+ 父子分块 4096/384/76
- **受保护模式**：LaTeX `$$`、fenced code、表格、图片、链接、inline code 不被切断
- **Markdown 标题面包屑**：子块携带所属标题上下文，折入 `parent_content`
- pgvector HNSW 余弦检索
- OpenAI 兼容 API（DeepSeek / Qwen / OpenAI / Ollama 兼容 等均可）
- 流式 SSE 回答 + 引用溯源
- TDesign Vue 前端，三页极简（问答 / 知识库 / 设置）
- 单机 Docker 部署，无鉴权（绑 127.0.0.1）

## 快速开始

### 1. 准备

需要 Docker 与 Docker Compose。

### 2. 启动

```bash
git clone git@github.com:zht475706171/TaiSang-KB.git
cd TaiSang-KB

# 生成 32 字符加密密钥(用于加密保存的 API Key)
echo "TSK_ENCRYPTION_KEY=$(openssl rand -hex 16)" > .env

docker compose up -d
```

启动后访问 `http://127.0.0.1:8081`。

### 3. 配置模型

进入"设置"页:
- API Base URL:如 `https://api.openai.com` 或 `https://api.deepseek.com`
- API Key:你的 sk-...
- Chat 模型:如 `gpt-4o-mini` / `deepseek-chat`
- Embedding 模型:如 `text-embedding-3-small` / `text-embedding-v3`

### 4. 创建知识库并上传文档

进入"知识库"页 → 新建 → 点"管理文档" → 上传文件。状态会从 `等待` → `解析中` → `完成`。

### 5. 问答

进入"问答"页 → 选择知识库 → 提问。回答会流式输出,下方可展开引用来源。

## 本地开发

### 后端

```bash
docker compose up -d postgres
cp .env.example .env  # 编辑 TSK_ENCRYPTION_KEY
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

前端跑在 `http://127.0.0.1:5173`,自动代理 `/api` 到 `:8080`。

## 项目结构

```
TaiSang-KB/
├── cmd/server/            程序入口
├── internal/
│   ├── config/            配置加载
│   ├── database/          DB 连接 + 迁移
│   ├── models/            GORM 实体
│   ├── router/            路由注册
│   ├── handler/           HTTP 处理器
│   ├── service/           业务逻辑(kb/document/chat/setting/chunker 适配层)
│   ├── worker/            异步解析 worker
│   ├── infrastructure/
│   │   ├── chunker/       WeKnora 切片包（5 策略 + 父子 + 受保护模式）
│   │   ├── postgres/      仓储实现
│   │   ├── llmclient/     OpenAI 兼容客户端
│   │   ├── parser/        文档解析器
│   │   ├── storage/       本地文件存储
│   │   └── crypto/        AES-256-GCM
│   └── logger/            zap 日志
├── frontend/              Vue 3 + TDesign 前端
├── docker/                Dockerfile 与 nginx 配置
├── docker-compose.yml
└── docs/superpowers/      设计文档与实现计划
```

## 安全提示

- 默认绑 `127.0.0.1`,不要直接暴露到公网。
- 如需公网访问,请自行加反向代理 + 鉴权。
- API Key 在 DB 中以 AES-256-GCM 加密存储,响应中脱敏显示 `****后4位`。

## 限制

- v1.1 仅做 dense 检索（pgvector 余弦），未接 sparse（BM25 + 中文分词）。
- 切片参数全局硬编码（4096/384/76），无 per-KB 配置 + 前端 UI。
- 不支持扫描件 PDF / 图片 OCR。
- 单进程解析 worker（个人用足够，高并发需引入 MQ）。
- session 历史端点是 stub，未完整实现对话管理。
- 无鉴权、无多用户。

## License

私有项目。