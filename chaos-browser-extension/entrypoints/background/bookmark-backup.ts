// 浏览器书签扁平化备份；async + await 避免 MV3 service worker 提前被杀。
import {action} from "@/utils/request";

interface BookmarkNode {
    id: string;
    parentId?: string;
    title?: string;
    url?: string;
    dateAdded?: number;
    index?: number;
    children?: BookmarkNode[];
}

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

const saveBookmarks = async () => {
    try {
        const tree = await browser.bookmarks.getTree();
        const flat: any[] = [];
        flatten(tree, null, flat);

        const batchSize = 256;
        for (let i = 0; i < flat.length; i += batchSize) {
            await action("proxy", "bookmarks/save", flat.slice(i, i + batchSize));
        }
    } catch (err) {
        console.error("saveBookmarks failed.", err);
        throw err;
    }
};

export {
    saveBookmarks,
};
