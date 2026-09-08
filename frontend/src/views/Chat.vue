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