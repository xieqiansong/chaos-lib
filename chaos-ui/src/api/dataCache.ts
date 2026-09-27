// 数据缓存（dataCache）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
// Value 为二进制存储（BLOB/BYTEA），JSON 传输为 base64 字符串，故接口类型仍为 string。
// 压缩算法（Compression）当前为预留字典，暂无后端压缩逻辑，仅做字段透传。
import {useRestApi} from '@/composables/useRestApi'

export interface DataCache {
  ID: number
  Category: string
  Key: string
  Value: string
  ExpireAt: string | null
  Compression: string
  DataType: string
  ValueLen: number
  ValueMd5: string
  CreatedAt: string
  UpdatedAt: string
  IsDeleted: boolean
}

/** 压缩算法预留字典（与后端注释一致）：none / gzip / zstd */
export const COMPRESSION_OPTIONS = [
  {label: '无', value: 'none'},
  {label: 'gzip', value: 'gzip'},
  {label: 'zstd', value: 'zstd'},
] as const

/** 元素数据类型字典（供统计用，手动选择） */
export const DATATYPE_OPTIONS = [
  {label: 'text', value: 'text'},
  {label: 'json', value: 'json'},
  {label: 'xml', value: 'xml'},
  {label: 'yaml', value: 'yaml'},
  {label: 'png', value: 'png'},
  {label: 'jpg', value: 'jpg'},
  {label: 'gif', value: 'gif'},
  {label: 'mp4', value: 'mp4'},
  {label: 'mp3', value: 'mp3'},
  {label: '其他', value: 'other'},
] as const

const rest = useRestApi<DataCache>('dataCache')

export const dataCacheApi = {
  ...rest,
}