package route

import (
	"github.com/gin-gonic/gin"
	"go-download/internal/core/api"
	"go-download/internal/core/service"
	"go-download/internal/core/sse"
)

// SetupRouter 在这里集中注册所有路由
// 注入 hub 与 service（依赖注入）
func SetupRouter(hub *sse.Hub, svc *service.DownloadService) *gin.Engine {
	r := gin.Default()

	apiHandler := api.NewAPI(svc, hub)

	// router group
	routerGroup := r.Group("/gd")
	{
		routerGroup.GET("/choose-dir", apiHandler.ChooseDirHandler)
		routerGroup.GET("/open-dir", apiHandler.OpenDirHandler)
		routerGroup.POST("/download", apiHandler.DownloadHandler)
		routerGroup.GET("/progress/:id", apiHandler.ProgressSSE)
	}

	return r
}
