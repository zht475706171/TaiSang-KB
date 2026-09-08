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
import { onMounted, reactive, ref } from 'vue'
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
  return ({ pending: 'warning', parsing: 'processing', done: 'success', failed: 'danger' } as const)[s as 'pending'] || 'default'
}
function statusLabel(s: string) {
  return ({ pending: '等待', parsing: '解析中', done: '完成', failed: '失败' } as const)[s as 'pending'] || s
}
</script>

<style scoped lang="less">
.page { padding: 24px; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; h2 { margin: 0; } }
.drawer-toolbar { display: flex; gap: 12px; margin-bottom: 16px; }
.err { color: #e34d59; margin-left: 8px; font-size: 12px; }
</style>