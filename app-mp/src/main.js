import {
  createSSRApp
} from 'vue'
import App from './App.vue'
import pinia from './stores'
import { installAuthGuard } from './utils/auth'

export function createApp() {
  const app = createSSRApp(App)
  app.use(pinia)
  installAuthGuard(app)
  return {
    app,
  }
}
