import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {crx} from '@crxjs/vite-plugin'
import manifest from './manifest.json'

export default defineConfig({
    plugins: [
        vue(),
        crx({manifest})
    ],
    build: {
        rollupOptions: {
            // crx 插件会处理 manifest 指定的 entry 文件
        }
    }
})
