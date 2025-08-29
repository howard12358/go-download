<template>
  <div class="options-root">
    <details open>
      <summary>下载历史</summary>
      <div class="panel">
        <ul class="history-list">
          <li v-if="history.length === 0">暂无下载历史</li>

          <li v-for="item in history" :key="item.id">
            <!-- 第一行：URL（左） + 状态（右） -->
            <div class="row row-top">
              <a href="#"
                 :title="item.url || '(no url)'"
                 class="history-link"
                 @click="handleClickUrl"
              >
                {{ ellipsisMiddle(item.url) }}
              </a>

              <div class="status-wrap">
                <span :id="`status-${item.id}`"
                      :class="['status-badge', item.status === 'done' ? 'status-done' : 'status-pending']">
                  {{ item.status === 'done' ? '已完成' : statusText(item.id, item.status) }}
                </span>
              </div>
            </div>

            <!-- 第二行：时间 · 大小 · 速度(下载中) · 已下载(下载中) -->
            <div class="row row-meta">
              <div class="meta-left">
                {{ item.ts ? formatTime(item.ts) : '' }}
                <span class="sep">·</span>
                <span class="meta-item">{{ formatBytes(totalRecord[item.id] || 0) }}</span>
              </div>

              <div class="meta-right">
                <!-- 仅在“下载中”显示速度与已下载百分比 -->
                <template v-if="(item.status !== 'done') && (downloadedRecord[item.id] || 0) > 0">
                  <span class="meta-item">{{ formatSpeed(speedRecord[item.id] || 0) }}</span>

                  <span class="meta-item">
                    {{ formatBytes(downloadedRecord[item.id] || 0) }}
                    ({{ percent(item.id) }}%)
                  </span>
                </template>
              </div>
            </div>

            <!-- 进度条（单独一行） -->
            <div class="row row-progress" v-if="(item.status !== 'done') && (downloadedRecord[item.id] || 0) > 0">
              <div class="progress-bg">
                <div class="progress-fill" :style="{ width: (percent(item.id)) + '%' }"></div>
              </div>
            </div>

          </li>
        </ul>
      </div>
    </details>

    <details>
      <summary>扩展设置</summary>
      <div class="panel">
        <!-- 保留你原来的设置布局 -->
        <div class="field">
          <label for="downloadPath">下载目录</label>
          <div class="input-group">
            <input id="downloadPath" type="text" v-model="downloadPath" placeholder="Your Download Path"/>
            <button @click="chooseDir">浏览…</button>
          </div>
        </div>
        <div class="field">
          <label for="proxyUrl">代理地址</label>
          <div class="input-group">
            <input id="proxyUrl" type="text" v-model="proxyUrl" placeholder="http://127.0.0.1:7897"/>
          </div>
        </div>
        <button class="primary" @click="saveSettings">保存设置</button>
        <div class="status-line">{{ statusTextLine }}</div>
      </div>
    </details>
  </div>
</template>


<script setup lang="ts">
import {onMounted, reactive, ref, toRefs} from 'vue'
import {BASE_URL, MSG} from "../types/constants";

const history = ref<Array<any>>([])
const speedRecord = reactive<Record<string, number>>({})
const downloadedRecord = reactive<Record<string, number>>({})
const totalRecord = reactive<Record<string, number>>({})

const settings = reactive({
  downloadPath: '',
  proxyUrl: '',
  statusTextLine: ''
})
const {downloadPath, proxyUrl, statusTextLine} = toRefs(settings)

function percent(id: string): string {
  const downloaded = downloadedRecord[id] || 0
  const total = totalRecord[id] || 0
  return total === 0 ? '0' : (downloaded / total * 100).toFixed(1)
}

function loadSettings() {
  chrome.storage.sync.get({downloadPath: '', proxyUrl: ''}, prefs => {
    downloadPath.value = prefs.downloadPath || ''
    proxyUrl.value = prefs.proxyUrl || ''
  })
}

async function loadHistory() {
  // 使用 Promise 包装 chrome.storage callback，写法更直观
  const res: any = await new Promise(resolve => {
    chrome.storage.local.get({history: []}, resolve);
  });

  const hist: Array<any> = res.history || [];
  history.value = hist;

  // 构建 id -> size 的临时映射（来自 storage 的历史数据）
  const newTotals: Record<string, number> = {};
  for (const item of hist) {
    if (item && item.id) {
      const size = Number(item.size || 0);
      newTotals[item.id] = Number.isNaN(size) ? 0 : size;
    }
  }

  // 现在把 totalRecord 以最小变动量更新，避免大量响应式触发
  // 1) 删除不再存在的 key
  for (const k of Object.keys(totalRecord)) {
    if (!(k in newTotals)) {
      delete totalRecord[k];
    }
  }
  // 2) 更新或新增，且仅在值变化时写入
  for (const k of Object.keys(newTotals)) {
    const newV = newTotals[k];
    if (totalRecord[k] !== newV) {
      totalRecord[k] = newV;
    }
  }
}

