package web

import (
	"github.com/gin-gonic/gin"
	"go-download/internal/progress"
)

// SetupRouter 在这里集中注册所有路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	hub := progress.NewHub()

	// 如果你有中间件，也可以在这里统一 apply：

	api := r.Group("/gd")
	{
		api.GET("/choose-dir", ChooseDirHandler)
		api.POST("/download", func(c *gin.Context) {
			DownloadHandler(c, hub)
		})
		api.GET("/progress/:id", func(c *gin.Context) {
			ProgressSSE(c, hub)
		})
	}

	return r
}
