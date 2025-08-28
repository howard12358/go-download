package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sqweek/dialog"
	"go-download/internal/core/sse"
	"go-download/internal/core/types"
	"net/http"
	"time"
)

// DownloadHandler 处理文件下载请求
func DownloadHandler(c *gin.Context, hub *sse.Hub) {
	var req types.Request
	// 1. 绑定 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	doDownload(c, hub, req)
}

const interval = 50 * time.Millisecond

// ProgressSSE 新增一个 /progress/:id SSE endpoint
func ProgressSSE(c *gin.Context, hub *sse.Hub) {
	id := c.Param("id")
	if _, ok := hub.Subs[id]; !ok {
		c.JSON(201, gin.H{"msg": "task finished"})
		return
	}
	sseConnect(c, hub, id)
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

func OpenDirHandler(c *gin.Context) {
	path := c.Query("path")
	err := openInFileManager(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}
