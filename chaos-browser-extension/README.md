# chaos-browser-extension

基于 WXT + Vue 3 的浏览器扩展，作为客户端直连本机 `chaos-go` 后端：管理浏览器书签树、备份浏览器历史，并提供新标签页搜索。

与 `chaos-ui` / `chaos-go` 一样是**独立工程**（自带依赖锁与构建配置），由根仓库 `scripts/chaos.deploy.ps1` 构建，产物 `.output/chrome-mv3/` 拷到部署目录的 `chrome-mv3/`。

## 目录

| 路径 | 作用 |
|------|------|
| `entrypoints/background.ts` | 后台入口：书签 / 历史备份与消息处理 |
| `entrypoints/background/` | 书签备份、浏览器历史备份逻辑 |
| `entrypoints/content.ts` | 内容脚本 |
| `entrypoints/newtab/` | 新标签页：`index.html`、`main.ts`、`Search.vue`、`search-engine-dropdown.ts`、`redirect.ts` |
| `entrypoints/popup/` | 弹窗页面（`App.vue` 等） |
| `utils/` | `request.ts` / `api.ts`：与后端 `/api/v1` 通信 |
| `public/` | 图标等静态资源 |
| `wxt.config.ts` | WXT 配置 |

> `components/HelloWorld.vue` 与 `assets/vue.svg` 是 WXT 模板残留，未参与业务；`entrypoints/newtab/Search.vue` 与 `chaos-ui/src/views/Search.vue` 目前同源重复，见 `docs/MODULE_INDEX.md` §5。

## 开发 / 构建

```bash
pnpm install
pnpm dev       # 开发模式（默认 Chrome，Firefox 用 dev:firefox）
pnpm build    # 输出到 .output/chrome-mv3/
pnpm compile  # vue-tsc 类型检查
```

> 扩展直连本机后端需在 `chaos-ui` 书签管理页填写直连 ID；本机地址与凭据只放本地配置，禁止写入仓库。
