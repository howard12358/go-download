// src/background/service-worker.js
const SERVER_BASE = 'http://127.0.0.1:11235/gd';
const sseMap = new Map(); // id -> EventSource
const lastForward = new Map(); // id -> {time, percent}

// 安全发送消息（统一检查 chrome.runtime.lastError）
function safeSendMessage(message, cb) {
    try {
        chrome.runtime.sendMessage(message, resp => {
            if (chrome.runtime.lastError) {
                // 没有接收端（popup/options 未打开或未注册 listener），静默忽略
                // console.debug('safeSendMessage: no receiver for', message.type, chrome.runtime.lastError.message);
                return;
            }
            if (typeof cb === 'function') cb(resp);
        });
    } catch (e) {
        // 防御性捕获（理论上 chrome.runtime.sendMessage 本身不会 throw）
        // console.error('safeSendMessage throw', e);
    }
}

// create context menu on install
chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
        id: 'download-link',
        title: 'Go Download',
        contexts: ['link']
    });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId !== 'download-link' || !info.linkUrl) return;

    chrome.storage.sync.get({downloadPath: '', proxyUrl: ''}, prefs => {
        const {downloadPath, proxyUrl} = prefs;
        const body = {url: info.linkUrl};
        if (downloadPath) body.downloadPath = downloadPath;
        if (proxyUrl) body.proxyUrl = proxyUrl;

        fetch(`${SERVER_BASE}/download`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        })
            .then(r => r.json())
            .then(data => {
                const {id, status} = data || {};
                if (status === 'success' && id) {
                    // push history (recent first, cap 100)
                    chrome.storage.local.get({history: []}, res => {
                        const hist = res.history || [];
                        hist.unshift({id, url: info.linkUrl, ts: Date.now(), status: 'pending'});
                        if (hist.length > 100) hist.length = 100;
                        chrome.storage.local.set({history: hist}, () => {
                            // notify options/popup if open (安全发送)
                            safeSendMessage({type: 'ADD_HISTORY', item: {id, url: info.linkUrl}});
                        });
                    });

                    openProgressSSE(id);
                } else {
                    console.error('download API error:', data);
                }
            })
            .catch(err => {
                console.error('cannot reach local service:', err);
            });
    });
});

// Start SSE for a task id (idempotent)
function openProgressSSE(id) {
    if (!id) return;
    if (sseMap.has(id)) return;

    try {
        const url = `${SERVER_BASE}/progress/${id}`;
        const es = new EventSource(url);
        sseMap.set(id, es);

        es.onmessage = e => {
            const percent = Number(e.data) || 0;
            chrome.storage.local.set({['gd_progress_' + id]: percent});

            // if not in history, add it (defensive)
            safePushHistoryIfNotExist({id, url: null, ts: Date.now(), status: percent >= 100 ? 'done' : 'pending'});

            if (percent >= 100) markHistoryDone(id);

            // throttle forward: min interval 10ms OR percent difference >=1
            const now = Date.now();
            const prev = lastForward.get(id) || {time: 0, percent: -1};
            if (now - prev.time < 10 && Math.abs(percent - prev.percent) < 1) {
                return;
            }
            lastForward.set(id, {time: now, percent});

            // use safeSendMessage to avoid "Receiving end does not exist" warnings
            safeSendMessage({type: 'DOWNLOAD_PROGRESS', id, percent});

            if (percent >= 100) {
                try {
                    es.close();
                } catch (e) {
                }
                sseMap.delete(id);
            }
        };

        es.onerror = err => {
            console.warn('SSE error for', id, err);
            try {
                es.close();
            } catch (e) {
            }
            sseMap.delete(id);
            // simple retry after delay
            setTimeout(() => {
                if (!sseMap.has(id)) openProgressSSE(id);
            }, 3000);
        };
    } catch (err) {
        console.error('openProgressSSE failed', err);
    }
}

function safePushHistoryIfNotExist(entry) {
    chrome.storage.local.get({history: []}, res => {
        const hist = res.history || [];
        const exists = hist.find(h => h.id === entry.id);
        if (!exists) {
            hist.unshift(entry);
            if (hist.length > 100) hist.length = 100;
            chrome.storage.local.set({history: hist});
        } else if (entry.status) {
            hist.forEach(h => {
                if (h.id === entry.id) h.status = entry.status;
            });
            chrome.storage.local.set({history: hist});
        }
    });
}

function markHistoryDone(id) {
    chrome.storage.local.get({history: []}, res => {
        const hist = res.history || [];
        let changed = false;
        for (let i = 0; i < hist.length; i++) {
            if (hist[i].id === id) {
                hist[i].status = 'done';
                changed = true;
                break;
            }
        }
        if (changed) chrome.storage.local.set({history: hist});
    });
}

// handle messages from popup/options
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (!msg || !msg.type) return;
    if (msg.type === 'START_PROGRESS_SSE' && msg.id) {
        openProgressSSE(msg.id);
        // 立即给 caller 一个确认，避免 "message port closed" 的警告
        sendResponse({ok: true});
        return false;
    }
});

