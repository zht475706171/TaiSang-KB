# TaiSang-KB

极简个人 / 小团队 RAG 知识库。上传文档 → 自动解析分块向量化 → 检索增强问答（RAG）。

## 特性

- 支持 Markdown / TXT / HTML / PDF / DOCX / XLSX 上传
- 父子分块 + pgvector HNSW 余弦检索
- OpenAI 兼容 API（DeepSeek / Qwen / OpenAI / Ollama 兼容 等均可）
- 流式 SSE 回答 + 引用溯源
- TDesign Vue 前端,三页极简(问答 / 知识库 / 设置)
- 单机 Docker 部署,无鉴权(绑 127.0.0.1)

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
│   ├── service/           业务逻辑(kb/document/chat/setting/chunker)
│   ├── worker/            异步解析 worker
│   ├── infrastructure/
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

- v1 仅做 dense 检索(pgvector 余弦),未接 sparse(PG 中文分词扩展)。
- 不支持扫描件 PDF / 图片 OCR。
- 单进程解析 worker(个人用足够,高并发需引入 MQ)。
- 无鉴权、无多用户。

## License

私有项目。