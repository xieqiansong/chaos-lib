export interface SearchEngine {
    name: string;
    icon: string;
    label: string;
    url: string;
    aliases?: string[];
}

const ALL_ENGINES: SearchEngine[] = [
    {name: 'google', icon: 'https://www.google.com/favicon.ico', label: 'googleLabel', url: 'https://www.google.com/search?q=', aliases: ['谷歌']},
    {name: 'bing', icon: 'https://www.bing.com/favicon.ico', label: 'bingLabel', url: 'https://www.bing.com/search?q='},
    {name: 'baidu', icon: 'https://www.baidu.com/favicon.ico', label: 'baiduLabel', url: 'https://www.baidu.com/s?wd=', aliases: ['百度']},
    {name: 'kimi', icon: 'https://kimi.moonshot.cn/favicon.ico', label: 'kimiLabel', url: 'https://kimi.moonshot.cn/?q=', aliases: ['Kimi']},
    {name: 'doubao', icon: 'https://www.doubao.com/favicon.ico', label: 'doubaoLabel', url: 'https://www.doubao.com/?q=', aliases: ['豆包']},
    {name: 'chatgpt', icon: 'https://chat.openai.com/favicon.ico', label: 'chatgptLabel', url: 'https://chat.openai.com/?q=', aliases: ['ChatGPT']},
    {name: 'felo', icon: 'https://felo.ai/favicon.ico', label: 'feloLabel', url: 'https://felo.ai/search?q=', aliases: ['Felo']},
    {name: 'metaso', icon: 'https://metaso.cn/favicon.ico', label: 'metasoLabel', url: 'https://metaso.cn/?q=', aliases: ['Metaso']},
    {name: 'perplexity', icon: 'https://www.perplexity.ai/favicon.ico', label: 'perplexityLabel', url: 'https://www.perplexity.ai/?q=', aliases: ['Perplexity']},
    {name: 'semanticscholar', icon: 'https://www.semanticscholar.org/favicon.ico', label: 'semanticscholarLabel', url: 'https://www.semanticscholar.org/search?q=', aliases: ['Semantic Scholar']},
    {name: 'deepseek', icon: 'https://chat.deepseek.com/favicon.ico', label: 'deepseekLabel', url: 'https://chat.deepseek.com/?q=', aliases: ['DeepSeek']},
    {name: 'grok', icon: 'https://grok.com/favicon.ico', label: 'grokLabel', url: 'https://grok.com/?q=', aliases: ['Grok']},
    {name: 'yahoo', icon: 'https://search.yahoo.com/favicon.ico', label: 'yahooLabel', url: 'https://search.yahoo.com/search?p=', aliases: ['雅虎']},
    {name: 'duckduckgo', icon: 'https://duckduckgo.com/favicon.ico', label: 'duckduckgoLabel', url: 'https://duckduckgo.com/?q=', aliases: ['DuckDuckGo']},
    {name: 'yandex', icon: 'https://yandex.com/favicon.ico', label: 'yandexLabel', url: 'https://yandex.com/search/?text=', aliases: ['Yandex']},
    {name: 'xiaohongshu', icon: 'https://www.xiaohongshu.com/favicon.ico', label: 'xiaohongshuLabel', url: 'https://www.xiaohongshu.com/search_result?keyword=', aliases: ['小红书']},
    {name: 'jike', icon: 'https://web.okjike.com/favicon.ico', label: 'jikeLabel', url: 'https://web.okjike.com/search?keyword=', aliases: ['即刻']},
    {name: 'zhihu', icon: 'https://www.zhihu.com/favicon.ico', label: 'zhihuLabel', url: 'https://www.zhihu.com/search?q=', aliases: ['知乎']},
    {name: 'douban', icon: 'https://www.douban.com/favicon.ico', label: 'doubanLabel', url: 'https://www.douban.com/search?q=', aliases: ['豆瓣']},
    {name: 'bilibili', icon: 'https://search.bilibili.com/favicon.ico', label: 'bilibiliLabel', url: 'https://search.bilibili.com/all?keyword=', aliases: ['Bilibili']},
    {name: 'github', icon: 'https://github.com/favicon.ico', label: 'githubLabel', url: 'https://github.com/search?q=', aliases: ['GitHub']}
];

const ENGINE_CATEGORIES = {
    AI: ['kimi', 'doubao', 'chatgpt', 'perplexity', 'claude', 'felo', 'metaso', 'semanticscholar', 'deepseek', 'grok'],
    SEARCH: ['google', 'bing', 'baidu', 'duckduckgo', 'yahoo', 'yandex'],
    SOCIAL: ['xiaohongshu', 'jike', 'zhihu', 'douban', 'bilibili', 'github']
};

export {
    ENGINE_CATEGORIES,
    ALL_ENGINES
};