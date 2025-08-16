<template>
  <div class="options-root">
    <details open>
      <summary>下载历史</summary>
      <div class="panel">
        <ul class="history-list">
          <li v-if="history.length === 0">暂无下载历史</li>
          <li v-for="item in history" :key="item.id">
            <!-- 第一行：仅 URL -->
            <div class="history-link-row">
              <a href="#"
                 :title="item.url || '(no url)'"
                 class="history-link">
                {{ ellipsisMiddle(item.url) }}
              </a>
            </div>

            <!-- 第二行：时间（左）  状态+移除（右） -->
            <div class="history-actions-row">
              <div class="time-left">
                <div class="history-meta">
                  {{ item.ts ? new Date(item.ts).toLocaleString() : '' }}
                  <span class="speed-text"
                        v-if="item.status !== 'done' && (speeds[item.id] || 0) > 0">
                    · {{ formatSpeed(speeds[item.id] || 0) }}
                  </span>
                </div>
              </div>

              <div class="actions-right">
                <span :id="`status-${item.id}`"
                      :class="['status-badge', item.status === 'done' ? 'status-done' : 'status-pending']">
                  {{ item.status === 'done' ? '已完成' : statusText(item.id, item.status) }}
                </span>
                <button class="btn btn-remove" @click="removeHistoryItem(item.id)">移除</button>
              </div>
            </div>

            <!-- 进度条 -->
            <div class="item-progress" v-if="item.status !== 'done'">
              <div class="progress-row">
                <div class="progress-bg">
                  <div class="progress-fill" :style="{ width: (progress[item.id] || 0) + '%' }"></div>
                </div>
                <div class="progress-percent">{{ progress[item.id] || 0 }}%</div>
              </div>
            </div>
          </li>
        </ul>
      </div>
    </details>

    <details>
      <summary>扩展设置</summary>
      <div class="panel">
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
import {BASE_URL, MSG, STORAGE} from "../common/constants";

const history = ref<Array<any>>([])
const progress = reactive<Record<string, number>>({})
const speeds = reactive<Record<string, number>>({})

const settings = reactive({
  downloadPath: '',
  proxyUrl: '',
  statusTextLine: ''
})
const {downloadPath, proxyUrl, statusTextLine} = toRefs(settings)

function loadSettings() {
  chrome.storage.sync.get({downloadPath: '', proxyUrl: ''}, prefs => {
    downloadPath.value = prefs.downloadPath || ''
    proxyUrl.value = prefs.proxyUrl || ''
  })
}

function loadHistory() {
  chrome.storage.local.get({history: []}, res => {
    history.value = res.history || []
  })
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
      .then(d => {
        if (d.path) downloadPath.value = d.path
      })
      .catch(e => alert('无法联系本地服务: ' + e))
}

function openSSE(item: any) {
  if (!item || !item.id) return;

  // ask background to open SSE (background will ensure only one per id)
  chrome.runtime.sendMessage({type: MSG.START_PROGRESS_SSE, id: item.id}, resp => {
    if (chrome.runtime.lastError) {
      // 后台可能无响应（被终止），静默忽略或记录
      console.debug('no bg response (ignored):', chrome.runtime.lastError.message);
      return;
    }
  });

  // 读已持久化的进度并显示
  chrome.storage.local.get([STORAGE.PERCENT_PREFIX + item.id], r => {
    const pct = r[STORAGE.PERCENT_PREFIX + item.id];
    if (typeof pct === 'number') progress[item.id] = pct;
  });
}

function updateItemProgress(id: string, percent?: number, speed?: number) {
  progress[id] = Math.max(0, Math.min(100, Math.round(percent)))
  speeds[id] = Math.max(0, Math.floor(Number(speed) || 0))
  if (percent >= 100) {
    // 清理速度显示（下载完成时隐藏）
    speeds[id] = 0

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

function removeHistoryItem(id: string) {
  chrome.storage.local.get({history: []}, res => {
    const hist = (res.history || []).filter((h: any) => h.id !== id)
    chrome.storage.local.set({history: hist}, () => {
      // 移除进度和速度的本地存储
      chrome.storage.local.remove(STORAGE.PERCENT_PREFIX + id)
      chrome.storage.local.remove(STORAGE.SPEED_PREFIX + id)
      delete progress[id]
      delete speeds[id]
      loadHistory()
    })
  })
}

function statusText(id: string, s: string) {
  if (s === 'done') return '已完成'
  const p = progress[id] || 0
  return p > 0 ? '下载中' : '等待中'
}

function ellipsisMiddle(url?: string) {
  if (!url) return '(no url)';
  if (url.length <= 35) return url;
  return `${url.slice(0, 25)}…${url.slice(url.length - 10)}`;
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

onMounted(() => {
  loadSettings()
  loadHistory()

  chrome.runtime.onMessage.addListener((msg) => {
    if (!msg) return
    if (msg.type === MSG.DOWNLOAD_PROGRESS) {
      // msg may carry { id, percent, speed }
      if (typeof msg.percent !== 'undefined') {
        updateItemProgress(msg.id, msg.percent, msg.speed)
      }
      // if (typeof msg.speed !== 'undefined') {
      //   updateItemSpeed(msg.id, msg.speed)
      // }
    } else if (msg.type === MSG.ADD_HISTORY) {
      // background might push this when starting a new download
      loadHistory()
    }
  })
})
</script>
