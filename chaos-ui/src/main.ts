import {createApp} from 'vue';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import './style.css';
import './plugins/echarts';
import App from './App.vue';
import VChart from 'vue-echarts';
import router from './router';
import * as ElementPlusIconsVue from '@element-plus/icons-vue';
import {initTheme} from './theme';
import {getHostname} from './utils/api';

initTheme();

const app = createApp(App);

// 全局注册 Element Plus 图标，供路由 meta.icon 按名称动态渲染
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.use(ElementPlus);
app.use(router);
app.component('VChart', VChart);
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
