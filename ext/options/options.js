// // DOM 元素
// const downloadInput = document.getElementById('downloadPath');
// const proxyInput = document.getElementById('proxyUrl');
// const saveBtn = document.getElementById('saveBtn');
// const statusDiv = document.getElementById('status');
// const browseBtn = document.getElementById('browseBtn');
//
// // 从 storage 读取已有设置，初始化输入框
// chrome.storage.sync.get({
//     downloadPath: '',
//     proxyUrl: ''
// }, prefs => {
//     downloadInput.value = prefs.downloadPath;
//     proxyInput.value = prefs.proxyUrl;
// });
//
// // 点击“保存”时，把输入框值存到 storage
// saveBtn.addEventListener('click', () => {
//     const downloadPath = downloadInput.value.trim();
//     const proxyUrl = proxyInput.value.trim();
//
//     chrome.storage.sync.set({downloadPath, proxyUrl}, () => {
//         statusDiv.textContent = '保存成功！';
//         setTimeout(() => statusDiv.textContent = '', 2000);
//     });
// });
//
//
// // 新增：点击“浏览”——调用本地服务，打开系统对话框
// browseBtn.addEventListener('click', () => {
//     fetch('http://127.0.0.1:11235/gd/choose-dir')
//         .then(res => res.json())
//         .then(data => {
//             if (data.error) {
//                 alert('选择失败: ' + data.error);
//             } else {
//                 downloadInput.value = data.path;
//             }
//         })
//         .catch(err => {
//             alert('无法联系本地服务，请先启动下载服务:\n' + err);
//         });
// });
//
// chrome.runtime.onMessage.addListener(msg => {
//     if (msg.type === 'DOWNLOAD_PROGRESS') {
//         console.log('DOWNLOAD_PROGRESS', msg);
//         const bar = document.getElementById('progressBar');
//         bar.style.width = msg.percent + '%';
//         bar.textContent = msg.percent + '%';
//     }
// });

// DOM 元素（可能在某些页面/测试时不存在，故做 null-check）
const downloadInput = document.getElementById('downloadPath');
const proxyInput = document.getElementById('proxyUrl');
const saveBtn = document.getElementById('saveBtn');
const statusDiv = document.getElementById('status');
const browseBtn = document.getElementById('browseBtn');
const historyList = document.getElementById('historyList');
const progressBar = document.getElementById('progressBar');

// 初始化表单值（从 sync 读取）
chrome.storage.sync.get({downloadPath: '', proxyUrl: ''}, prefs => {
    if (downloadInput) downloadInput.value = prefs.downloadPath || '';
    if (proxyInput) proxyInput.value = prefs.proxyUrl || '';
});

// 保存设置
if (saveBtn) {
    saveBtn.addEventListener('click', () => {
        const downloadPath = downloadInput ? downloadInput.value.trim() : '';
        const proxyUrl = proxyInput ? proxyInput.value.trim() : '';
        chrome.storage.sync.set({downloadPath, proxyUrl}, () => {
            if (statusDiv) {
                statusDiv.textContent = '保存成功！';
                setTimeout(() => {
                    statusDiv.textContent = '';
                }, 2000);
            }
        });
    });
}

// 浏览按钮：调用本地服务打开系统选择目录对话框
if (browseBtn) {
    browseBtn.addEventListener('click', () => {
        fetch('http://127.0.0.1:11235/gd/choose-dir')
            .then(res => res.json())
            .then(data => {
                if (data.error) {
                    alert('选择失败: ' + data.error);
                } else {
                    if (downloadInput) downloadInput.value = data.path || '';
                }
            })
            .catch(err => {
                alert('无法联系本地服务，请先启动下载服务:\n' + err);
            });
    });
}

// 填充历史列表（从 storage.local.history）
function renderHistory() {
    if (!historyList) return;
    chrome.storage.local.get({history: []}, res => {
        const hist = res.history || [];
        // 清空
        historyList.innerHTML = '';
        if (hist.length === 0) {
            const li = document.createElement('li');
            li.textContent = '暂无下载历史';
            historyList.appendChild(li);
            return;
        }
        hist.forEach(entry => {
            const li = document.createElement('li');
            li.style.padding = '0.25em 0';
            const a = document.createElement('a');
            a.href = '#';
            a.textContent = entry.url;
            a.title = entry.url;
            a.style.textDecoration = 'none';
            a.style.color = '#333';

            // 点击历史项：把该任务的进度加载到主进度条
            a.addEventListener('click', (ev) => {
                ev.preventDefault();
                const id = entry.id;
                // 设置为最近查看的 id
                chrome.storage.local.set({'gd_last_id': id, 'gd_last_url': entry.url});
                // 请求 background 建立 SSE（如果尚未建立）
                chrome.runtime.sendMessage({type: 'START_PROGRESS_SSE', id});
                // 加载已持久化进度（如果有）
                chrome.storage.local.get(['gd_progress_' + id], r => {
                    const pct = r['gd_progress_' + id];
                    if (typeof pct === 'number') {
                        updateProgressBar(pct);
                    }
                });
            });

            li.appendChild(a);
            // small timestamp
            if (entry.ts) {
                const span = document.createElement('div');
                span.style.fontSize = '11px';
                span.style.color = '#666';
                span.textContent = new Date(entry.ts).toLocaleString();
                li.appendChild(span);
            }
            historyList.appendChild(li);
        });
    });
}

// 更新页面上的主进度条（安全检查）
function updateProgressBar(percent) {
    if (!progressBar) return;
    const p = Math.max(0, Math.min(100, Number(percent) || 0));
    progressBar.style.width = p + '%';
    progressBar.textContent = p + '%';
}

// 在页面加载时，恢复最近任务进度（如果存在）
function restoreLastProgress() {
    chrome.storage.local.get(['gd_last_id'], res => {
        const lastId = res['gd_last_id'];
        if (!lastId) return;
        // 请求 background 建立 SSE（如果还没建立）
        chrome.runtime.sendMessage({type: 'START_PROGRESS_SSE', id: lastId});
        // 读取持久化进度并显示
        chrome.storage.local.get(['gd_progress_' + lastId], r => {
            const pct = r['gd_progress_' + lastId];
            if (typeof pct === 'number') updateProgressBar(pct);
        });
    });
}

// 接收 background 转发的进度消息
chrome.runtime.onMessage.addListener((msg, sender) => {
    if (!msg || msg.type !== 'DOWNLOAD_PROGRESS') return;
    // persist latest progress (background 已保存，但这里也保存，保险)
    chrome.storage.local.set({['gd_progress_' + msg.id]: msg.percent});
    // 更新主进度条
    updateProgressBar(msg.percent);
});

// 初始渲染
renderHistory();
restoreLastProgress();