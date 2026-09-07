# TaiSang-KB Implementation Plan (Part 5: Frontend — Vue 3 + TDesign)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Vue 3 SPA with three pages (Chat, Knowledge Bases, Settings) reusing WeKnora's visual style (TDesign Vue Next).

**Architecture:** Vite + Vue 3 + Pinia + Vue Router. Axios for REST, `@microsoft/fetch-event-source` for SSE chat. Markdown rendering with marked + highlight.js + KaTeX. Three routes: `/chat`, `/kb`, `/settings`. App shell with TDesign `t-layout` + sidebar nav.

**Tech Stack:** Vue 3.5 / Vite 7 / TDesign Vue Next 1.19 / Pinia 3 / Vue Router 4 / Less / axios / @microsoft/fetch-event-source / marked 17 / highlight.js 11 / KaTeX 0.16.

**Spec ref:** §2 (前端栈), §6.2 (SSE 协议), §9 (API 端点).

**Prerequisite:** Part 4 complete (backend APIs available).

---

### Task 1: Scaffold Vite + Vue 3 project

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/index.html`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.app.json`
- Create: `frontend/tsconfig.node.json`
- Create: `frontend/env.d.ts`
- Create: `frontend/src/main.ts`
- Create: `frontend/src/App.vue`

- [ ] **Step 1: Create directory and package.json**

```bash
cd D:/Project/TaiSang-KB
mkdir -p frontend/src
```

`frontend/package.json`:
```json
{
  "name": "taisang-kb-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "type-check": "vue-tsc --build"
  },
  "dependencies": {
    "@microsoft/fetch-event-source": "^2.0.1",
    "axios": "^1.16.0",
    "dompurify": "^3.4.11",
    "highlight.js": "^11.11.1",
    "katex": "^0.16.45",
    "marked": "^17.0.5",
    "marked-katex-extension": "^5.1.8",
    "pinia": "^3.0.4",
    "tdesign-icons-vue-next": "0.4.4",
    "tdesign-vue-next": "^1.19.2",
    "vue": "^3.5.34",
    "vue-router": "^4.5.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "6.0.6",
    "@vue/tsconfig": "^0.9.1",
    "less": "^4.6.4",
    "typescript": "~5.6.0",
    "vite": "^7.3.5",
    "vue-tsc": "^3.2.8"
  }
}
```

- [ ] **Step 2: Create vite.config.ts**

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
```

- [ ] **Step 3: Create index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>TaiSang-KB</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
```

- [ ] **Step 4: Create tsconfig files**

`frontend/tsconfig.json`:
```json
{
  "files": [],
  "references": [
    { "path": "./tsconfig.app.json" },
    { "path": "./tsconfig.node.json" }
  ]
}
```

`frontend/tsconfig.app.json`:
```json
{
  "extends": "@vue/tsconfig/tsconfig.dom.json",
  "compilerOptions": {
    "composite": true,
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src/**/*.ts", "src/**/*.tsx", "src/**/*.vue", "env.d.ts"]
}
```

`frontend/tsconfig.node.json`:
```json
{
  "extends": "@tsconfig/node22/tsconfig.json",
  "compilerOptions": {
    "composite": true,
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "types": ["node"]
  },
  "include": ["vite.config.*"]
}
```
Add `@tsconfig/node22` to devDeps in package.json: `"@tsconfig/node22": "^22.14.0"`.

- [ ] **Step 5: Create env.d.ts**

```ts
/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
```

- [ ] **Step 6: Create main.ts**

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'
import 'katex/dist/katex.min.css'
import 'highlight.js/styles/github.css'
import App from './App.vue'
import router from './router'
import './assets/main.less'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(TDesign)
app.mount('#app')
```

- [ ] **Step 7: Create App.vue**

```vue
<template>
  <router-view />
</template>

<script setup lang="ts">
</script>
```

- [ ] **Step 8: Create assets/main.less**

```less
html, body, #app {
  height: 100%;
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  color: #181818;
  background: #f5f7fa;
}
* { box-sizing: border-box; }
```

- [ ] **Step 9: Install deps and verify dev server**

```bash
cd frontend
npm install
npm run dev
```
Expected: Vite dev server runs on `http://127.0.0.1:5173`. Browser shows blank page (no router yet). Ctrl+C.

