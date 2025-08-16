package route

import (
	"github.com/gin-gonic/gin"
	"go-download/internal/core"
)

// SetupRouter 在这里集中注册所有路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	hub := core.NewHub()

	// 如果你有中间件，也可以在这里统一 apply：

	api := r.Group("/gd")
	{
		api.GET("/choose-dir", core.ChooseDirHandler)
		api.POST("/download", func(c *gin.Context) {
			core.DownloadHandler(c, hub)
		})
		api.GET("/progress/:id", func(c *gin.Context) {
			core.ProgressSSE(c, hub)
		})
	}

	return r
}
