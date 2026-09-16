import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {resolve} from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import {ElementPlusResolver} from 'unplugin-vue-components/resolvers'

export default defineConfig({
    resolve: {
        alias: {
            '@': resolve(import.meta.dirname, 'src')
        }
    },
    server: {
        proxy: {
            '/api': {
                target: 'http://localhost:8080',
                changeOrigin: true
            }
        }
    },
    plugins: [
        vue(),
        AutoImport({
            resolvers: [ElementPlusResolver()]
        }),
        Components({
            resolvers: [ElementPlusResolver()]
        })
    ],
    build: {
        rolldownOptions: {
            external: ['monaco-editor'],
            output: {
                // Rolldown 的 manualChunks 仅支持函数形式（对象形式是 Rollup 语法）
                manualChunks(id) {
                    if (id.includes('node_modules/echarts')) return 'echarts'
                }
            }
        }
    }
})