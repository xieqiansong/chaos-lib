import {defineConfig} from 'wxt';

// See https://wxt.dev/api/config.html
export default defineConfig({
    modules: ['@wxt-dev/module-vue'],
    manifest: {
        permissions: ['history', 'bookmarks', 'alarms', 'tabs', 'storage'],
        host_permissions: [
            "http://localhost:8080/*",
            "http://localhost:1234/*",
            "http://localhost:30030/*",
        ],
        // 允许网页（chaos-ui）经 chrome.runtime 直接向本扩展发消息，实现「即时命令」直连通道。
        // 开发期用 http://localhost/* 覆盖任意 localhost 端口；生产请改为你的实际域名。
        externally_connectable: {
            matches: ["http://localhost/*"],
        },
    },
});