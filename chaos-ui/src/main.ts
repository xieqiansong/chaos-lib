import {createApp} from 'vue';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import './style.css';
import './plugins/echarts';
import App from './App.vue';
import VChart from 'vue-echarts';

const app = createApp(App);
app.use(ElementPlus);
app.component('VChart', VChart);
app.mount('#app');