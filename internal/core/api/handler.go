package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sqweek/dialog"
	"go-download/internal/core/service"
	"go-download/internal/core/sse"
	"go-download/internal/core/types"
	"go-download/internal/core/util/r"
	"net/http"
)

// API 把 handler 封装到结构体里，便于测试/依赖注入
type API struct {
	svc *service.DownloadService
	hub *sse.Hub
}

func NewAPI(svc *service.DownloadService, hub *sse.Hub) *API {
	return &API{
		svc: svc,
		hub: hub,
	}
}

// DownloadHandler 处理文件下载请求
func (a *API) DownloadHandler(c *gin.Context) {
	var req types.Request
	// 1. 绑定 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		r.Error(c, 400, err.Error())
		return
	}
	a.svc.DoDownload(c, req)
}

// ProgressSSE 新增一个 /progress/:id SSE endpoint
func (a *API) ProgressSSE(c *gin.Context) {
	id := c.Param("id")
	// 如果任务不存在，保持原来的行为
	if _, ok := a.hub.Subs[id]; !ok {
		r.Error(c, http.StatusNoContent, "task finished")
		return
	}
	a.svc.SSEConnect(c, id)
}

// ChooseDirHandler 处理选择下载目录请求
func (a *API) ChooseDirHandler(c *gin.Context) {
	path, err := dialog.Directory().Title("请选择下载目录").Browse()
	if err != nil {
		if errors.Is(err, dialog.ErrCancelled) {
			r.Success(c, struct {
			}{})
			return
		}
		r.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	r.Success(c, gin.H{"path": path})
}

func (a *API) OpenDirHandler(c *gin.Context) {
	path := c.Query("path")
	if err := a.svc.OpenInFileManager(path); err != nil {
		r.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	r.Success(c, gin.H{"path": path})
}
