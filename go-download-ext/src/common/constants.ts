// 后端 base URL（开发/生产可用 import.meta.env 覆盖）
export const BASE_URL = 'http://127.0.0.1:11235';

// 消息类型（统一名称，避免字符串拼写错误）
export const MSG = {
    START_PROGRESS_SSE: 'START_PROGRESS_SSE',
    DOWNLOAD_PROGRESS: 'DOWNLOAD_PROGRESS',
    ADD_HISTORY: 'ADD_HISTORY',
    // 如需新增，统一写在这里
} as const;
export type MsgType = typeof MSG[keyof typeof MSG];

// storage key 命名（建议统一前缀 gd_）
export const STORAGE = {
    HISTORY: 'history',
    PROGRESS_PREFIX: 'gd_progress_', // 用：STORAGE.PROGRESS_PREFIX + id
} as const;

// app level 配置
export const APP = {
    HISTORY_MAX: 100,
    PROGRESS_THROTTLE_MS: 100, // 写 storage 的节流阈值（ms）
} as const;

// 删除 storage 中进度条数据前的延迟（毫秒），调整为你想要的值；0 表示立即删除
export const PROGRESS_CLEANUP_DELAY_MS = 5 * 1000;
