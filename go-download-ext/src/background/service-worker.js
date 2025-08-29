import {MSG, PROGRESS_CLEANUP_DELAY_MS, SERVER_BASE, STORAGE} from "../types/constants"
import {syncAddHistoryItem} from "../mutex/history"

const sseMap = new Map() // id -> EventSource
const lastForward = new Map() // id -> {time, percent}

// 安全发送消息（统一检查 chrome.runtime.lastError）
function safeSendMessage(message, cb) {
    try {
        chrome.runtime.sendMessage(message, resp => {
            if (chrome.runtime.lastError) {
                // 没有接收端（popup/options 未打开或未注册 listener），静默忽略
                // console.debug('safeSendMessage: no receiver for', message.type, chrome.runtime.lastError.message)
                return
            }
            if (typeof cb === 'function') cb(resp)
        })
    } catch (e) {
        // 防御性捕获（理论上 chrome.runtime.sendMessage 本身不会 throw）
        // console.error('safeSendMessage throw', e)
    }
}

// create context menu on install
chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
        id: 'download-link',
        title: 'Go Download',
        contexts: ['link']
    })
})

chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId !== 'download-link' || !info.linkUrl) return

    chrome.storage.sync.get({downloadPath: '', proxyUrl: ''}, prefs => {
        const {downloadPath, proxyUrl} = prefs
        const body = {url: info.linkUrl}
        if (downloadPath) body.downloadPath = downloadPath.trim()
        if (proxyUrl) body.proxyUrl = proxyUrl

        fetch(`${SERVER_BASE}/download`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        })
            .then(r => r.json())
            .then(res => {
                const {id, size} = res.data || {}
                if (res.code === 0 && id) {
                    // 写入 history（唯一负责写 history 的位置）
                    addHistory({id, url: info.linkUrl, ts: Date.now(), status: 'pending', size: size})

                    openProgressSSE(id)
                } else {
                    console.error('download API error:', res.code, res.message)
                }
            })
            .catch(err => {
                console.error('cannot reach local service:', err)
            })
    })
})

// Start SSE for a task id (idempotent)
function openProgressSSE(id) {
    if (!id) return
    if (sseMap.has(id)) return

    try {
        const url = `${SERVER_BASE}/progress/${id}`
        const es = new EventSource(url)
        sseMap.set(id, es)

        es.onmessage = (e) => {
            const data = JSON.parse(e.data)
            const downloaded = data.downloaded || 0
            const total = data.total || 0
            const speed = data.speed || 0

            // 写当前进度到 storage（覆盖）
            chrome.storage.local.set({[STORAGE.DOWNLOADED_PREFIX + id]: downloaded})
            chrome.storage.local.set({[STORAGE.TOTAL_PREFIX + id]: total})
            chrome.storage.local.set({[STORAGE.SPEED_PREFIX + id]: speed})

            const now = Date.now()
            lastForward.set(id, {time: now, downloaded: downloaded, total: total})

            // 向打开的前端安全转发进度消息
            safeSendMessage({type: MSG.DOWNLOAD_PROGRESS, id, downloaded, total, speed})

            // 当任务完成时：清理 SSE、本地内存，并在延迟后移除持久化进度
            if (downloaded >= total) {
                try {
                    es.close()
                } catch (err) {
                }
                sseMap.delete(id)
                lastForward.delete(id)

                // 延迟移除 gd_progress_<id>，给前端留时间读取最后进度（可改为 0 立即删除）
                const delay = typeof PROGRESS_CLEANUP_DELAY_MS !== 'undefined' ? PROGRESS_CLEANUP_DELAY_MS : 5000
                setTimeout(() => {
                    try {
                        chrome.storage.local.remove(STORAGE.DOWNLOADED_PREFIX + id, () => {
                            // 可选：检查 chrome.runtime.lastError
                            if (chrome.runtime.lastError) {
                            }
                        })
                        chrome.storage.local.remove(STORAGE.SPEED_PREFIX + id, () => {
                            if (chrome.runtime.lastError) {
                            }
                        })
                        chrome.storage.local.remove(STORAGE.TOTAL_PREFIX + id, () => {
                            if (chrome.runtime.lastError) {
                            }
                        })
                    } catch (err) {
                        // 防御性捕获
                    }
                }, delay)
            }
        }

        es.onerror = err => {
            console.warn('SSE error for', id, err)
            try {
                es.close()
            } catch (e) {
            }
            sseMap.delete(id)
            // simple retry after delay
            setTimeout(() => {
                if (!sseMap.has(id)) openProgressSSE(id)
            }, 3000)
        }
    } catch (err) {
        console.error('openProgressSSE failed', err)
    }
}

function addHistory(entry) {
    syncAddHistoryItem(entry).then(() => {
        safeSendMessage({
            type: MSG.ADD_HISTORY,

            id: entry.id,
            url: entry.url,
            ts: entry.ts || Date.now(),
            status: entry.status || 'pending',
            size: entry.size || 0
        })
    })

}

// handle messages from popup/options
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    if (!msg || !msg.type) return
    if (msg.type === MSG.START_PROGRESS_SSE && msg.id) {
        openProgressSSE(msg.id)
        // 立即给 caller 一个确认，避免 "message port closed" 的警告
        sendResponse({ok: true})
        return false
    }
})

