# 应用名称
APP_NAME := GoDownload

# 版本
VERSION := 1.0.0

# 输出目录
BIN_DIR := bin

# Go 源码主文件
MAIN_FILE := main.go

# 目标平台列表
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

# 默认任务：编译全部平台
all: clean build

# 编译全部平台
build:
	@echo "==> Building for all target platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} $(GOFLAGS) go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(BIN_DIR)/$(APP_NAME)-$${platform%/*}-$${platform#*/} $(MAIN_FILE); \
	done

# 清理输出
clean:
	@echo "==> Cleaning..."
	@rm -rf $(BIN_DIR)

# 只编译Mac amd64
mac_amd64:
	@mkdir -p $(BIN_DIR)/darwin_amd64
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC=clang go build -ldflags="-s -w" -o $(BIN_DIR)/darwin_amd64/$(APP_NAME) $(MAIN_FILE)

# 只编译Mac arm64
mac_arm64:
	@mkdir -p $(BIN_DIR)/darwin_arm64
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC=clang go build -ldflags="-s -w" -o $(BIN_DIR)/darwin_arm64/$(APP_NAME) $(MAIN_FILE)

# 只编译Windows amd64
win_amd64:
	@mkdir -p $(BIN_DIR)/windows_amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/windows_amd64/$(APP_NAME).exe $(MAIN_FILE)

.PHONY: all build clean mac_amd64 mac_arm64 win_amd64

pkgbuild:
	pkgbuild --root app/mac/payload --install-location / --scripts app/mac/scripts GoDownload.pkg \
      --identifier org.lxy.godownload.pkg --version $(VERSION)