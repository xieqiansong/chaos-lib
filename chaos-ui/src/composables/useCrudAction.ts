// 写操作动作控制器：把「二次确认 → 执行请求 → 成功/失败提示 → 关弹窗/刷新」收敛成一次调用。
//
// 存量页面里这套骨架被逐字复制（TaskPlan 的 6 个 confirm 块、ProjectManage 的 5 个 action），
// 每处都要手动写 try/catch、判断 `e === 'cancel'`、console.error、ElMessage.success/error。
// 使用方只需提供真正有差异的部分：要执行的请求、确认文案、提示文案、成功后的回调。
import {ref} from 'vue'
import {confirm as confirmBox, showError, showSuccess} from '@/utils/message'

export interface CrudActionOptions {
  /** 二次确认文案；不传则直接执行 */
  confirm?: string
  /** 确认框标题，默认「提示」 */
  confirmTitle?: string
  /** 确认按钮文案，默认「确定」 */
  confirmButtonText?: string
  /** 确认框图标类型，默认 warning */
  type?: 'success' | 'warning' | 'info' | 'error'
  /** 确认按钮追加 class，危险操作传 'el-button--danger' */
  confirmButtonClass?: string
  /** 成功提示文案；不传则不提示 */
  success?: string
  /**
   * 失败提示：传字符串时为「兜底文案」（服务端有 message 时优先用服务端消息）；
   * 传函数则完全自定义（返回空串表示不提示），用于「失败原因分支提示」等场景。
   */
  error?: string | ((err: any) => string)
  /** 成功后回调：关弹窗 / 刷新列表，支持异步 */
  onDone?: () => void | Promise<void>
  /** 失败时是否打印到控制台，默认 true */
  log?: boolean
}

/**
 * 创建一个动作控制器。
 * - running：执行中标志，可绑到弹窗的 :loading 或按钮的 :disabled
 * - run(task, opts)：执行一个写操作；失败只提示不抛出，调用处无需 try/catch
 */
export function useCrudAction() {
  const running = ref(false)

  async function run(task: () => Promise<unknown>, opts: CrudActionOptions = {}): Promise<void> {
    if (opts.confirm) {
      try {
        await confirmBox(opts.confirm, {
          title: opts.confirmTitle,
          type: opts.type,
          confirmButtonText: opts.confirmButtonText,
          confirmButtonClass: opts.confirmButtonClass,
        })
      } catch {
        return // 用户取消 / 关闭确认框
      }
    }

    running.value = true
    try {
      await task()
      if (opts.success) showSuccess(opts.success)
      await opts.onDone?.()
    } catch (e: any) {
      if (opts.log !== false) console.error(e)
      if (opts.error) {
        const msg = typeof opts.error === 'function' ? opts.error(e) : (e?.message || opts.error)
        if (msg) showError(msg)
      }
    } finally {
      running.value = false
    }
  }

  return {running, run}
}