- [ ] **Step 10: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/
git commit -m "feat(frontend): scaffold Vite + Vue 3 + TDesign + Pinia"
```

---

### Task 2: Router + app shell layout

**Files:**
- Create: `frontend/src/router/index.ts`
- Create: `frontend/src/views/Chat.vue` (placeholder)
- Create: `frontend/src/views/KnowledgeBases.vue` (placeholder)
- Create: `frontend/src/views/Settings.vue` (placeholder)
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Create router**

`frontend/src/router/index.ts`:
```ts
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/chat' },
    { path: '/chat', name: 'chat', component: () => import('@/views/Chat.vue') },
    { path: '/kb', name: 'kb', component: () => import('@/views/KnowledgeBases.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/Settings.vue') },
  ],
})

export default router
```

- [ ] **Step 2: Update App.vue with layout**

```vue
<template>
  <t-layout>
    <t-aside style="width: 220px; background: #1f2b3a; color: #fff;">
      <div class="logo">TaiSang-KB</div>
      <t-menu :value="activeMenu" @change="onMenuChange" theme="dark">
        <t-menu-item value="chat">
          <template #icon><t-icon name="chat" /></template>
          问答
        </t-menu-item>
        <t-menu-item value="kb">
          <template #icon><t-icon name="folder" /></template>
          知识库
        </t-menu-item>
        <t-menu-item value="settings">
          <template #icon><t-icon name="setting" /></template>
          设置
        </t-menu-item>
      </t-menu>
    </t-aside>
    <t-content>
      <router-view />
    </t-content>
  </t-layout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const activeMenu = computed(() => route.name as string)
function onMenuChange(value: string) {
  router.push({ name: value })
}
</script>

