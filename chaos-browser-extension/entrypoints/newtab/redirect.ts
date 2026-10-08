// 外部脚本避免扩展 CSP 的 script-src 'self' 阻断内联脚本。
// 优先尝试 https，连接失败（拒绝/超时）再回退 http，兼容两种地址。
const HTTPS_URL = 'https://localhost:30030/';
const HTTP_URL = 'http://localhost:30030/';
const PROBE_TIMEOUT = 1500;

async function probe(url: string): Promise<boolean> {
  try {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), PROBE_TIMEOUT);
    await fetch(url, { method: 'HEAD', mode: 'no-cors', signal: ctrl.signal });
    clearTimeout(timer);
    return true;
  } catch {
    return false;
  }
}

(async () => {
  window.location.href = (await probe(HTTPS_URL)) ? HTTPS_URL : HTTP_URL;
})();
