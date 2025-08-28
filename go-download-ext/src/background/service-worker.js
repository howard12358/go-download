// src/background/service-worker.js
import {APP, MSG, PROGRESS_CLEANUP_DELAY_MS, STORAGE} from "../common/constants";

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
        if (downloadPath) body.downloadPath = downloadPath.trim();
        if (proxyUrl) body.proxyUrl = proxyUrl;

        fetch(`${SERVER_BASE}/download`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        })
            .then(r => r.json())
            .then(data => {
                const {id, status, size} = data || {};
                if (status === 'success' && id) {
                    // 写入 history（唯一负责写 history 的位置）
                    upsertHistoryEntry({id, url: info.linkUrl, ts: Date.now(), status: 'pending', size: size});
                    console.log('openProgressSSE', id)
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

        es.onmessage = (e) => {
            const data = JSON.parse(e.data);
            const downloaded = data.downloaded || 0;
            const total = data.total || 0;
            const speed = data.speed || 0;

            // 写当前进度到 storage（覆盖）
            chrome.storage.local.set({[STORAGE.DOWNLOADED_PREFIX + id]: downloaded});
            chrome.storage.local.set({[STORAGE.TOTAL_PREFIX + id]: total});
            chrome.storage.local.set({[STORAGE.SPEED_PREFIX + id]: speed});

            // 如果达到 100%，标记为完成（更新 history）
            if (downloaded >= total) {
                markHistoryDone(id);
            }

            // 节流 / 合并：最小时间间隔或变化阈值
            const now = Date.now();
            // const prev = lastForward.get(id) || {time: 0, downloaded: 0, total: 0};
            // if (now - prev.time < 10 && Math.abs(downloaded - prev.percent) < 1) {
            //     // 已写 storage，但不再转发消息（节流）
            //     return;
            // }
            lastForward.set(id, {time: now, downloaded: downloaded, total: total});

            // 向打开的前端安全转发进度消息
            safeSendMessage({type: MSG.DOWNLOAD_PROGRESS, id, downloaded, total, speed});

            // 当任务完成时：清理 SSE、本地内存，并在延迟后移除持久化进度
            if (downloaded >= total) {
                try {
                    es.close();
                } catch (err) {
                }
                sseMap.delete(id);
                lastForward.delete(id);

                // 延迟移除 gd_progress_<id>，给前端留时间读取最后进度（可改为 0 立即删除）
                const delay = typeof PROGRESS_CLEANUP_DELAY_MS !== 'undefined' ? PROGRESS_CLEANUP_DELAY_MS : 5000;
                setTimeout(() => {
                    try {
                        chrome.storage.local.remove(STORAGE.DOWNLOADED_PREFIX + id, () => {
                            // 可选：检查 chrome.runtime.lastError
                            if (chrome.runtime.lastError) {
                            }
                        });
                        chrome.storage.local.remove(STORAGE.SPEED_PREFIX + id, () => {
                            if (chrome.runtime.lastError) {
                            }
                        });
                        chrome.storage.local.remove(STORAGE.TOTAL_PREFIX + id, () => {
                            if (chrome.runtime.lastError) {
                            }
                        });
                    } catch (err) {
                        // 防御性捕获
                    }
                }, delay);
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

function upsertHistoryEntry(entry) {
    chrome.storage.local.get({history: []}, res => {
        const hist = res.history || [];
        const idx = hist.findIndex(h => h.id === entry.id);

        if (idx === -1) {
            // not exist -> push to head
            const e = {
                id: entry.id,
                url: entry.url || null,
                ts: entry.ts || Date.now(),
                status: entry.status || 'pending',
                size: entry.size || 0
            };
            hist.unshift(e);
        } else {
            // exist -> merge fields (do NOT clobber good data with null/undefined)
            const cur = hist[idx];
            if (!cur.url && entry.url) cur.url = entry.url;
            // prefer more recent ts if provided
            if (entry.ts && (!cur.ts || entry.ts > cur.ts)) cur.ts = entry.ts;
            if (entry.status) cur.status = entry.status;
            if (entry.size) cur.size = entry.size;
        }

        // cap length
        if (hist.length > APP.HISTORY_MAX) hist.length = APP.HISTORY_MAX;

        chrome.storage.local.set({history: hist}, () => {
            safeSendMessage({
                type: MSG.ADD_HISTORY,

                id: entry.id,
                url: entry.url,
                ts: entry.ts || Date.now(),
                status: entry.status || 'pending',
                size: entry.size || 0
            });
        });
    });
}

// handle messages from popup/options
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (!msg || !msg.type) return;
    if (msg.type === MSG.START_PROGRESS_SSE && msg.id) {
        openProgressSSE(msg.id);
        // 立即给 caller 一个确认，避免 "message port closed" 的警告
        sendResponse({ok: true});
        return false;
    }
});

