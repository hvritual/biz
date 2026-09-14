import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { initializeUiTheme } from './ui/theme'
import './styles/shadcn.css'
import './styles/tokens.css'
import './styles/base.css'
initializeUiTheme()
createApp(App).use(createPinia()).use(router).mount('#app')
