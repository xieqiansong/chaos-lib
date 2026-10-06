// 笔记模块接口门户：视图只从这里导入类型与 api 对象。
// 标准 CRUD 走 useRestApi<Note>('notes')；树 / 内容 / 扫描为笔记自定义接口（v1 动作）。
import {action} from '@/utils/request'
import {useRestApi} from '@/composables/useRestApi'

export interface Note {
  ID: number
  VaultID: number
  RelPath: string
  ParentRel: string
  Name: string
  Title: string
  Summary: string
  Format: string
  SizeBytes: number
  DiskMTime: string
  DiskMissing: boolean
  Starred: boolean
  WordCount: number
  TagNames: string
  IndexedAt: string
  CreatedAt: string
  UpdatedAt: string
}

export interface TreeNode {
  name: string
  relPath: string
  isLeaf: boolean
  children?: TreeNode[]
}

export interface ScanResult {
  scanned: number
  added: number
  updated: number
  missing: number
  skipped: number
  elapsedMs: number
}

export interface ContentResponse {
  relPath: string
  name: string
  format: string
  content: string
  contentHash: string
  sizeBytes: number
}

// 保存成功回执
export interface SaveResult {
  contentHash: string
  updatedAt: string
}

// 乐观锁冲突详情（随 409 返回），前端据以「覆盖 / 重载」
export interface ConflictInfo {
  baseHash: string
  currentHash: string
  currentSize: number
  diskUpdatedAt: string
}

const rest = useRestApi<Note>('notes')

export const noteApi = {
    ...rest,
    // 目录树（前端 el-tree 使用，节点为 {name, relPath, isLeaf, children}）
    tree: () => action<TreeNode[]>('notes', 'tree'),
    // 按 id 读取磁盘原文（只读）
    content: (id: number) => action<ContentResponse>('notes', 'getContent', {id}),
    // 触发一次 Vault 扫描（full=true 强制全量重算）
    scan: (full = false) =>
        action<ScanResult>('notes', 'scan', {}, full ? {full: true} : undefined),
    // 保存正文（body: content + baseHash 乐观锁）；冲突时抛 409，调用方读 err.response.data.conflict
    saveContent: (id: number, content: string, baseHash: string) =>
        action<SaveResult>('notes', 'saveContent', {id, content, baseHash}),
    // 新建空笔记（文件优先，走 v1 create 动作）
    createNote: (parentRel: string, name: string) =>
        action<Note>('notes', 'create', {parentRel, name}),
    // 重命名（仅改文件名）
    renameNote: (id: number, name: string) =>
        action<Note>('notes', 'rename', {id, name}),
    // 移动到目标目录（根级传空串）
    moveNote: (id: number, targetDir: string) =>
        action<Note>('notes', 'move', {id, targetDir}),
    // 移入回收站
    deleteNote: (id: number) =>
        action<{ ok: boolean }>('notes', 'trash', {id}),
}
