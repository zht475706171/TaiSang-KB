# TaiSang-KB Implementation Plan (Part 6: Dockerfile + full-stack compose + README)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Containerize backend + frontend, provide a single `docker compose up` experience, and write a clear README so a new user can clone and run.

**Architecture:** Multi-stage Dockerfile for the Go backend (build → minimal runtime image). Multi-stage Dockerfile for the frontend (build → nginx static). Compose file runs postgres + backend + frontend together. Backend serves `/api/*`; frontend (nginx) serves everything else and proxies `/api` to backend.

**Tech Stack:** Docker / docker compose / nginx / Go / Node 20.

**Spec ref:** §2 (部署：本地 / Docker), §10 (目录结构).

**Prerequisite:** Parts 1-5 complete.

---

### Task 1: Backend Dockerfile (multi-stage)

**Files:**
- Create: `docker/backend.Dockerfile`
- Create: `docker/backend.dockerignore`

- [ ] **Step 1: Create backend Dockerfile**

`docker/backend.Dockerfile`:
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache deps first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/server /app/server

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
```

- [ ] **Step 2: Create .dockerignore at repo root**

`.dockerignore`:
```
.git
.github
.idea
.vscode
bin/
frontend/node_modules/
frontend/dist/
storage/
*.log
.env
.env.local
docs/
testdata/
```

- [ ] **Step 3: Verify backend image builds**

```bash
cd D:/Project/TaiSang-KB
docker build -f docker/backend.Dockerfile -t taisang-kb-backend:dev .
```
Expected: image built. Run quick smoke:
```bash
docker run --rm -e TSK_DB_DSN=postgres://x -e TSK_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef taisang-kb-backend:dev
```
Expected: starts, logs config loaded, fails at DB connect (no postgres) — that's fine, proves binary runs.

- [ ] **Step 4: Commit**

```bash
git add docker/backend.Dockerfile .dockerignore
git commit -m "feat(docker): backend multi-stage Dockerfile (alpine, static binary)"
```

---

### Task 2: Frontend Dockerfile (build → nginx)

**Files:**
- Create: `docker/frontend.Dockerfile`
- Create: `docker/frontend.nginx.conf`

- [ ] **Step 1: Create nginx config**

`docker/frontend.nginx.conf`:
```nginx
server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API to backend service.
    location /api/ {
        proxy_pass http://backend:8080/api/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # SSE support
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 300s;
        chunked_transfer_encoding on;
    }
}
```

- [ ] **Step 2: Create frontend Dockerfile**

`docker/frontend.Dockerfile`:
```dockerfile
# Build stage
FROM node:20-alpine AS builder

WORKDIR /src
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# Runtime stage
FROM nginx:1.27-alpine

COPY --from=builder /src/dist /usr/share/nginx/html
COPY docker/frontend.nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
```

- [ ] **Step 3: Verify frontend image builds**

```bash
cd D:/Project/TaiSang-KB
docker build -f docker/frontend.Dockerfile -t taisang-kb-frontend:dev .
```
Expected: image built (build takes 1-2 min).

- [ ] **Step 4: Commit**

```bash
git add docker/frontend.Dockerfile docker/frontend.nginx.conf
git commit -m "feat(docker): frontend multi-stage Dockerfile (vite build → nginx)"
```

---

### Task 3: Full-stack docker compose

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] **Step 1: Replace docker-compose.yml**

```yaml
services:
  postgres:
    image: pgvector/pgvector:pg14
    container_name: tsk-postgres
    environment:
      POSTGRES_USER: tsk
      POSTGRES_PASSWORD: tsk
      POSTGRES_DB: taisang
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - tsk_pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tsk -d taisang"]
      interval: 5s
      timeout: 3s
      retries: 10

  backend:
    build:
      context: .
      dockerfile: docker/backend.Dockerfile
    container_name: tsk-backend
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      TSK_DB_DSN: postgres://tsk:tsk@postgres:5432/taisang?sslmode=disable
      TSK_LISTEN_ADDR: 0.0.0.0:8080
      TSK_STORAGE_DIR: /data/storage
      TSK_ENCRYPTION_KEY: ${TSK_ENCRYPTION_KEY}
      TSK_EMBEDDING_DIM: 1536
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - tsk_storage:/data/storage
    restart: unless-stopped

  frontend:
    build:
      context: .
      dockerfile: docker/frontend.Dockerfile
    container_name: tsk-frontend
    depends_on:
      - backend
    ports:
      - "127.0.0.1:80:80"
    restart: unless-stopped

volumes:
  tsk_pg:
  tsk_storage:
```

- [ ] **Step 2: Update .env.example**

```bash
# === Required ===
# 32-char key for AES-256-GCM (used to encrypt API Key in settings table).
# Generate with: openssl rand -hex 16
# MUST be set before `docker compose up`.
TSK_ENCRYPTION_KEY=change_me_to_32_char_random_string

