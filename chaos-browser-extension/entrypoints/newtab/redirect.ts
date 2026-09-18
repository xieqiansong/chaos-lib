// 立即跳转到后端 Web 界面（本地服务）。
// 单独抽出为外部脚本，避免内联脚本触发扩展默认 CSP（script-src 'self'）报错。
window.location.href = 'http://localhost:30030/';
