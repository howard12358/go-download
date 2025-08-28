package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqweek/dialog"
	"go-download/internal/common"
	"go-download/internal/pget"
	"log"
	"net/http"
	"os"
	"time"
)

// DownloadHandler 处理文件下载请求
func DownloadHandler(c *gin.Context, hub *Hub) {
	var req common.Request
	// 1. 绑定 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	id := uuid.New().String()
	log.Println("start download, id:", id)
	// 2. 异步调用 pget
	go func(url string) {
		cli := pget.New()
		hub.NewTask(id)
		cli.ProgressFn = func(downloaded, total, speed int64) {
			//percent := int(float64(downloaded) / float64(total) * 100)
			hub.Publish(id, Progress{
				Downloaded: downloaded,
				Total:      total,
				Speed:      speed,
			})
		}
		ags := common.ToPgetArgs(url, req)
		if err := cli.Run(context.Background(), common.Version, ags); err != nil {
			if cli.Trace {
				fmt.Fprintf(os.Stderr, "Error:\n%+v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Error:\n  %v\n", err)
			}
		}

		// 确保结束后推 100%
		//hub.Publish(id, Progress{100, 0})
	}(req.URL)

	// 查询文件大小
	client := pget.NewClientByProxy(16, req.ProxyUrl)
	r, err := http.NewRequest("HEAD", req.URL, nil)
	if err != nil {
		log.Println("new request failed:", err)
	}
	res, err := client.Do(r)
	if err != nil {
		log.Println("failed to head request:", err)
	}
	// 3. 马上返回成功
	c.JSON(200, gin.H{
		"status": "success",
		"id":     id,
		"size":   res.ContentLength,
	})
}

const interval = 50 * time.Millisecond

// ProgressSSE 新增一个 /progress/:id SSE endpoint
func ProgressSSE(c *gin.Context, hub *Hub) {
	id := c.Param("id")
	if _, ok := hub.Subs[id]; !ok {
		c.JSON(201, gin.H{"msg": "task finished"})
		return
	}
	ch := hub.Subscribe(id)
	defer hub.Unsubscribe(id, ch)

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// 用 ticker 做节流，间隔 50ms
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 缓存最近接收到但还未发送的进度
	lastProg := Progress{}
	pending := false

	ctx := c.Request.Context()

	// helper: 立即发送一次数据
	send := func(p Progress) {
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
		case <-ticker.C:
			// 周期性发送最新的进度，对高频的进度上报进行节流
			if pending {
				send(lastProg)
				pending = false
			}
		}
	}
}

// ChooseDirHandler 处理选择下载目录请求
func ChooseDirHandler(c *gin.Context) {
	path, err := dialog.Directory().Title("请选择下载目录").Browse()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}
