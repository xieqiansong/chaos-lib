// @ts-ignore
import browser from "webextension-polyfill";
import {saveBrowserHistory} from "./background/browser-history-backup";
import {saveBookmarks} from "./background/bookmark-backup";
import {action} from "@/utils/request";

export default defineBackground(() => {
    // runtime.sendMessage 收敛为 action 模式：新契约 {module, action, data?, meta?}，旧契约 {path, payload?} 兼容。
    browser.runtime.onMessage.addListener(async (msg: any) => {
        if (!msg) return;
        if (msg.module && msg.action) {
            return await action(msg.module, msg.action, msg.data, msg.meta);
        }
        if (msg.path) {
            const cleaned = String(msg.path).replace(/^\/+|\/+$/g, "");
            const slash = cleaned.indexOf("/");
            const module = slash < 0 ? cleaned : cleaned.slice(0, slash);
            const act = slash < 0 ? "" : cleaned.slice(slash + 1);
            return await action(module, act, msg.payload, msg.meta);
        }
        throw new Error("未知消息格式：缺少 module/action 或 path");
    });

    // 网页经 externally_connectable 发来的即时指令；自动唤醒 service worker，无需保活。
    browser.runtime.onMessageExternal.addListener(
        (msg: any, _sender: any, sendResponse: (r: any) => void) => {
            handleCommand(msg)
                .then(sendResponse)
                .catch((err) =>
                    sendResponse({id: msg?.id, type: msg?.type, ok: false, error: String(err?.message || err)}),
                );
            return true; // 保持通道开放，等待异步 sendResponse
        },
    );

    browser.alarms.onAlarm.addListener((alarm: { name: string; }) => {
        if (alarm.name === 'saveBrowserHistory') {
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

    // 启动即触发一次（兜底，周期由下方 alarm 保证）
    saveBrowserHistory().catch((err) => {
        console.error("saveBrowserHistory (startup) failed:", err);
    });
    browser.alarms.create('saveBrowserHistory', {
        delayInMinutes: 0,
        periodInMinutes: 1,
    });

    // 书签变更频率低，5 分钟一次
    saveBookmarks().catch((err) => {
        console.error("saveBookmarks (startup) failed:", err);
    });
    browser.alarms.create('saveBookmarks', {
        delayInMinutes: 0,
        periodInMinutes: 5,
    });
});

// 执行指令并返回结果；调用方（网页经 externally_connectable）负责 sendResponse。
async function handleCommand(cmd: any): Promise<any> {
    console.log("[reverse] 收到指令", cmd?.type, "nodeId=", cmd?.nodeId, "id=", cmd?.id)
    let result: any = {id: cmd.id, type: cmd.type, ok: true};
    try {
        switch (cmd.type) {
            case "hello":
                return;
            case "openTab":
                if (cmd.url) {
                    await browser.tabs.create({url: cmd.url});
                }
                break;
            case "ping":
                result = {...result, type: "pong", echo: cmd};
                break;

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
                const targetId = cmd.nodeId
                if (targetId === undefined || targetId === null || targetId === "") {
                    throw new Error("缺少 nodeId，无法删除（前端未传入书签节点 id）")
                }
                const idStr = String(targetId)
                if (cmd.isFolder) {
                    await browser.bookmarks.removeTree(idStr)
                } else {
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
        result = {id: cmd.id, type: cmd.type, ok: false, error: String(err?.message || err)};
    }
    return result;
}
