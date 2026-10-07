import {API_BASE} from "@/utils/api";

/** 后端统一响应（code === 0 成功）。 */
export interface ApiResponse<T = unknown> {
    code: number;
    message: string;
    data: T;
    requestId?: string; // 链路追踪 / 幂等
    timestamp?: number;
}

/** 统一请求信封（POST + Action 模式）。 */
export interface ActionEnvelope {
    requestId: string;
    action: string; // 形如 proxy.browserHistories/save
    data: unknown;
    meta?: unknown;
    timestamp: number;
}

/** 生成请求唯一 ID，优先 crypto.randomUUID，降级随机数。 */
function genRequestId(): string {
    try {
        if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
            return crypto.randomUUID();
        }
    } catch { /* 降级 */ }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = (Math.random() * 16) | 0;
        const v = c === 'x' ? r : (r & 0x3) | 0x8;
        return v.toString(16);
    });
}

/** 构造请求信封，供特殊场景复用。 */
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

/** 调用后端 action：POST {API_BASE}/{module}/{act}，code !== 0 抛 message。 */
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
