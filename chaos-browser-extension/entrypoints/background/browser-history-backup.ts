import {subDays} from 'date-fns';
import {action} from "@/utils/request";


function chunkArray<T>(arr: T[], size: number): T[][] {
    const result: T[][] = [];
    for (let i = 0; i < arr.length; i += size) {
        result.push(arr.slice(i, i + size));
    }
    return result;
}

// 关键：必须为 async 且全程 await，返回 Promise。
// 否则 MV3 service worker 会在同步代码结束后立刻被杀，上传永远“没生效”。
const saveBrowserHistory = async () => {
    try {
        const startTime = subDays(new Date(), 360).getTime();
        // 1M 条会把内存与 getVisits 调用量放大到不可接受，限制为合理上限。
        const maxResults = 50000;
        const batchSize = 256;

        const histories = await browser.history.search({text: '', maxResults, startTime});

        // 1) 上传历史条目（url / title / lastVisitTime / visitCount 等）
        for (const chunk of chunkArray(histories, batchSize)) {
            // 统一「POST + Action」信封：module=proxy, action=browserHistories/save，负载放 data。
            await action("proxy", "browserHistories/save", chunk);
        }

        // 2) 上传每条 url 的访问明细（visit）
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
