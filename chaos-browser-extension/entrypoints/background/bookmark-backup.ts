// 浏览器书签树扁平化备份：把树形结构展平为节点列表上报后端 /bookmarks，
// 供「常用书签」接口按 url 关联浏览器历史的访问次数。
// 与 history 备份一致：必须为 async 且全程 await，避免 MV3 service worker 提前被杀。
import {API_BASE} from "@/utils/api";

interface BookmarkNode {
    id: string;
    parentId?: string;
    title?: string;
    url?: string;
    dateAdded?: number;
    index?: number;
    children?: BookmarkNode[];
}

// 深度优先展平：每个节点一行（叶子带 url，文件夹 url 为空）。
function flatten(nodes: BookmarkNode[], parentId: string | null, out: any[]): any[] {
    for (const n of nodes || []) {
        out.push({
            id: n.id,
            parentId: parentId ?? "",
            title: n.title || "",
            url: n.url || "",
            isFolder: !n.url,
            sortIndex: n.index ?? 0,
            dateAdded: n.dateAdded ?? 0,
        });
        if (n.children && n.children.length) {
            flatten(n.children, n.id, out);
        }
    }
    return out;
}

async function postJson(path: string, payload: unknown): Promise<Response> {
    const res = await fetch(API_BASE + path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(payload),
    });
    console.log(`[saveBookmarks] POST ${path} -> ${res.status}`);
    return res;
}

const saveBookmarks = async () => {
    try {
        const tree = await browser.bookmarks.getTree();
        const flat: any[] = [];
        flatten(tree, null, flat);

        const batchSize = 256;
        for (let i = 0; i < flat.length; i += batchSize) {
            await postJson("/bookmarks", flat.slice(i, i + batchSize));
        }
    } catch (err) {
        console.error("saveBookmarks failed.", err);
        throw err;
    }
};

export {
    saveBookmarks,
};
