package r

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Resp 通用响应结构：仅包含 code, message, data
type Resp[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// JSON 统一发送 JSON
func JSON[T any](c *gin.Context, httpStatus int, code int, message string, data T) {
	c.AbortWithStatusJSON(httpStatus, Resp[T]{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Success 便捷成功（http 200, code 0）
func Success[T any](c *gin.Context, data T) {
	JSON(c, http.StatusOK, 0, "ok", data)
}

// Error 便捷失败（自定义业务 code & http status）
func Error(c *gin.Context, httpStatus int, message string) {
	JSON(c, httpStatus, -1, message, struct {
	}{})
}
