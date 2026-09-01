import { VueQueryPlugin } from '@tanstack/vue-query'
import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'
import 'element-plus/theme-chalk/dark/css-vars.css'
import './design-tokens.css'
import './global.css'

createApp(App).use(i18n).use(router).use(VueQueryPlugin).mount('#app')
