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