<style scoped lang="less">
.logo {
  height: 56px;
  line-height: 56px;
  text-align: center;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 1px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
:deep(.t-layout__content) { padding: 0; height: 100vh; overflow: auto; }
:deep(.t-menu__item) { color: #c7d0e0; }
</style>
```

- [ ] **Step 3: Create placeholder views**

`frontend/src/views/Chat.vue`:
```vue
<template>
  <div style="padding: 24px;">
    <h2>问答</h2>
    <p>待实现</p>
  </div>
</template>
```

`frontend/src/views/KnowledgeBases.vue`:
```vue
<template>
  <div style="padding: 24px;">
    <h2>知识库</h2>
    <p>待实现</p>
  </div>
</template>
```

`frontend/src/views/Settings.vue`:
```vue
<template>
  <div style="padding: 24px;">
    <h2>设置</h2>
    <p>待实现</p>
  </div>
</template>
```

- [ ] **Step 4: Verify dev server**

```bash
cd frontend
npm run dev
```
Expected: `http://127.0.0.1:5173` shows sidebar with three menu items, clicking switches route. Ctrl+C.

- [ ] **Step 5: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/src/
git commit -m "feat(frontend): router + app shell with TDesign sidebar"
```

---

### Task 3: API client + types

**Files:**
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/kb.ts`
- Create: `frontend/src/api/document.ts`
- Create: `frontend/src/api/chat.ts`
- Create: `frontend/src/api/setting.ts`
- Create: `frontend/src/types/index.ts`

- [ ] **Step 1: Create types**

`frontend/src/types/index.ts`:
```ts
export interface KnowledgeBase {
  id: string
  name: string
  description: string
  embedding_model: string
  created_at: string
  updated_at: string
}

export interface Document {
  id: string
  kb_id: string
  title: string
  source: string
  file_path: string
  mime_type: string
  status: 'pending' | 'parsing' | 'done' | 'failed'
  error_msg: string
  meta: Record<string, any>
  created_at: string
}

export interface Citation {
  chunk_id: string
  doc_id: string
  doc_title: string
  content_snippet: string
  score: number
}

export interface Setting {
  id: number
  api_base_url: string
  api_key: string
  chat_model: string
  embedding_model: string
  rerank_enabled: boolean
  top_k: number
}
```

- [ ] **Step 2: Create API client**

`frontend/src/api/client.ts`:
```ts
import axios from 'axios'

export const http = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

http.interceptors.response.use(
  (resp) => resp,
  (err) => {
    const msg = err?.response?.data?.error || err.message || '请求失败'
    return Promise.reject(new Error(msg))
  }
)
```

- [ ] **Step 3: Create API modules**

`frontend/src/api/kb.ts`:
```ts
import { http } from './client'
import type { KnowledgeBase } from '@/types'

export const kbApi = {
  list: () => http.get<{ items: KnowledgeBase[] }>('/kb').then(r => r.data.items),
  get: (id: string) => http.get<KnowledgeBase>(`/kb/${id}`).then(r => r.data),
  create: (data: { name: string; description?: string; embedding_model?: string }) =>
    http.post<KnowledgeBase>('/kb', data).then(r => r.data),
  update: (id: string, data: Partial<KnowledgeBase>) =>
    http.put<KnowledgeBase>(`/kb/${id}`, data).then(r => r.data),
  delete: (id: string) => http.delete(`/kb/${id}`),
}
```

`frontend/src/api/document.ts`:
```ts
import { http } from './client'
import type { Document } from '@/types'

export const documentApi = {
  list: (kbId: string) => http.get<{ items: Document[] }>(`/kb/${kbId}/documents`).then(r => r.data.items),
  get: (id: string) => http.get<Document>(`/documents/${id}`).then(r => r.data),
  delete: (id: string) => http.delete(`/documents/${id}`),
  reparse: (id: string) => http.post(`/documents/${id}/reparse`),
  upload: (kbId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<Document>(`/kb/${kbId}/documents`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then(r => r.data)
  },
}
```

`frontend/src/api/chat.ts`:
```ts
import { fetchEventSource } from '@microsoft/fetch-event-source'
import type { Citation } from '@/types'

export interface ChatCallbacks {
  onDelta: (delta: string) => void
  onCitations: (citations: Citation[]) => void
  onError: (err: string) => void
  onDone: () => void
}

export const chatApi = {
  stream: async (kbId: string, question: string, sessionId: string | null, cb: ChatCallbacks) => {
    const ctrl = new AbortController()
    await fetchEventSource('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ kb_id: kbId, question, session_id: sessionId || '' }),
      signal: ctrl.signal,
      onmessage(ev) {
        if (ev.data === '[DONE]') { cb.onDone(); return }
        try {
          const payload = JSON.parse(ev.data)
          if (payload.error) { cb.onError(payload.error); return }
          if (payload.delta) { cb.onDelta(payload.delta); return }
          if (payload.citations) { cb.onCitations(payload.citations as Citation[]); return }
        } catch (e) {
          // ignore malformed
        }
      },
      onerror(err) {
        cb.onError(String(err))
        throw err
      },
    })
    return ctrl
  },
}
```

`frontend/src/api/setting.ts`:
```ts
import { http } from './client'
import type { Setting } from '@/types'

export const settingApi = {
  get: () => http.get<Setting>('/setting').then(r => r.data),
  update: (data: Partial<Setting>) => http.put('/setting', data).then(r => r.data),
}
```

- [ ] **Step 4: Verify type-check**

```bash
cd frontend
npm run type-check
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/src/api frontend/src/types
git commit -m "feat(frontend): api client + types for kb/document/chat/setting"
```

---

### Task 4: Settings page

**Files:**
- Modify: `frontend/src/views/Settings.vue`

- [ ] **Step 1: Implement Settings.vue**

```vue
<template>
  <div class="page">
    <t-card title="模型与 API 设置" class="card">
      <t-form :data="form" :rules="rules" @submit="onSubmit" label-width="160px">
        <t-form-item label="API Base URL" name="api_base_url">
          <t-input v-model="form.api_base_url" placeholder="https://api.openai.com" />
        </t-form-item>
        <t-form-item label="API Key" name="api_key">
          <t-input v-model="form.api_key" placeholder="sk-..." type="password" />
          <div class="hint">已保存的 Key 显示为 ****后4位，留空则不变。</div>
        </t-form-item>
        <t-form-item label="Chat 模型" name="chat_model">
          <t-input v-model="form.chat_model" placeholder="gpt-4o-mini" />
        </t-form-item>
        <t-form-item label="Embedding 模型" name="embedding_model">
          <t-input v-model="form.embedding_model" placeholder="text-embedding-3-small" />
        </t-form-item>
        <t-form-item label="Top K" name="top_k">
          <t-input-number v-model="form.top_k" :min="1" :max="20" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" type="submit" :loading="saving">保存</t-button>
        </t-form-item>
      </t-form>
    </t-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { settingApi } from '@/api/setting'
import type { Setting } from '@/types'

const form = reactive<Setting>({
  id: 1, api_base_url: '', api_key: '', chat_model: '',
  embedding_model: '', rerank_enabled: false, top_k: 4,
})
const saving = ref(false)
const rules = {
  api_base_url: [{ required: true, message: '必填' }],
  chat_model: [{ required: true, message: '必填' }],
  embedding_model: [{ required: true, message: '必填' }],
}

onMounted(async () => {
  try {
    const s = await settingApi.get()
    Object.assign(form, s)
  } catch (e: any) {
    MessagePlugin.error(e.message)
  }
})

async function onSubmit({ validateResult }: any) {
  if (validateResult !== true) return
  saving.value = true
  try {
    await settingApi.update(form)
    MessagePlugin.success('已保存')
    // Reload to show masked key.
    const s = await settingApi.get()
    Object.assign(form, s)
  } catch (e: any) {
    MessagePlugin.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="less">
.page { padding: 24px; }
.card { max-width: 720px; }
.hint { color: #888; font-size: 12px; margin-top: 4px; }
</style>
```

- [ ] **Step 2: Verify dev server with backend**

Start backend:
```bash
cd D:/Project/TaiSang-KB
docker compose up -d postgres
cp .env.example .env
# Edit .env
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```
Start frontend in another terminal:
```bash
cd D:/Project/TaiSang-KB/frontend
npm run dev
```
Visit `http://127.0.0.1:5173/settings`, fill form, save. Expected: success toast.

- [ ] **Step 3: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/src/views/Settings.vue
git commit -m "feat(frontend): settings page with api key/model config"
```

---

### Task 5: Knowledge Bases page

**Files:**
- Modify: `frontend/src/views/KnowledgeBases.vue`

- [ ] **Step 1: Implement KnowledgeBases.vue**

```vue
<template>
  <div class="page">
    <div class="header">
      <h2>知识库</h2>
      <t-button theme="primary" @click="openCreate">
        <template #icon><t-icon name="add" /></template>
        新建知识库
      </t-button>
    </div>

    <t-table row-key="id" :data="kbs" :loading="loading" :columns="columns">
      <template #operation="{ row }">
        <t-space>
          <t-link theme="primary" @click="openKb(row)">管理文档</t-link>
          <t-link theme="danger" @click="onDelete(row)">删除</t-link>
        </t-space>
      </template>
    </t-table>

    <t-dialog v-model:visible="dialogVisible" header="新建知识库" @confirm="onCreate">
      <t-form :data="createForm" label-width="100px">
        <t-form-item label="名称">
          <t-input v-model="createForm.name" placeholder="我的知识库" />
        </t-form-item>
        <t-form-item label="描述">
          <t-textarea v-model="createForm.description" :autosize="{ minRows: 3 }" />
        </t-form-item>
        <t-form-item label="Embedding">
          <t-input v-model="createForm.embedding_model" placeholder="text-embedding-3-small" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-drawer v-model:visible="docDrawerVisible" size="60%" :header="`文档管理 · ${currentKb?.name}`">
      <div class="drawer-toolbar">
        <t-upload
          :action="`/api/kb/${currentKb?.id}/documents`"
          :show-file-list="false"
          :max="10"
          @success="onUploadSuccess"
          @error="onUploadError"
        >
          <t-button theme="primary">
            <template #icon><t-icon name="upload" /></template>
            上传文档
          </t-button>
        </t-upload>
        <t-button variant="outline" @click="loadDocs">刷新</t-button>
      </div>
      <t-table row-key="id" :data="docs" :loading="docsLoading" :columns="docColumns">
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)">{{ statusLabel(row.status) }}</t-tag>
          <span v-if="row.status === 'failed'" class="err">{{ row.error_msg }}</span>
        </template>
        <template #operation="{ row }">
          <t-space>
            <t-link theme="primary" @click="onReparse(row)">重新解析</t-link>
            <t-link theme="danger" @click="onDeleteDoc(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { kbApi } from '@/api/kb'
import { documentApi } from '@/api/document'
import type { KnowledgeBase, Document } from '@/types'

const kbs = ref<KnowledgeBase[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const createForm = reactive({ name: '', description: '', embedding_model: 'text-embedding-3-small' })
const docDrawerVisible = ref(false)
const currentKb = ref<KnowledgeBase | null>(null)
const docs = ref<Document[]>([])
const docsLoading = ref(false)

const columns = [
  { colKey: 'name', title: '名称' },
  { colKey: 'description', title: '描述' },
  { colKey: 'created_at', title: '创建时间' },
  { colKey: 'operation', title: '操作', width: 200 },
]
const docColumns = [
  { colKey: 'title', title: '文件名' },
  { colKey: 'status', title: '状态', width: 180 },
  { colKey: 'created_at', title: '上传时间' },
  { colKey: 'operation', title: '操作', width: 180 },
]

import { reactive } from 'vue'

async function load() {
  loading.value = true
  try { kbs.value = await kbApi.list() }
  catch (e: any) { MessagePlugin.error(e.message) }
  finally { loading.value = false }
}
onMounted(load)

function openCreate() {
  createForm.name = ''
  createForm.description = ''
  createForm.embedding_model = 'text-embedding-3-small'
  dialogVisible.value = true
}
async function onCreate() {
  if (!createForm.name.trim()) { MessagePlugin.warning('名称必填'); return }
  try {
    await kbApi.create(createForm)
    dialogVisible.value = false
    MessagePlugin.success('已创建')
    load()
  } catch (e: any) { MessagePlugin.error(e.message) }
}

function openKb(row: KnowledgeBase) {
  currentKb.value = row
  docDrawerVisible.value = true
  loadDocs()
}
async function loadDocs() {
  if (!currentKb.value) return
  docsLoading.value = true
  try { docs.value = await documentApi.list(currentKb.value.id) }
  catch (e: any) { MessagePlugin.error(e.message) }
  finally { docsLoading.value = false }
}

function onUploadSuccess() {
  MessagePlugin.success('已上传，正在解析')
  loadDocs()
  // Auto-refresh after a few seconds for status update.
  setTimeout(loadDocs, 5000)
}
function onUploadError({ file }: any) {
  MessagePlugin.error(`上传失败：${file?.name || ''}`)
}

async function onReparse(row: Document) {
  try {
    await documentApi.reparse(row.id)
    MessagePlugin.success('已重新排队')
    setTimeout(loadDocs, 2000)
  } catch (e: any) { MessagePlugin.error(e.message) }
}
async function onDeleteDoc(row: Document) {
  const dlg = DialogPlugin.confirm({
    header: '删除文档',
    body: `确认删除 "${row.title}"？`,
    onConfirm: async () => {
      await documentApi.delete(row.id)
      dlg.destroy()
      MessagePlugin.success('已删除')
      loadDocs()
    },
  })
}
function onDelete(row: KnowledgeBase) {
  const dlg = DialogPlugin.confirm({
    header: '删除知识库',
    body: `确认删除 "${row.name}" 及其所有文档？`,
    onConfirm: async () => {
      await kbApi.delete(row.id)
      dlg.destroy()
      MessagePlugin.success('已删除')
      load()
    },
  })
}

function statusTheme(s: string) {
  return { pending: 'warning', parsing: 'processing', done: 'success', failed: 'danger' }[s] || 'default'
}
function statusLabel(s: string) {
  return { pending: '等待', parsing: '解析中', done: '完成', failed: '失败' }[s] || s
}
</script>

<style scoped lang="less">
.page { padding: 24px; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; h2 { margin: 0; } }
.drawer-toolbar { display: flex; gap: 12px; margin-bottom: 16px; }
.err { color: #e34d59; margin-left: 8px; font-size: 12px; }
</style>
```

- [ ] **Step 2: Verify in browser**

```bash
# backend already running
cd frontend && npm run dev
```
Visit `/kb`, create a KB, open it, upload a `.md` file, watch status transition. Expected: works end-to-end.

- [ ] **Step 3: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/src/views/KnowledgeBases.vue
git commit -m "feat(frontend): kb management page with upload and parse status"
```

---

### Task 6: Chat page

**Files:**
- Modify: `frontend/src/views/Chat.vue`
- Create: `frontend/src/components/MarkdownView.vue`

- [ ] **Step 1: Create MarkdownView component**

`frontend/src/components/MarkdownView.vue`:
```vue
<template>
  <div class="md" v-html="html"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import { markedKatex } from 'marked-katex-extension'
import hljs from 'highlight.js'
import DOMPurify from 'dompurify'

marked.use(markedKatex({ throwOnError: false }))

const props = defineProps<{ content: string }>()

const html = computed(() => {
  const raw = marked.parse(props.content || '', { async: false }) as string
  // Highlight code blocks.
  const wrapped = raw.replace(/<pre><code class="language-(\w+)">([\s\S]*?)<\/code><\/pre>/g,
    (_m, lang, code) => {
      try {
        const hl = hljs.highlight(code, { language: lang }).value
        return `<pre><code class="hljs language-${lang}">${hl}</code></pre>`
      } catch {
        return `<pre><code>${code}</code></pre>`
      }
    })
  return DOMPurify.sanitize(wrapped)
})
</script>

<style scoped lang="less">
.md {
  line-height: 1.7;
  :deep(h1) { font-size: 1.5em; margin: 0.8em 0 0.4em; }
  :deep(h2) { font-size: 1.3em; margin: 0.8em 0 0.4em; }
  :deep(h3) { font-size: 1.15em; margin: 0.6em 0 0.3em; }
  :deep(p) { margin: 0.5em 0; }
  :deep(pre) {
    background: #f6f8fa; padding: 12px; border-radius: 6px; overflow: auto;
    code { font-family: 'JetBrains Mono', Consolas, monospace; font-size: 13px; }
  }
  :deep(code) { background: #f0f2f5; padding: 2px 4px; border-radius: 3px; font-size: 0.9em; }
  :deep(pre code) { background: transparent; padding: 0; }
  :deep(table) { border-collapse: collapse; }
  :deep(th), :deep(td) { border: 1px solid #ddd; padding: 6px 12px; }
  :deep(blockquote) { border-left: 3px solid #ccc; padding-left: 12px; color: #666; }
}
</style>
```

- [ ] **Step 2: Implement Chat.vue**

```vue
<template>
  <div class="chat-page">
    <div class="chat-header">
      <t-select v-model="selectedKbId" placeholder="选择知识库" style="width: 280px;">
        <t-option v-for="kb in kbs" :key="kb.id" :value="kb.id" :label="kb.name" />
      </t-select>
      <t-button variant="text" @click="newConversation">新建对话</t-button>
    </div>

    <div class="messages" ref="messagesEl">
      <div v-for="msg in messages" :key="msg.id" :class="['msg', msg.role]">
        <div class="avatar">{{ msg.role === 'user' ? '我' : 'AI' }}</div>
        <div class="bubble">
          <MarkdownView v-if="msg.role === 'assistant'" :content="msg.content" />
          <template v-else>{{ msg.content }}</template>
          <div v-if="msg.citations?.length" class="citations">
            <t-collapse>
              <t-collapse-panel :header="`引用 ${msg.citations.length}`">
                <div v-for="(c, i) in msg.citations" :key="i" class="citation">
                  <span class="cit-title">{{ c.doc_title }}</span>
                  <span class="cit-snippet">{{ c.content_snippet }}</span>
                  <span class="cit-score">score: {{ c.score.toFixed(3) }}</span>
                </div>
              </t-collapse-panel>
            </t-collapse>
          </div>
        </div>
      </div>
    </div>

    <div class="input-bar">
      <t-textarea
        v-model="input"
        :autosize="{ minRows: 1, maxRows: 6 }"
        placeholder="输入问题，Enter 发送，Shift+Enter 换行"
        @keydown.enter.exact.prevent="onSend"
        class="input"
      />
      <t-button theme="primary" :loading="streaming" :disabled="!selectedKbId" @click="onSend">发送</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { kbApi } from '@/api/kb'
import { chatApi } from '@/api/chat'
import MarkdownView from '@/components/MarkdownView.vue'
import type { KnowledgeBase, Citation } from '@/types'

interface Msg {
  id: string
  role: 'user' | 'assistant'
  content: string
  citations?: Citation[]
}

const kbs = ref<KnowledgeBase[]>([])
const selectedKbId = ref('')
const input = ref('')
const messages = ref<Msg[]>([])
const streaming = ref(false)
const messagesEl = ref<HTMLElement | null>(null)

onMounted(async () => {
  try { kbs.value = await kbApi.list() } catch (e: any) { MessagePlugin.error(e.message) }
})

function newConversation() {
  messages.value = []
}

async function onSend() {
  const q = input.value.trim()
  if (!q || !selectedKbId.value || streaming.value) return
  const userMsg: Msg = { id: cryptoId(), role: 'user', content: q }
  const aiMsg: Msg = { id: cryptoId(), role: 'assistant', content: '' }
  messages.value.push(userMsg, aiMsg)
  input.value = ''
  streaming.value = true
  await scrollDown()
  try {
    await chatApi.stream(selectedKbId.value, q, null, {
      onDelta: (delta) => {
        aiMsg.content += delta
        scrollDown()
      },
      onCitations: (cits) => {
        aiMsg.citations = cits
      },
      onError: (err) => {
        MessagePlugin.error(err)
      },
      onDone: () => { /* stream end */ },
    })
  } catch (e: any) {
    MessagePlugin.error(e.message)
  } finally {
    streaming.value = false
  }
}

function cryptoId() {
  return Math.random().toString(36).slice(2)
}
async function scrollDown() {
  await nextTick()
  if (messagesEl.value) messagesEl.value.scrollTop = messagesEl.value.scrollHeight
}
</script>

<style scoped lang="less">
.chat-page {
  display: flex; flex-direction: column; height: 100vh;
  background: #f5f7fa;
}
.chat-header {
  padding: 12px 24px; background: #fff; border-bottom: 1px solid #eee;
  display: flex; align-items: center; gap: 12px;
}
.messages { flex: 1; overflow-y: auto; padding: 24px; }
.msg {
  display: flex; gap: 12px; margin-bottom: 24px; max-width: 920px; margin-left: auto; margin-right: auto;
  &.user { .bubble { background: #e6f0ff; } .avatar { background: #2e6cc4; } }
  &.assistant { .bubble { background: #fff; border: 1px solid #eee; } .avatar { background: #1f8a5b; } }
}
.avatar {
  width: 36px; height: 36px; border-radius: 6px; color: #fff;
  display: flex; align-items: center; justify-content: center; font-size: 14px; flex-shrink: 0;
}
.bubble {
  flex: 1; padding: 12px 16px; border-radius: 8px; line-height: 1.6;
}
.citations { margin-top: 12px; border-top: 1px dashed #ddd; padding-top: 8px; }
.citation {
  padding: 8px 0; border-bottom: 1px dashed #eee; font-size: 13px;
  .cit-title { font-weight: 600; margin-right: 8px; }
  .cit-snippet { color: #555; }
  .cit-score { color: #999; margin-left: 8px; }
}
.input-bar {
  padding: 12px 24px; background: #fff; border-top: 1px solid #eee;
  display: flex; gap: 12px; align-items: flex-end;
  .input { flex: 1; }
}
</style>
```

- [ ] **Step 3: Verify in browser**

```bash
# backend running
cd frontend && npm run dev
```
Visit `/chat`, select KB, ask a question. Expected: streamed answer + citations drawer.

- [ ] **Step 4: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/src/views/Chat.vue frontend/src/components/
git commit -m "feat(frontend): chat page with SSE stream + markdown + citations"
```

---

### Task 7: Type-check and production build

- [ ] **Step 1: Type-check**

```bash
cd frontend
npm run type-check
```
Expected: no errors.

- [ ] **Step 2: Build**

```bash
npm run build
```
Expected: `frontend/dist/` produced with index.html + assets.

- [ ] **Step 3: Commit**

```bash
cd D:/Project/TaiSang-KB
git add frontend/
git commit -m "chore(frontend): type-check and build pass" --allow-empty
```

---

### Task 8: Self-review and checkpoint

- [ ] **Step 1: End-to-end smoke**

```bash
# Backend
docker compose up -d postgres
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
# Frontend (separate terminal)
cd frontend && npm run dev
```
- Open `/settings`, fill OpenAI-compatible API key + models, save.
- Open `/kb`, create KB, upload a markdown file.
- Wait for status → done (refresh if needed).
- Open `/chat`, select KB, ask "你的文档里讲了什么？" — expect streamed answer with citations.

- [ ] **Step 2: Commit checkpoint**

```bash
git commit --allow-empty -m "checkpoint: Part 5 frontend complete"
```

---

## End of Part 5

**What's done:**
- Vite + Vue 3 + TDesign + Pinia + Vue Router scaffold
- App shell with sidebar nav (WeKnora-style dark sidebar)
- API client + types
- Settings page (API key/model config, masked display)
- Knowledge Bases page (CRUD + upload + parse status + reparse + delete)
- Chat page (SSE stream + Markdown render with code highlight + KaTeX + citations drawer)

**Next:** Part 6 — Dockerfile + full-stack docker compose + README.