# === Optional (override for local dev without docker) ===
# TSK_DB_DSN=postgres://tsk:tsk@localhost:5432/taisang?sslmode=disable
# TSK_LISTEN_ADDR=127.0.0.1:8080
# TSK_STORAGE_DIR=./storage
# TSK_EMBEDDING_DIM=1536
```

- [ ] **Step 3: Verify compose config**

```bash
docker compose config
```
Expected: valid YAML, three services shown.

- [ ] **Step 4: Full stack smoke test**

```bash
cd D:/Project/TaiSang-KB
# Set a real 32-char key:
export TSK_ENCRYPTION_KEY=$(openssl rand -hex 16)
echo "TSK_ENCRYPTION_KEY=$TSK_ENCRYPTION_KEY" > .env
docker compose build
docker compose up -d
# Wait for healthy:
docker compose ps
# Visit http://127.0.0.1
```
Expected: frontend loads, can configure settings, create KB, upload, chat. All via port 80.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "feat(docker): full-stack compose (postgres+backend+frontend)"
```

---

### Task 4: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md**

```markdown
# TaiSang-KB

极简个人 / 小团队 RAG 知识库。上传文档 → 自动解析分块向量化 → 检索增强问答（RAG）。

## 特性

- 支持 Markdown / TXT / HTML / PDF / DOCX / XLSX / PPTX 上传
- 父子分块 + pgvector HNSW 余弦检索
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

# 生成 32 字符加密密钥（用于加密保存的 API Key）
echo "TSK_ENCRYPTION_KEY=$(openssl rand -hex 16)" > .env

docker compose up -d
```

启动后访问 `http://127.0.0.1`。

### 3. 配置模型

进入"设置"页：
- API Base URL：如 `https://api.openai.com` 或 `https://api.deepseek.com`
- API Key：你的 sk-...
- Chat 模型：如 `gpt-4o-mini` / `deepseek-chat`
- Embedding 模型：如 `text-embedding-3-small` / `text-embedding-v3`

### 4. 创建知识库并上传文档

进入"知识库"页 → 新建 → 点"管理文档" → 上传文件。状态会从 `等待` → `解析中` → `完成`。

### 5. 问答

进入"问答"页 → 选择知识库 → 提问。回答会流式输出，下方可展开引用来源。

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

前端跑在 `http://127.0.0.1:5173`，自动代理 `/api` 到 `:8080`。

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
│   ├── service/           业务逻辑（kb/document/chat/setting/chunker）
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
├── docs/superpowers/      设计文档与实现计划
└── Makefile
```

## 安全提示

- 默认绑 `127.0.0.1`，不要直接暴露到公网。
- 如需公网访问，请自行加反向代理 + 鉴权。
- API Key 在 DB 中以 AES-256-GCM 加密存储，响应中脱敏显示 `****后4位`。

## 限制

- v1 仅做 dense 检索（pgvector 余弦），未接 sparse（PG 中文分词扩展）。
- 不支持扫描件 PDF / 图片 OCR。
- 单进程解析 worker（个人用足够，高并发需引入 MQ）。
- 无鉴权、无多用户。

## License

私有项目。
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README with quickstart and project structure"
```

---

### Task 5: Final integration test + push to remote

- [ ] **Step 1: Full clean build**

```bash
cd D:/Project/TaiSang-KB
docker compose down -v
docker compose build --no-cache
docker compose up -d
docker compose ps  # all healthy
```

- [ ] **Step 2: Run all backend tests**

```bash
go test ./... -count=1 -short
go vet ./...
```
Expected: PASS / no issues.

- [ ] **Step 3: Run frontend type-check + build**

```bash
cd frontend
npm run type-check
npm run build
cd ..
```
Expected: both succeed.

- [ ] **Step 4: Manual E2E**

Visit `http://127.0.0.1`:
- Settings: save API key
- KB: create + upload markdown
- Chat: ask question, see streamed answer + citations

- [ ] **Step 5: Push to remote**

```bash
git remote -v  # confirm origin = git@github.com:zht475706171/TaiSang-KB.git
git branch -M main
git push -u origin main
```
Expected: pushes all commits to GitHub.

- [ ] **Step 6: Commit checkpoint**

```bash
git commit --allow-empty -m "checkpoint: Part 6 docker+readme complete, v1 shippable"
```

---

## End of Part 6

**What's done:**
- Backend Dockerfile (multi-stage, alpine, static binary, non-root)
- Frontend Dockerfile (vite build → nginx, SSE-aware proxy)
- Full-stack docker compose (postgres + backend + frontend)
- README with quickstart, dev guide, structure, security notes, limitations
- Pushed to `git@github.com:zht475706171/TaiSang-KB.git`

**v1 is shippable.** Known gaps to address in v1.1+:
- Sparse retrieval (PG 中文分词扩展)
- Session listing/messages endpoints (currently stubs)
- Embedding retry loop startup in main.go
- OCR / scanned PDF support
- Multi-user + auth (if ever needed)

---

## End of all parts (1-6)

Total: 6 plan files covering foundation → parser/chunker → LLM clients → services/handlers/worker → frontend → docker/readme.

Recommended execution: Subagent-Driven Development, dispatching one task at a time across all 6 plans sequentially. Review between tasks.