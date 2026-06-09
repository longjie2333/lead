import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'
import { WeappTailwindcss } from 'weapp-tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    uni(),
    WeappTailwindcss({
      rem2rpx: true,
    }),
  ],
})
