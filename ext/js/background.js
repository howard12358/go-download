// chrome.runtime.onInstalled.addListener(() => {
//     console.log("Extension installed");
//     // 确保插件安装后创建右键菜单
//     chrome.contextMenus.create({
//         id: "download-link",
//         title: "Go Download",
//         contexts: ["link"]
//     });
// });
//
// chrome.contextMenus.onClicked.addListener((info, tab) => {
//     if (info.menuItemId === "download-link" && info.linkUrl) {
//         // 读取下载路径和代理地址
//         chrome.storage.sync.get({
//             downloadPath: '',
//             proxyUrl: ''
//         }, prefs => {
//             const {downloadPath, proxyUrl} = prefs;
//
//             // 构造请求体，可附加 downloadPath 和 proxyUrl
//             const body = {url: info.linkUrl};
//             if (downloadPath) body.downloadPath = downloadPath;
//             if (proxyUrl) body.proxyUrl = proxyUrl;
//
//             fetch("http://127.0.0.1:11235/gd/download", {
//                 method: "POST",
//                 headers: {"Content-Type": "application/json"},
//                 body: JSON.stringify(body)
//             })
//                 .then(resp => resp.json())
//                 .then(data => {
//                     let {id, status} = data;
//                     if (status === "success") {
//                         console.log("已推送到下载队列:", info.linkUrl);
//                         // 打开 SSE 通道
//                         const evt = new EventSource(`http://127.0.0.1:11235/gd/progress/${id}`);
//                         evt.onmessage = e => {
//                             // const p = Number(e.data); // 0~100
//                             // // 3）更新你的 UI，比如进度条 width = p + '%'
//                             // document.getElementById('progressBar').style.width = p + '%';
//                             // if (p >= 100) evt.close();
//
//                             // 把进度转发给 popup（或其它页面）
//                             chrome.runtime.sendMessage({
//                                 type: 'DOWNLOAD_PROGRESS',
//                                 id,
//                                 percent: Number(e.data)
//                             });
//                             if (Number(e.data) >= 100) {
//                                 evt.close();
//                             }
//                         };
//                     } else {
//                         console.error("服务端错误:", data);
//                     }
//                 })
//                 .catch(err => {
//                     console.error("无法联系本地服务:", err);
//                 });
//         });
//     }
// });
//
// // 监听来自 popup 的“开始下载”消息，payload: { url, downloadPath, id }
// chrome.runtime.onMessage.addListener((msg, sender) => {
//     if (msg.type === 'START_DOWNLOAD') {
//         const {id} = msg;
//         // 打开 SSE
//         const evt = new EventSource(`http://127.0.0.1:11235/gd/progress/${id}`);
//         evt.onmessage = e => {
//             // 把进度转发给 popup（或其它页面）
//             chrome.runtime.sendMessage({
//                 type: 'DOWNLOAD_PROGRESS',
//                 id,
//                 percent: Number(e.data)
//             });
//             if (Number(e.data) >= 100) {
//                 evt.close();
//             }
//         };
//     }
// });
//
// function openProgressSSE(id) {
//     // if already opened for this id, skip (可选：用 map 存 evt)
//     const evt = new EventSource(`http://127.0.0.1:11235/gd/progress/${id}`);
//     evt.onmessage = e => {
//         const percent = Number(e.data);
//         // 1) persist latest progress so pages opened later可以读取
//         chrome.storage.local.set({['gd_progress_' + id]: percent});
//
//         // 2) forward to any open extension pages (popup/options)
//         chrome.runtime.sendMessage({
//             type: 'DOWNLOAD_PROGRESS',
//             id,
//             percent
//         });
//
//         if (percent >= 100) {
//             evt.close();
//         }
//     };
//     evt.onerror = err => {
//         console.warn('SSE error for', id, err);
//         // 可实现自动重连逻辑
//     };
//     return evt;
// }

const SERVER_BASE = 'http://127.0.0.1:11235/gd';
const sseMap = new Map(); // id -> EventSource

// 打开 SSE 并处理消息（幂等：如果已打开则跳过）
function openProgressSSE(id) {
    if (!id) return;
    if (sseMap.has(id)) return;

    const url = `${SERVER_BASE}/progress/${id}`;
    try {
        const evt = new EventSource(url);
        sseMap.set(id, evt);

        evt.onmessage = e => {
            const percent = Number(e.data);
            // 1) 持久化最新进度，页面打开时可恢复
            chrome.storage.local.set({ ['gd_progress_' + id]: percent });

            // 2) 记录为最近任务 id（方便 options/popup 恢复）
            chrome.storage.local.set({ 'gd_last_id': id });

            // 3) 转发给任何已打开的扩展页面（popup/options）
            chrome.runtime.sendMessage({
                type: 'DOWNLOAD_PROGRESS',
                id,
                percent
            });

            // 4) 如果完成，关闭并清理
            if (percent >= 100) {
                try {
                    evt.close();
                } catch (err) { /* ignore */ }
                sseMap.delete(id);
            }
        };

        evt.onerror = err => {
            console.warn('SSE error for', id, err);
            // 失败时关闭并尝试短延迟后重连（简单策略）
            try {
                evt.close();
            } catch (e) {}
            sseMap.delete(id);
            // 可选：短延迟后重连
            setTimeout(() => {
                if (!sseMap.has(id)) openProgressSSE(id);
            }, 3000);
        };
    } catch (err) {
        console.error('openProgressSSE failed', err);
    }
}

// 安全地把历史插入 storage.local.history（最新在前，最多保存 50 条）
function pushHistoryEntry(entry) {
    chrome.storage.local.get({ history: [] }, res => {
        const hist = res.history || [];
        hist.unshift(entry);
        if (hist.length > 50) hist.length = 50;
        chrome.storage.local.set({ history: hist });
    });
}

// 安装时创建右键菜单
chrome.runtime.onInstalled.addListener(() => {
    console.log('Extension installed');
    chrome.contextMenus.create({
        id: 'download-link',
        title: 'Go Download',
        contexts: ['link']
    });
});

// 右键菜单点击处理
chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId !== 'download-link' || !info.linkUrl) return;

    // 读取设置（下载目录 / 代理地址）
    chrome.storage.sync.get({ downloadPath: '', proxyUrl: '' }, prefs => {
        const { downloadPath, proxyUrl } = prefs;
        const body = { url: info.linkUrl };
        if (downloadPath) body.downloadPath = downloadPath;
        if (proxyUrl) body.proxyUrl = proxyUrl;

        fetch(`${SERVER_BASE}/download`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        })
            .then(r => r.json())
            .then(data => {
                const { id, status } = data || {};
                if (status === 'success' && id) {
                    console.log('已推送到下载队列:', info.linkUrl, id);
                    // 记录历史（包含 id 和 url）
                    pushHistoryEntry({ id, url: info.linkUrl, ts: Date.now() });

                    // 记录为最近任务 id/url 便于 options 页面恢复
                    chrome.storage.local.set({ 'gd_last_id': id, 'gd_last_url': info.linkUrl });

                    // 打开 SSE（并转发进度）
                    openProgressSSE(id);
                } else {
                    console.error('服务端返回错误：', data);
                }
            })
            .catch(err => {
                console.error('无法联系本地服务:', err);
            });
    });
});

// 监听来自 popup/options 的启动 SSE 消息（如果页面主动请求）
chrome.runtime.onMessage.addListener((msg, sender) => {
    if (!msg || !msg.type) return;
    if (msg.type === 'START_PROGRESS_SSE' && msg.id) {
        openProgressSSE(msg.id);
    }
});