package route

import (
	"github.com/gin-gonic/gin"
	"go-download/internal/core/http"
	"go-download/internal/core/sse"
)

// SetupRouter 在这里集中注册所有路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	hub := sse.NewHub()

	// 如果你有中间件，也可以在这里统一 apply：

	api := r.Group("/gd")
	{
		api.GET("/choose-dir", http.ChooseDirHandler)
		api.GET("/open-dir", http.OpenDirHandler)
		api.POST("/download", func(c *gin.Context) {
			http.DownloadHandler(c, hub)
		})
		api.GET("/progress/:id", func(c *gin.Context) {
			http.ProgressSSE(c, hub)
		})
	}

	return r
}
