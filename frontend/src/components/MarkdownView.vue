<template>
  <div class="md" v-html="html"></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import markedKatex from 'marked-katex-extension'
import hljs from 'highlight.js'
import DOMPurify from 'dompurify'

marked.use(markedKatex({ throwOnError: false }))

const props = defineProps<{ content: string }>()

const html = computed(() => {
  const raw = marked.parse(props.content || '', { async: false }) as string
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