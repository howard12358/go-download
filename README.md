# GoDownload

**GoDownload** — 将 Chrome 右键下载与本地 Go 下载服务（基于 pget 的分段多线程下载）结合的工具集合。  
项目包含：

- 一个用 Go 实现的本地下载服务（server），负责接收来自扩展的下载请求并启动分段并发下载。
- 一个 Chrome 扩展程序，在浏览器中通过右键菜单把下载任务发送到本地服务。:contentReference[oaicite:1]{index=1}

---

## 主要特性

- [x] 分段多线程并发下载（依赖 `github.com/Code-Hex/pget` 的实现）
- [x] 浏览器一键发送下载任务到本地服务（方便把大文件/直链交给本地下载器），并实时查看下载进度
- [x] 简洁的命令行/HTTP 接口，易于集成与扩展
- [ ] 支持断点续传（后端程序已支持，前端未实现）
- [ ] 根据网络状况实现对文件动态分段下载

---

## 快速开始

进入 [releases](https://github.com/howard12358/go-download/releases) 页面：

- chrome 扩展安装：下载 `dist.zip` 文件，解压后将 dist 目录拖拽进 `chrome://extensions/` 页面（前提是打开 “开发者模式”）
- 本地程序安装：
  - Mac 系统：下载 `GoDownload.dmg` 磁盘进行安装，在启动台找到 GoDownload 并运行
  - Windows 系统：下载 `GoDownload.exe` 便携式程序直接运行

## 本地开发

### 前提条件

- Go 1.20+
- Chrome/Chromium 浏览器用于加载扩展
- Nodejs v22+

### 启动服务

```bash
# 使用 go build
go build -o go-download main.go

./go-download

# 或直接使用 go run
go run main.go
```

### 加载 Chrome 扩展

```shell
cd go-download-ext

npm install

npm run watch
```

1. 打开 Chrome -> `chrome://extensions/`
2. 打开右上角 “开发者模式”
3. 点击 “加载已解压的扩展程序（Load unpacked）”，选择仓库中的 `go-download-ext/dist` 目录
4. 点击 Go Download 扩展的 Service Worker 即可对扩展进行调试

## 致谢

- 感谢 `github.com/Code-Hex/pget` 为本项目提供分段并发下载能力
