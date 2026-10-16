import {createApp} from 'vue';
import 'element-plus/theme-chalk/dark/css-vars.css';
// ElMessage / ElMessageBox 为 JS 函数式调用且各处均为显式 import，
// unplugin 按需解析器不会为它们注入样式，必须在入口手动引入
import 'element-plus/es/components/message/style/css';
import 'element-plus/es/components/message-box/style/css';
import './tokens.css';
import './style.css';
import App from './App.vue';
import router from './router';
import {
    AlarmClock,
    Coin,
    Collection,
    Connection,
    Delete,
    EditPen,
    Folder,
    Grid,
    HomeFilled,
    Notebook,
    Key,
    Link,
    MagicStick,
    Monitor,
    Moon,
    Odometer,
    Plus,
    Setting,
    Sort,
    Star,
    Sunny
} from '@element-plus/icons-vue';
import * as ElementPlusIcons from '@element-plus/icons-vue';
import {collectIcons, installPlugins} from '@/plugins/registry';
import {initTheme} from './theme';
import {getHostname} from './utils/api';
import {Buffer} from 'buffer';

// polyform-tools 的 CSV 解析器依赖 Buffer（浏览器无内置），在此注入浏览器实现
;(window as any).Buffer = Buffer;

initTheme();

const app = createApp(App);

// 全局注册用到的 Element Plus 图标（白名单），供路由 meta.icon 按名称动态渲染
const usedIcons = {
    AlarmClock,
    Coin,
    Collection,
    Connection,
    Delete,
    EditPen,
    Folder,
    HomeFilled,
    Notebook,
    Link,
    MagicStick,
    Monitor,
    Moon,
    Odometer,
    Plus,
    Setting,
    Sort,
    Star,
    Sunny,
    Grid,
    Key
}
for (const [key, component] of Object.entries(usedIcons)) {
    app.component(key, component);
}

// 插件声明的图标自动注册：新增插件只要在路由 meta.icon 写 Element Plus 图标名，
// 即自动可用，无需再维护上方白名单（后续存量路由迁入插件后，上方白名单可逐步清空）。
for (const name of collectIcons()) {
    const comp = (ElementPlusIcons as Record<string, unknown>)[name]
    if (comp && !app.component(name)) {
        app.component(name, comp as any)
    }
}

// 触发各插件的 install 钩子（如 echarts 按需注册组件），须在挂载前完成。
installPlugins(app);

app.use(router);
app.mount('#app');

// 用后端返回的主机名覆盖浏览器标签标题（默认占位为 index.html 中的 "Chaos"）
getHostname()
    .then(info => {
        if (info && info.hostname) {
            let title = 'chaos(' + info.username + '@' + info.hostname + ")";
            document.title = title.trim();
        }
    })
    .catch(() => {
        // 获取失败时保留默认标题
    });
