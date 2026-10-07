import {subDays} from 'date-fns';
import {action} from "@/utils/request";


function chunkArray<T>(arr: T[], size: number): T[][] {
    const result: T[][] = [];
    for (let i = 0; i < arr.length; i += size) {
        result.push(arr.slice(i, i + size));
    }
    return result;
}

// 必须 async + 全程 await，否则 MV3 service worker 会提前被杀。
const saveBrowserHistory = async () => {
    try {
        const startTime = subDays(new Date(), 360).getTime();
        const maxResults = 50000; // 上限避免内存与 getVisits 调用量失控
        const batchSize = 256;

        const histories = await browser.history.search({text: '', maxResults, startTime});

        for (const chunk of chunkArray(histories, batchSize)) {
            await action("proxy", "browserHistories/save", chunk);
        }

        const visitsBuffer: any[] = [];

        const flushVisits = async () => {
            if (visitsBuffer.length === 0) return;
            const batch = visitsBuffer.splice(0, visitsBuffer.length);
            await action("proxy", "browserHistories/saveVisits", batch);
        };

        for (const item of histories) {
            if (!item.url) continue;
            const visits = await browser.history.getVisits({url: item.url});
            for (const visit of visits) {
                visitsBuffer.push({...visit, id: item.url});
                if (visitsBuffer.length >= batchSize) {
                    await flushVisits();
                }
            }
        }
        await flushVisits();
    } catch (err) {
        console.error("saveBrowserHistory failed.", err);
        throw err;
    }
}

export {
    saveBrowserHistory
}
