import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { initializeUiTheme } from './ui/base'
import { i18n, initializeUiLocale } from './i18n'
import { startAuthorizationSynchronization } from './services/runtime/authorization'
import './styles/shadcn.css'
import './styles/tokens.css'
import './styles/base.css'
initializeUiTheme()
initializeUiLocale()
startAuthorizationSynchronization()
createApp(App).use(createPinia()).use(router).use(i18n).mount('#app')
