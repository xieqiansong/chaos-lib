import {API_BASE} from "@/utils/api";

/** 后端接口返回的统一结构（按仓库《接口规范.md》：code === 0 成功）。 */
export interface ApiResponse<T = unknown> {
    code: number;
    message: string;
    data: T;
    /** 本次请求的唯一 ID，链路追踪与幂等用。 */
    requestId?: string;
    /** 响应毫秒时间戳。 */
    timestamp?: number;
}

/** 统一请求信封（POST + Action 模式），详见仓库《接口规范.md》。 */
export interface ActionEnvelope {
    requestId: string;
    /** 动作名，形如 `proxy.browserHistories/save`。 */
    action: string;
    /** 业务数据。 */
    data: unknown;
    /** 分页/排序/来源等附加信息。 */
    meta?: unknown;
    /** 请求毫秒时间戳。 */
    timestamp: number;
}

/** 生成请求唯一 ID（用于幂等、链路追踪）：优先原生 crypto.randomUUID，降级到随机数。 */
function genRequestId(): string {
    try {
        if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
            return crypto.randomUUID();
        }
    } catch {
        // 忽略，走降级
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = (Math.random() * 16) | 0;
        const v = c === 'x' ? r : (r & 0x3) | 0x8;
        return v.toString(16);
    });
}

/** 构造统一请求信封，供需要自定义 fetch 配置的特殊场景复用。 */
export function buildEnvelope(
    module: string,
    act: string,
    data?: unknown,
    meta?: unknown,
): ActionEnvelope {
    return {
        requestId: genRequestId(),
        action: `${module}.${act}`,
        data: data ?? {},
        meta,
        timestamp: Date.now(),
    };
}

/**
 * action 调用：与后端「POST + Action」统一契约对应（详见《接口规范.md》）。
 * 向 POST {API_BASE}/{module}/{act} 发送带信封的请求体，自动注入 requestId 与 timestamp。
 * 解析响应信封：code !== 0 时抛出 message（便于 service worker 侧捕获并上报告警），
 * 否则只返回 data 业务字段。
 *
 * 示例：await action('proxy', 'browserHistories/save', chunk)
 * 对应后端：POST http://localhost:30030/api/v1/proxy/browserHistories/save，
 *          body 为 { requestId, action: 'proxy.browserHistories/save', data: chunk, timestamp }
 */
export async function action<T = any>(
    module: string,
    act: string,
    data?: unknown,
    meta?: unknown,
): Promise<T> {
    const url = `${API_BASE}/${module}/${act}`;
    const envelope = buildEnvelope(module, act, data, meta);

    let res: Response;
    try {
        res = await fetch(url, {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify(envelope),
        });
    } catch (err) {
        throw new Error(`网络请求失败: ${String((err as Error)?.message || err)}`);
    }

    let json: ApiResponse<T>;
    try {
        json = (await res.json()) as ApiResponse<T>;
    } catch {
        throw new Error(`响应解析失败 (HTTP ${res.status})`);
    }

    if (json && typeof json.code === 'number' && json.code !== 0) {
        throw new Error(json.message || `请求失败 (code=${json.code})`);
    }
    return json.data;
}

export default action;
