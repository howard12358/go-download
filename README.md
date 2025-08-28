# GoDownload

**GoDownload** 是一个 Chrome 浏览器上用于下载文件的插件，下载程序用 go 语言编写，采用分段多连接并发下载的设计，性能高、体积小，尤其适用于大文件下载（特别是文件未做 CDN 加速时，效果比较显著）。

项目包含：

- 一个用 Go 实现的本地下载服务（server），负责处理所有的前端请求。
- 一个 Chrome 扩展程序，支持右键下载链接，并实时查看下载进度。

## 主要特性

- [x] 分段多线程并发下载（依赖 `github.com/Code-Hex/pget` 的实现）
- [x] 浏览器右击一键下载（仅对文件链接生效），并实时查看下载进度
- [x] 性能高、体积小，数据保存在浏览器本地和云端的谷歌账号里
- [ ] 支持断点续传（后端程序已支持，前端未实现）
- [ ] 根据网络状况实现对文件动态分段下载

## 快速开始

进入 [releases](https://github.com/howard12358/go-download/releases) 页面：

- chrome 扩展安装：下载 `dist.zip` 文件，解压后将 dist 目录拖拽进 `chrome://extensions/` 页面（前提是打开 “开发者模式”），本项目前暂未上架 chrome 应用商店
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
$ go build -o go-download main.go

$ ./go-download

# 或直接使用 go run
$ go run main.go
```

### 加载 Chrome 扩展

插件使用 vue、vite 开发，方便扩展，开发步骤：

```shell
$ cd go-download-ext

$ npm install

$ npm run watch
```

1. 打开 Chrome -> `chrome://extensions/`
2. 打开右上角 “开发者模式”
3. 点击 “加载已解压的扩展程序（Load unpacked）”，选择仓库中的 `go-download-ext/dist` 目录
4. 点击 Go Download 扩展的 Service Worker 即可对扩展进行调试

## 打包项目

- 后端程序打包依赖 make 工具，各步骤详见 makefile 文件
- 扩展程序的打包步骤则与上述开发步骤一致

## 致谢

- 感谢 `github.com/Code-Hex/pget` 为本项目提供的分段并发下载能力
