package web

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-download/internal/common"
	"go-download/internal/pget"
	"go-download/internal/progress"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DownloadHandler 处理文件下载请求
func DownloadHandler(c *gin.Context, hub *progress.Hub) {
	var req common.Request
	// 1. 绑定 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	id := uuid.New().String()
	// 2. 异步调用 pget
	go func(url string) {
		cli := pget.New()
		hub.NewTask(id)
		cli.ProgressFn = func(downloaded, total int64) {
			percent := int(float64(downloaded) / float64(total) * 100)
			hub.Publish(id, percent)
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
		hub.Publish(id, 100)
	}(req.URL)

	// 3. 马上返回成功
	c.JSON(200, gin.H{
		"status": "success",
		"id":     id,
	})
}

// ProgressSSE 新增一个 /progress/:id SSE endpoint
func ProgressSSE(c *gin.Context, hub *progress.Hub) {
	id := c.Param("id")
	ch := hub.Subscribe(id)
	defer hub.Unsubscribe(id, ch)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	for prog := range ch {
		// 发送一条事件，data 就是 0~100 的数字
		fmt.Fprintf(c.Writer, "data: %d\n\n", prog)
		c.Writer.Flush()
		if prog >= 100 {
			break
		}
	}
}

// ChooseDirHandler 处理选择下载目录请求
func ChooseDirHandler(c *gin.Context) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS 用 AppleScript
		cmd = exec.Command("osascript", "-e",
			`POSIX path of (choose folder with prompt "请选择下载目录")`)
	case "linux":
		// Linux 需安装 zenity
		cmd = exec.Command("zenity", "--file-selection", "--directory", "--title=请选择下载目录")
	case "windows":
		// Windows 用 PowerShell
		// 你可能需要改成一个更完整的 PowerShell 脚本
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`$f = New-Object System.Windows.Forms.FolderBrowserDialog;` +
			`if($f.ShowDialog() -eq "OK"){ Write-Output $f.SelectedPath }`
		cmd = exec.Command("powershell", "-NoProfile", "-Command", script)
	default:
		c.JSON(http.StatusNotImplemented, gin.H{"error": "unsupported platform"})
		return
	}

	out, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	path := strings.TrimSpace(string(out))
	c.JSON(http.StatusOK, gin.H{"path": path})
}
