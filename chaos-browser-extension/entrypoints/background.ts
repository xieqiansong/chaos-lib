// entrypoints/background.ts
// @ts-ignore
import browser from "webextension-polyfill";
import {saveBrowserHistory} from "./background/browser-history-backup";
import {saveBookmarks} from "./background/bookmark-backup";
import {API_BASE} from "@/utils/api";

export default defineBackground(() => {
    browser.runtime.onMessage.addListener(async (msg: any) => {
        let url = API_BASE + "/" + msg.path;
        const noBodyMethods = ['GET', 'HEAD', 'OPTIONS', 'TRACE'];

        const fetchOptions: RequestInit = {
            method: msg.method,
            headers: {"Content-Type": "application/json"},
        };

        if (!noBodyMethods.includes(msg.method)) {
            fetchOptions.body = JSON.stringify(msg.payload);
        } else if (msg.payload && Object.keys(msg.payload).length > 0) {
            const params = new URLSearchParams(msg.payload);
            url += '?' + params.toString();
        }

        const res = await fetch(url, fetchOptions);
        return await res.json();
    });

    // 网页（chaos-ui）经 externally_connectable 发来的即时指令：直接执行并回包。
    // 该消息会自动唤醒 MV3 service worker，无需心跳/SSE 保活。
    browser.runtime.onMessageExternal.addListener(
        (msg: any, _sender: any, sendResponse: (r: any) => void) => {
            handleCommand(msg)
                .then(sendResponse)
                .catch((err) =>
                    sendResponse({id: msg?.id, type: msg?.type, ok: false, error: String(err?.message || err)}),
                );
            return true; // 保持消息通道开放，等待异步 sendResponse
        },
    );

    browser.alarms.onAlarm.addListener((alarm: { name: string; }) => {
        if (alarm.name === 'saveBrowserHistory') {
            // 返回 Promise，告知 MV3 在异步上传完成前保持 service worker 存活。
            return saveBrowserHistory().catch((err) => {
                console.error("saveBrowserHistory (alarm) failed:", err);
            });
        }
        if (alarm.name === 'saveBookmarks') {
            return saveBookmarks().catch((err) => {
                console.error("saveBookmarks (alarm) failed:", err);
            });
        }
    });

    // 启动即触发一次（兜底，真正的周期任务由下方 alarm 保证）。
    saveBrowserHistory().catch((err) => {
        console.error("saveBrowserHistory (startup) failed:", err);
    });
    // 使用 chrome.alarms（✅ 推荐）
    browser.alarms.create('saveBrowserHistory', {
        delayInMinutes: 0,
        periodInMinutes: 1,
    });

    // 书签变更频率远低于历史，5 分钟备份一次即可。
    saveBookmarks().catch((err) => {
        console.error("saveBookmarks (startup) failed:", err);
    });
    browser.alarms.create('saveBookmarks', {
        delayInMinutes: 0,
        periodInMinutes: 5,
    });
});

// 执行指令并返回结果，由调用方（网页经 externally_connectable 直连）负责 sendResponse。
async function handleCommand(cmd: any): Promise<any> {
    console.log("[reverse] 收到指令", cmd?.type, "nodeId=", cmd?.nodeId, "id=", cmd?.id)
    let result: any = {id: cmd.id, type: cmd.type, ok: true};
    try {
        switch (cmd.type) {
            case "hello":
                // 通道就绪心跳，无需回传
                return;
            case "openTab":
                if (cmd.url) {
                    await browser.tabs.create({url: cmd.url});
                }
                break;
            case "ping":
                result = {...result, type: "pong", echo: cmd};
                break;

            // ── 书签操作（仅操作浏览器书签，不落库）─────────────────────
            case "bookmarks:getTree": {
                const tree = await browser.bookmarks.getTree();
                result = {...result, type: "bookmarks:getTree", echo: tree};
                break;
            }
            case "bookmarks:search": {
                const nodes = await browser.bookmarks.search(cmd.query || "");
                result = {...result, type: "bookmarks:search", echo: {query: cmd.query, nodes}};
                break;
            }
            case "bookmarks:create": {
                // url 缺省则创建文件夹
                const node = await browser.bookmarks.create({
                    parentId: cmd.parentId,
                    title: cmd.title,
                    url: cmd.url || undefined,
                });
                result = {...result, type: "bookmarks:created", echo: node};
                break;
            }
            case "bookmarks:update": {
                const changes: any = {};
                if (cmd.title !== undefined) changes.title = cmd.title;
                if (cmd.url !== undefined) changes.url = cmd.url;
                const node = await browser.bookmarks.update(cmd.nodeId, changes);
                result = {...result, type: "bookmarks:updated", echo: node};
                break;
            }
            case "bookmarks:remove": {
                // nodeId 必须是字符串；undefined / 数字都直接报错，避免浏览器底层签名错误掩盖真相。
                const targetId = cmd.nodeId
                if (targetId === undefined || targetId === null || targetId === "") {
                    throw new Error("缺少 nodeId，无法删除（前端未传入书签节点 id）")
                }
                const idStr = String(targetId)
                if (cmd.isFolder) {
                    await browser.bookmarks.removeTree(idStr)
                } else {
                    // 先用 remove（普通书签）；若误判为书签而实际是文件夹，兜底 removeTree。
                    try {
                        await browser.bookmarks.remove(idStr)
                    } catch (e) {
                        await browser.bookmarks.removeTree(idStr)
                    }
                }
                result = {...result, type: "bookmarks:removed", echo: {id: idStr, removed: true}};
                break;
            }
            case "bookmarks:move": {
                // 拖拽移动：修改节点的父级（parentId），可选 index 指定同级顺序
                const targetId = cmd.nodeId
                if (targetId === undefined || targetId === null || targetId === "") {
                    throw new Error("缺少 nodeId，无法移动")
                }
                const destination: any = {}
                if (cmd.parentId !== undefined && cmd.parentId !== null && cmd.parentId !== "") {
                    destination.parentId = String(cmd.parentId)
                }
                if (typeof cmd.index === "number" && cmd.index >= 0) {
                    destination.index = cmd.index
                }
                const node = await browser.bookmarks.move(String(targetId), destination)
                result = {...result, type: "bookmarks:moved", echo: node};
                break;
            }
            default:
                result = {id: cmd.id, type: cmd.type, ok: false, error: "未知指令: " + cmd.type};
        }
    } catch (err: any) {
        // 任何执行异常都回传，避免后端傻等超时；同时把真实错误带回 UI
        result = {id: cmd.id, type: cmd.type, ok: false, error: String(err?.message || err)};
    }
    return result;
}