function saveSettings() {
  chrome.storage.sync.set({downloadPath: downloadPath.value, proxyUrl: proxyUrl.value}, () => {
    statusTextLine.value = '设置已保存'
    setTimeout(() => (statusTextLine.value = ''), 1400)
  })
}

function chooseDir() {
  fetch(`${BASE_URL}/gd/choose-dir`)
      .then(r => r.json())
      .then(res => {
        if (res.code === 0) {
          if (res.data.path) downloadPath.value = res.data.path
        } else {
          console.log('choose dir error', res.message)
        }
      })
      .catch(e => alert('无法联系本地服务: ' + e))
}

function updateItemProgress(id: string, downloaded?: number, total?: number, speed?: number) {
  if (typeof total === 'number' && !Number.isNaN(total)) {
    totalRecord[id] = total
  }
  if (typeof downloaded === 'number' && !Number.isNaN(downloaded)) {
    downloadedRecord[id] = Math.max(downloaded, downloadedRecord[id] || 0)
  }
  speedRecord[id] = Math.floor(Number(speed) || 0)
  if (downloaded >= total) {
    // 清理速度显示（下载完成时隐藏）
    speedRecord[id] = 0

    // refresh history status marker
    chrome.storage.local.get({history: []}, res => {
      const hist = res.history || []
      let changed = false
      for (let i = 0; i < hist.length; i++) {
        if (hist[i].id === id) {
          hist[i].status = 'done'
          changed = true
          break
        }
      }
      if (changed) chrome.storage.local.set({history: hist}, loadHistory)
    })
  }
}

// function removeHistoryItem(id: string) {
//   chrome.storage.local.get({history: []}, res => {
//     const hist = (res.history || []).filter((h: any) => h.id !== id)
//     chrome.storage.local.set({history: hist}, () => {
//       // 移除进度和速度的本地存储
//       chrome.storage.local.remove(STORAGE.DOWNLOADED_PREFIX + id)
//       chrome.storage.local.remove(STORAGE.TOTAL_PREFIX + id)
//       chrome.storage.local.remove(STORAGE.SPEED_PREFIX + id)
//       delete downloadedRecord[id]
//       delete totalRecord[id]
//       delete speedRecord[id]
//       loadHistory()
//     })
//   })
// }

function statusText(id: string, s: string) {
  if (s === 'done') return '已完成'
  let downloaded = downloadedRecord[id] || 0
  return downloaded > 0 ? '下载中' : '等待中'
}

function ellipsisMiddle(url?: string) {
  if (!url) return '(no url)';
  if (url.length <= 35) return url;
  return `${url.slice(0, 25)}…${url.slice(url.length - 6)}`;
}

// 格式化 speed（bytes/sec）到可读字符串
function formatSpeed(bytesPerSec?: number) {
  const v = Number(bytesPerSec || 0)
  if (v <= 0) return '0 B/s'
  const KB = 1024
  const MB = KB * 1024
  const GB = MB * 1024

  if (v < KB) return `${v} B/s`
  if (v < MB) return `${(v / KB).toFixed(1).replace(/\.0$/, '')} KB/s`
  if (v < GB) return `${(v / MB).toFixed(1).replace(/\.0$/, '')} MB/s`
  return `${(v / GB).toFixed(2).replace(/\.00$/, '')} GB/s`
}

function formatBytes(bytes?: number) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);
  return value.toFixed(1) + ' ' + sizes[i]; // 保留1位小数
}

function formatTime(ts: number): string {
  const date = new Date(ts);
  const now = new Date();

  // 今天 0 点
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  // 昨天 0 点
  const yesterdayStart = todayStart - 24 * 60 * 60 * 1000;

  if (ts >= todayStart) {
    // 今天
    return `今天 ${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
  } else if (ts >= yesterdayStart) {
    // 昨天
    return `昨天 ${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
  } else {
    // 其它日期
    return `${date.getFullYear()}/${(date.getMonth() + 1).toString().padStart(2, "0")}/${date.getDate().toString().padStart(2, "0")} ` +
        `${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
  }
}

function handleClickUrl() {
  fetch(`${BASE_URL}/gd/open-dir?path=${downloadPath.value}`)
      .then(r => r.json())
      .then(res => {
        if (res.code !== 0) {
          console.error('open dir failed', res.message)
        }
      })
      .catch(e => alert('无法联系本地服务: ' + e))
}

onMounted(() => {
  loadSettings()
  loadHistory()

  chrome.runtime.onMessage.addListener((msg) => {
    if (!msg) return
    if (msg.type === MSG.DOWNLOAD_PROGRESS) {
      const {id, downloaded, total, speed} = msg;
      if (id && typeof downloaded !== 'undefined' && typeof total !== 'undefined') {
        updateItemProgress(id, downloaded, total, speed);
      }
    } else if (msg.type === MSG.ADD_HISTORY) {
      const {id, url, ts, status, size} = msg
      // history.value.push({id, url, ts, status, size})
      history.value = [{id, url, ts, status, size}, ...history.value]
      totalRecord[id] = size
    }
  })
})
</script>
