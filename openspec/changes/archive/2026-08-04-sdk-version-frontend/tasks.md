# Tasks

## 1. 数据层：封装 defs 接口
- [x] 在 `chaos-ui/src/utils/api.ts` 封装 `getSdkVersions / getSdkDefs / createSdkDef / updateSdkDef / deleteSdkDef`

## 2. Sdk.vue 来源可视化
- [x] 挂载时并行拉 `GET /sdks` 与 `GET /sdks/defs`
- [x] 每个 SDK 类型行展示 `sources` 数组（repo/single 徽标 + root 路径）

## 3. 切换语义修正
- [x] 仅含 single 来源的类型禁用切换点击并提示；含 repo 的类型允许切换
- [x] switch 返回 400 时 ElMessage 提示「单版本来源不可切换」

## 4. 管理界面
- [x] 新增「管理来源」对话框：列出 defs、新增/编辑（动态 sources 表单 + Enabled + Note）、删除（软删除确认）
- [x] 操作后刷新 defs 与 sdks

## 5. 校验
- [x] `npm run build` 通过，无类型错误
- [x] `openspec validate --changes` 通过
