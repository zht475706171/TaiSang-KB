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