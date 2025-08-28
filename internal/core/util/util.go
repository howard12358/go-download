package util

import (
	"go-download/internal/core/types"
	"os"
	"path/filepath"
	"runtime"
)

func ToPgetArgs(url string, req types.Request) []string {
	var ags []string
	if req.ProxyUrl != "" {
		ags = append(ags, "-x")
		ags = append(ags, req.ProxyUrl)
	}
	ags = append(ags, "-p")
	ags = append(ags, "4")
	ags = append(ags, "-o")
	if req.DownloadPath != "" {
		ags = append(ags, req.DownloadPath)
	} else {
		ags = append(ags, defaultDownloadsDir())
	}
	ags = append(ags, url)
	return ags
}

func defaultDownloadsDir() string {
	// 简单且通常有效的做法：用 home + "Downloads"
	// 更严格的实现可以在 Linux 读取 ~/.config/user-dirs.dirs 中 XDG_DOWNLOAD_DIR
	home, err := os.UserHomeDir()
	if err != nil {
		return "." // 兜底
	}

	switch runtime.GOOS {
	case "windows":
		// Windows 的 Downloads 通常在 %USERPROFILE%\Downloads
		return filepath.Join(home, "Downloads")
	case "darwin":
		return filepath.Join(home, "Downloads")
	default: // linux / other
		// 尝试读取 XDG 配置（更完善），这里先用 home/Downloads
		return filepath.Join(home, "Downloads")
	}
}
