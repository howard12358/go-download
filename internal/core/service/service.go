package service

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go-download/internal/core/sse"
	"go-download/internal/core/types"
	"go-download/internal/core/util"
	"go-download/internal/core/util/r"
	"go-download/internal/pget"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// DownloadService 把下载相关逻辑封装到结构体
type DownloadService struct {
	hub *sse.Hub
}

func NewDownloadService(hub *sse.Hub) *DownloadService {
	return &DownloadService{hub: hub}
}

func (s *DownloadService) DoDownload(c *gin.Context, req types.Request) {
	id := uuid.New().String()
	s.hub.NewTask(id) // 同步注册任务，避免竞态
	log.Println("start download, id:", id)

	res, err := doHeadRequest(req)
	if err != nil {
		r.Error(c, http.StatusNotAcceptable, err.Error())
		return
	}
	// 2. 异步调用 pget
	go func(url string) {
		cli := pget.New()
		cli.ProgressFn = func(downloaded, total, speed int64) {
			//percent := int(float64(downloaded) / float64(total) * 100)
			s.hub.Publish(id, sse.Progress{
				Downloaded: downloaded,
				Total:      total,
				Speed:      speed,
			})
		}
		ags := util.ToPgetArgs(url, req)
		if err := cli.Run(context.Background(), types.Version, ags); err != nil {
			if cli.Trace {
				fmt.Fprintf(os.Stderr, "Error:\n%+v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Error:\n  %v\n", err)
			}
		}
	}(req.URL)

	// 3. 马上返回成功
	r.Success(c, gin.H{
		"id":   id,
		"size": res.ContentLength,
	})
}

func doHeadRequest(req types.Request) (*http.Response, error) {
	// 查询文件大小
	client := pget.NewClientByProxy(16, req.ProxyUrl)
	r, err := http.NewRequest("HEAD", req.URL, nil)
	if err != nil {
		log.Println("new request failed:", err)
	}
	res, err := client.Do(r)
	if err != nil {
		log.Println("failed to head request:", err)
		return res, err
	}

	if res.Header.Get("Accept-Ranges") != "bytes" {
		return res, errors.New("does not support range request")
	}
	if res.ContentLength <= 0 {
		return res, errors.New("invalid content length")
	}
	return res, nil
}

const throttleInterval = 100 * time.Millisecond

func (s *DownloadService) SSEConnect(c *gin.Context, id string) {
	ch := s.hub.Subscribe(id)
	defer s.hub.Unsubscribe(id, ch)

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// 用 ticker 做节流，间隔 50ms
	throttle := time.NewTicker(throttleInterval)
	defer throttle.Stop()

	// 缓存最近接收到但还未发送的进度
	lastProg := sse.Progress{}
	pending := false

	ctx := c.Request.Context()

	// helper: 立即发送一次数据
	send := func(p sse.Progress) {
		data, err := json.Marshal(p)
		if err != nil {
			log.Println("marshal progress failed:", err)
			return
		}
		_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if err != nil {
			log.Println("send progress failed:", err)
			return
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case prog, ok := <-ch:
			if !ok {
				// channel 关闭：如果有未发送的 pending，先发一次
				if pending {
					send(lastProg)
				}
				log.Println("download finished, id:", id)
				return
			}
			// 收到新的进度，缓存起来（不立即发送，等待 ticker）
			if prog.Speed > 0 {
				lastProg = prog
			} else {
				lastProg.Downloaded = prog.Downloaded
				lastProg.Total = prog.Total
			}
			pending = true
			if lastProg.Downloaded >= lastProg.Total {
				send(lastProg)
				log.Println("download finished, id:", id)
				return
			}
		case <-throttle.C:
			// 周期性发送最新的进度，对高频的进度上报进行节流
			if pending {
				send(lastProg)
				pending = false
			}
		}
	}
}

func (s *DownloadService) OpenInFileManager(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	// 绝对化并清理路径
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	p = filepath.Clean(p)

	// 检查是否存在
	fi, err := os.Stat(p)
	if err != nil {
		return err
	}

	isFile := !fi.IsDir()

	switch runtime.GOOS {
	case "darwin":
		// macOS: open path （文件用 -R reveal）
		if isFile {
			// -R : reveal the file in Finder
			cmd := exec.Command("open", "-R", p)
			return cmd.Start()
		}
		cmd := exec.Command("open", p)
		return cmd.Start()

	case "windows":
		// Windows: explorer.exe path  ; 若为文件，使用 /select,PATH
		// explorer 参数通常以单个字符串传递： "/select,C:\path\to\file"
		if isFile {
			arg := "/select," + p
			cmd := exec.Command("explorer", arg)
			return cmd.Start()
		}
		cmd := exec.Command("explorer", p)
		return cmd.Start()

	default:
		// 大多数 Linux 发行版：xdg-open（GNOME下也可用 nautilus，但 xdg-open 更通用）
		// 如果是文件，打开所在目录（并不一定能选中文件，xdg-open 不支持 select）
		if isFile {
			dir := filepath.Dir(p)
			cmd := exec.Command("xdg-open", dir)
			return cmd.Start()
		}
		cmd := exec.Command("xdg-open", p)
		return cmd.Start()
	}
}
