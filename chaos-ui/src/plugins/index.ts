// 插件聚合入口：以副作用导入的方式触发各插件的 registerPlugin（插件的自注册在各自
// index.ts 中完成，与后端各包 init() 调 plugin.Register 对齐）。
// router/index.ts 会在使用 collectRoutes 之前导入本文件，确保登记先于路由汇总完成。
import './taskplan'
import './echarts'
import './project'
import './note'
import './portfwd'
import './envvar'
import './filelink'
import './sdk'
import './standarddata'
import './quickedit'
import './cronjob'
import './mqttsync'
import './proxy'
