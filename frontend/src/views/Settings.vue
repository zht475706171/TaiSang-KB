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