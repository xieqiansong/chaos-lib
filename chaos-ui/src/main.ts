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
