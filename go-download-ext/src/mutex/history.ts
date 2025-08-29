import {historyLock} from "./asyncLock"
import {APP} from "../types/constants"

export interface HistoryItem {
    id: string
    url: string
    status: string
    size?: number
    ts: number
}

export function getHistory(): Promise<HistoryItem[]> {
    return new Promise(resolve =>
        chrome.storage.local.get({history: []}, result => {
            resolve(result.history as HistoryItem[])
        })
    )
}

export function setHistory(hist: HistoryItem[]): Promise<void> {
    return new Promise(resolve =>
        chrome.storage.local.set({history: hist}, () => resolve())
    )
}

/**
 * 标记完成
 */
export async function syncMarkHistoryDone(id: string) {
    const unlock = await historyLock.acquire()
    try {
        const hist = await getHistory()
        let changed = false
        for (let item of hist) {
            if (item.id === id) {
                item.status = "done"
                changed = true
                break
            }
        }
        if (changed) {
            await setHistory(hist)
        }
    } finally {
        unlock()
    }
}

/**
 * 新增任务
 */
export async function syncAddHistoryItem(entry: any): Promise<void> {
    const hist = await getHistory()

    hist.unshift(entry)

    if (hist.length > APP.HISTORY_MAX) {
        hist.length = APP.HISTORY_MAX
    }

    await setHistory(hist)
}
